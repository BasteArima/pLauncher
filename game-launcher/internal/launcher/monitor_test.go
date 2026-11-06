package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"game-launcher/internal/db"
	"game-launcher/internal/models"
	"game-launcher/internal/sessionlock"
)

// Re-executed test binaries use the production monitor entry, with only the clock
// accelerated. The game and the launch-parent really are separate OS processes.
func TestMain(m *testing.M) {
	if len(os.Args) == 4 && os.Args[1] == MonitorFlag {
		elapsed := time.Since
		if os.Getenv("PLAUNCHER_TEST_GAME") != "" {
			elapsed = func(time.Time) time.Duration { return 90 * time.Second }
		}
		if err := runMonitor(os.Args[2], os.Args[3], os.Stdout, elapsed); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	if release := os.Getenv("PLAUNCHER_TEST_GAME"); release != "" {
		_ = os.WriteFile(release+".started", []byte(fmt.Sprint(os.Getpid())), 0600)
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(release); err == nil {
				os.Exit(7)
			}
			time.Sleep(10 * time.Millisecond)
		}
		os.Exit(8)
	}
	os.Exit(m.Run())
}

func monitorRepo(t *testing.T) (*db.SQLiteRepo, string) {
	t.Helper()
	dir := t.TempDir()
	r, err := db.NewSQLiteRepo(filepath.Join(dir, "games.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if err := r.SaveGame(context.Background(), &models.Game{ID: "g", TimePlayed: 5}); err != nil {
		t.Fatal(err)
	}
	return r, dir
}

func TestMonitorLaunchParent(t *testing.T) {
	dir := os.Getenv("PLAUNCHER_TEST_PARENT")
	if dir == "" {
		return
	}
	r, err := db.NewSQLiteRepo(filepath.Join(dir, "games.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLAUNCHER_TEST_GAME", filepath.Join(dir, "release"))
	self, _ := os.Executable()
	if err := StartMonitored(context.Background(), r, dir, "g", self, dir); err != nil {
		t.Fatal(err)
	}
	r.Close() // Parent process exits immediately after this test; monitor remains.
}

func eventually(t *testing.T, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out")
}

func TestMonitorSurvivesParentAndReopen(t *testing.T) {
	r, dir := monitorRepo(t)
	self, _ := os.Executable()
	parent := exec.Command(self, "-test.run=^TestMonitorLaunchParent$")
	parent.Env = append(os.Environ(), "PLAUNCHER_TEST_PARENT="+dir)
	if out, err := parent.CombinedOutput(); err != nil {
		t.Fatalf("parent: %v %s", err, out)
	}
	release := filepath.Join(dir, "release")
	defer os.WriteFile(release, []byte("exit"), 0600)
	eventually(t, func() bool { _, err := os.Stat(release + ".started"); return err == nil })
	active, err := sessionlock.Active(dir)
	if err != nil || !active {
		t.Fatalf("monitor didn't survive parent: %v %v", active, err)
	}
	r.Close()
	r, err = db.NewSQLiteRepo(filepath.Join(dir, "games.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	gate, err := sessionlock.Try(dir, "gate")
	if err != nil {
		t.Fatal(err)
	}
	if err := Recover(context.Background(), r, dir); err != nil {
		t.Fatal(err)
	}
	gate.Close()
	ps, err := r.PendingSessions(context.Background())
	if err != nil || len(ps) != 1 {
		t.Fatalf("live session was recovered: %v %v", ps, err)
	}
	if err := os.WriteFile(release, []byte("exit"), 0600); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { active, err := sessionlock.Active(dir); return err == nil && !active })
	s, err := r.ReadSessionSnapshot(context.Background(), []string{"g"})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Sessions) != 1 || s.Sessions[0].DurationSeconds != 90 || s.Sessions[0].Interrupted || s.Games[0].TimePlayed != 6 {
		t.Fatalf("lost or doubled session: %+v", s)
	}
	if err := Recover(context.Background(), r, dir); err != nil {
		t.Fatal(err)
	}
	s, _ = r.ReadSessionSnapshot(context.Background(), []string{"g"})
	if len(s.Sessions) != 1 || s.Games[0].TimePlayed != 6 {
		t.Fatal("duplicate recovery")
	}
}

func TestFailedStartAndRecovery(t *testing.T) {
	r, dir := monitorRepo(t)
	ctx := context.Background()
	if err := r.BeginSession(ctx, db.PendingSession{ID: "failed", GameID: "g", Executable: filepath.Join(dir, "missing.exe"), Folder: dir}); err != nil {
		t.Fatal(err)
	}
	var reply bytes.Buffer
	if err := RunMonitor(dir, "failed", &reply); err == nil {
		t.Fatal("expected start failure")
	}
	ps, _ := r.PendingSessions(ctx)
	if len(ps) != 0 {
		t.Fatal(ps)
	}
	for _, id := range []string{"dead", "alive", "short"} {
		if err := r.BeginSession(ctx, db.PendingSession{ID: id, GameID: "g"}); err != nil {
			t.Fatal(err)
		}
		r.StartSession(ctx, id, 1700000000)
		r.CheckpointSession(ctx, id, 90, false)
	}
	r.CheckpointSession(ctx, "short", 0, false) // monotonic checkpoints cannot move backwards
	l, err := sessionlock.Try(dir, "run-alive")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	gate, err := sessionlock.Try(dir, "gate")
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Close()
	if err := Recover(ctx, r, dir); err != nil {
		t.Fatal(err)
	}
	s, _ := r.ReadSessionSnapshot(ctx, []string{"g"})
	ps, _ = r.PendingSessions(ctx)
	if len(s.Sessions) != 2 || !s.Sessions[0].Interrupted || len(ps) != 1 || ps[0].ID != "alive" {
		t.Fatal(s, ps)
	}
	// Stale lock files are harmless after handles close, irrespective of PID/heartbeat.
	l.Close()
	if err := Recover(ctx, r, dir); err != nil {
		t.Fatal(err)
	}
	active, err := sessionlock.Active(dir)
	if err != nil || active {
		t.Fatal(active, err)
	}
}

type retryWriter struct {
	mu                            sync.Mutex
	checkpoints, finishes, errors int
	seconds                       int64
	complete                      bool
}

func (w *retryWriter) CheckpointSession(_ context.Context, _ string, s int64, c bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.checkpoints++
	w.seconds = s
	w.complete = c
	if w.checkpoints == 1 {
		return errors.New("busy")
	}
	return nil
}
func (w *retryWriter) FinishSession(context.Context, string, bool) error {
	w.finishes++
	if w.finishes == 1 {
		return errors.New("busy")
	}
	return nil
}
func (w *retryWriter) SessionError(context.Context, string, string) error { w.errors++; return nil }

func TestMonitorRetryKeepsExactDuration(t *testing.T) {
	w := &retryWriter{}
	done := make(chan time.Duration, 1)
	done <- 90*time.Second + 999*time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := trackSession(ctx, w, "s", time.Now(), done, time.Hour, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if w.checkpoints != 3 || w.finishes != 2 || w.errors != 2 || w.seconds != 90 || !w.complete {
		t.Fatal(w)
	}
}

func TestMissingIntentDoesNotLaunch(t *testing.T) {
	_, dir := monitorRepo(t)
	if err := RunMonitor(dir, "absent", io.Discard); err == nil {
		t.Fatal("expected missing intent")
	}
	missing := filepath.Join(t.TempDir(), "moved")
	if err := RunMonitor(missing, "absent", io.Discard); err == nil {
		t.Fatal("expected missing database")
	}
	if _, err := os.Stat(filepath.Join(missing, "games.db")); !os.IsNotExist(err) {
		t.Fatal("created obsolete database")
	}
}

func TestKilledMonitorRecoversCheckpoint(t *testing.T) {
	r, dir := monitorRepo(t)
	ctx := context.Background()
	self, _ := os.Executable()
	release := filepath.Join(dir, "release")
	defer func() {
		_ = os.WriteFile(release, []byte("exit"), 0600)
		// A killed monitor cannot reap its child. On Windows retain/wait a process
		// handle before TempDir and Go remove the still-loaded test executable.
		if raw, err := os.ReadFile(release + ".started"); err == nil {
			if pid, err := strconv.Atoi(string(raw)); err == nil {
				if p, err := os.FindProcess(pid); err == nil {
					_, _ = p.Wait()
				}
			}
		}
	}()
	if err := r.BeginSession(ctx, db.PendingSession{ID: "killed", GameID: "g", Executable: self, Folder: dir}); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, MonitorFlag, dir, "killed")
	cmd.Env = append(os.Environ(), "PLAUNCHER_TEST_GAME="+release)
	detach(cmd)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	var reply startReply
	if err := json.NewDecoder(out).Decode(&reply); err != nil || reply.Error != "" {
		t.Fatal(reply, err)
	}
	out.Close()
	eventually(t, func() bool { _, err := os.Stat(release + ".started"); return err == nil })
	if err := r.CheckpointSession(ctx, "killed", 90, false); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	gate, err := sessionlock.Try(dir, "gate")
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Close()
	if err := Recover(ctx, r, dir); err != nil {
		t.Fatal(err)
	}
	s, err := r.ReadSessionSnapshot(ctx, []string{"g"})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Sessions) != 1 || s.Sessions[0].DurationSeconds != 90 || !s.Sessions[0].Interrupted || s.Games[0].TimePlayed != 6 {
		t.Fatal(s)
	}
}

func TestCheckpointTimerAndFinalTime(t *testing.T) {
	r, _ := monitorRepo(t)
	ctx := context.Background()
	if err := r.BeginSession(ctx, db.PendingSession{ID: "timer", GameID: "g"}); err != nil {
		t.Fatal(err)
	}
	r.StartSession(ctx, "timer", 1700000000)
	done := make(chan time.Duration, 1)
	finished := make(chan error, 1)
	go func() {
		finished <- trackSession(ctx, r, "timer", time.Now().Add(-2*time.Minute), done, 5*time.Millisecond, time.Millisecond)
	}()
	eventually(t, func() bool {
		ps, err := r.PendingSessions(ctx)
		return err == nil && len(ps) == 1 && ps[0].Seconds >= 120
	})
	done <- 90 * time.Second
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	s, err := r.ReadSessionSnapshot(ctx, []string{"g"})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Sessions) != 1 || s.Sessions[0].DurationSeconds != 90 {
		t.Fatal("checkpoint overrode exact Wait duration", s)
	}
}

