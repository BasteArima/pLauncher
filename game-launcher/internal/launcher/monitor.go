package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"game-launcher/internal/db"
	"game-launcher/internal/scanner"
	"game-launcher/internal/sessionlock"
)

const MonitorFlag = "--play-session-monitor"

type startReply struct {
	Error string `json:"error"`
}

// StartMonitored hands the library lifecycle gate to the child. The monitor holds its
// own lifetime lock before acknowledging start, so closing the UI loses no ownership.
func StartMonitored(ctx context.Context, repo *db.SQLiteRepo, dataDir, gameID, exe, folder string) error {
	gate, err := sessionlock.Try(dataDir, "gate")
	if err != nil {
		return err
	}
	defer gate.Close()
	id := scanner.NewID()
	p := db.PendingSession{ID: id, GameID: gameID, Executable: exe, Folder: folder}
	if err := repo.BeginSession(ctx, p); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		repo.CancelSession(ctx, id)
		return err
	}
	cmd := exec.Command(self, MonitorFlag, dataDir, id)
	detach(cmd)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		repo.CancelSession(ctx, id)
		return err
	}
	if err := cmd.Start(); err != nil {
		pipe.Close()
		repo.CancelSession(ctx, id)
		return err
	}
	// The child must take this same gate before opening the DB. If this parent
	// dies here, import/move may win, but then the child cannot start a stale intent.
	gate.Close()
	// Reap the monitor when the UI stays alive; this goroutine isn't needed for tracking.
	go cmd.Wait()
	var reply startReply
	err = json.NewDecoder(pipe).Decode(&reply)
	pipe.Close()
	if err != nil {
		return fmt.Errorf("session monitor did not acknowledge launch: %w", err)
	}
	if reply.Error != "" {
		return errors.New(reply.Error)
	}
	return nil
}

// RunMonitor is a windowless entry point, before Wails and updater initialization.
func RunMonitor(dataDir, id string, reply io.Writer) error {
	return runMonitor(dataDir, id, reply, time.Since)
}

func runMonitor(dataDir, id string, reply io.Writer, elapsed func(time.Time) time.Duration) error {
	ack := func(err error) {
		msg := startReply{}
		if err != nil {
			msg.Error = err.Error()
		}
		_ = json.NewEncoder(reply).Encode(msg)
	}
	var gate *sessionlock.Lock
	var err error
	deadline := time.Now().Add(15 * time.Second)
	for {
		gate, err = sessionlock.Try(dataDir, "gate")
		if !errors.Is(err, sessionlock.ErrBusy) || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		ack(err)
		return err
	}
	defer gate.Close()
	lock, err := sessionlock.Try(dataDir, "run-"+id)
	if err != nil {
		ack(err)
		return err
	}
	defer lock.Close()
	if _, err := os.Stat(filepath.Join(dataDir, "games.db")); err != nil {
		ack(err)
		return err
	}
	repo, err := db.NewSQLiteRepo(filepath.Join(dataDir, "games.db"))
	if err != nil {
		ack(err)
		return err
	}
	defer repo.Close()
	ctx := context.Background()
	ps, err := repo.PendingSessions(ctx)
	if err != nil {
		ack(err)
		return err
	}
	var pending *db.PendingSession
	for i := range ps {
		if ps[i].ID == id {
			pending = &ps[i]
			break
		}
	}
	if pending == nil || pending.StartedAt != 0 {
		err = fmt.Errorf("session launch intent is missing or already started")
		ack(err)
		return err
	}
	cmd := exec.Command(pending.Executable)
	cmd.Dir = pending.Folder
	// Null stdio: game output must never be mistaken for the startup handshake.
	if err = cmd.Start(); err != nil {
		_ = repo.CancelSession(ctx, id)
		ack(err)
		return err
	}
	start := time.Now()
	done := make(chan time.Duration, 1)
	go func() { _ = cmd.Wait(); done <- elapsed(start) }()
	ok, err := repo.StartSession(ctx, id, start.Unix())
	if err != nil {
		// We cannot promise tracked play if the durable start wasn't accepted.
		_ = cmd.Process.Kill()
		<-done
		_ = repo.CancelSession(ctx, id)
		ack(err)
		return err
	}
	ack(nil)
	gate.Close()
	if !ok {
		// Deleting a game during startup cancels recording, never the game itself.
		<-done
		return nil
	}
	return trackSession(ctx, repo, id, start, done, 30*time.Second, time.Second)
}

type sessionWriter interface {
	CheckpointSession(context.Context, string, int64, bool) error
	FinishSession(context.Context, string, bool) error
	SessionError(context.Context, string, string) error
}

// The completion duration is captured by Wait, not after a potentially slow DB write.
// Retry keeps the monitor alive and the exact final duration in memory until committed.
func trackSession(ctx context.Context, repo sessionWriter, id string, start time.Time, done <-chan time.Duration, checkpoint, retry time.Duration) error {
	ticker := time.NewTicker(checkpoint)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := repo.CheckpointSession(ctx, id, int64(time.Since(start)/time.Second), false); err != nil {
				_ = repo.SessionError(ctx, id, err.Error())
			}
		case elapsed := <-done:
			seconds := max(0, int64(elapsed/time.Second))
			for {
				err := repo.CheckpointSession(ctx, id, seconds, true)
				if err == nil {
					err = repo.FinishSession(ctx, id, false)
				}
				if err == nil {
					return nil
				}
				_ = repo.SessionError(ctx, id, err.Error())
				t := time.NewTimer(retry)
				select {
				case <-ctx.Done():
					t.Stop()
					return ctx.Err()
				case <-t.C:
				}
			}
		}
	}
}

// Recover must run under the lifecycle gate. Try the OS lock, never infer death
// from a PID or heartbeat. Imported snapshots use RecoverImportedSessions instead.
func Recover(ctx context.Context, repo *db.SQLiteRepo, dataDir string) error {
	ps, err := repo.PendingSessions(ctx)
	if err != nil {
		return err
	}
	for _, p := range ps {
		l, err := sessionlock.Try(dataDir, "run-"+p.ID)
		if errors.Is(err, sessionlock.ErrBusy) {
			continue
		}
		if err != nil {
			return err
		}
		err = repo.FinishSession(ctx, p.ID, true)
		l.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func CanTrack(exe string) bool { return isDirectExecutable(exe, strings.ToLower(filepath.Ext(exe))) }