// Optional smoke check for the packaged Wails executable's early, windowless entry.
// The helper exits quickly; the real clock must discard this sub-minute launch.
func TestBuiltMonitorEntry(t *testing.T) {
	binary := os.Getenv("PLAUNCHER_BUILT_EXE")
	if binary == "" {
		t.Skip("set PLAUNCHER_BUILT_EXE after build.ps1")
	}
	r, dir := monitorRepo(t)
	ctx := context.Background()
	self, _ := os.Executable()
	if err := r.BeginSession(ctx, db.PendingSession{ID: "built", GameID: "g", Executable: self, Folder: dir}); err != nil {
		t.Fatal(err)
	}
	release := filepath.Join(dir, "release")
	defer os.WriteFile(release, []byte("exit"), 0600)
	cmd := exec.Command(binary, MonitorFlag, dir, "built")
	detach(cmd)
	cmd.Env = append(os.Environ(), "PLAUNCHER_TEST_GAME="+release)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	var reply startReply
	if err := json.NewDecoder(pipe).Decode(&reply); err != nil || reply.Error != "" {
		t.Fatal(reply, err)
	}
	pipe.Close()
	eventually(t, func() bool { _, err := os.Stat(release + ".started"); return err == nil })
	if active, err := sessionlock.Active(dir); err != nil || !active {
		t.Fatal(active, err)
	}
	if err := os.WriteFile(release, []byte("exit"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	ps, err := r.PendingSessions(ctx)
	if err != nil || len(ps) != 0 {
		t.Fatal(ps, err)
	}
	s, err := r.ReadSessionSnapshot(ctx, []string{"g"})
	if err != nil || len(s.Sessions) != 0 || s.Games[0].TimePlayed != 5 || s.Games[0].LastLaunchedAt == 0 {
		t.Fatal(s, err)
	}
}
