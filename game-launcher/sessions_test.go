package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"game-launcher/internal/apperr"
	"game-launcher/internal/db"
	"game-launcher/internal/launcher"
	"game-launcher/internal/models"
	"game-launcher/internal/sessionlock"
)

func appSession(t *testing.T, a *App, id, game string, seconds int64) {
	t.Helper()
	r := a.repo.(*db.SQLiteRepo)
	if err := r.BeginSession(a.ctx, db.PendingSession{ID: id, GameID: game}); err != nil {
		t.Fatal(err)
	}
	if ok, err := r.StartSession(a.ctx, id, time.Now().Add(-2*time.Hour).Unix()); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err := r.CheckpointSession(a.ctx, id, seconds, false); err != nil {
		t.Fatal(err)
	}
}

func TestStatisticsScopePrivacyAndValidation(t *testing.T) {
	a := newTestApp(t)
	for _, g := range []*models.Game{{ID: "visible", TimePlayed: 10, FolderPath: t.TempDir(), SizeBytes: 100, SizeCheckedAt: 1}, {ID: "hidden", TimePlayed: 999, FolderPath: t.TempDir(), SizeBytes: 9999, SizeCheckedAt: 1}} {
		if err := a.repo.SaveGame(a.ctx, g); err != nil {
			t.Fatal(err)
		}
	}
	appSession(t, a, "v", "visible", 120)
	appSession(t, a, "h", "hidden", 600)
	r := a.repo.(*db.SQLiteRepo)
	r.FinishSession(a.ctx, "v", false)
	r.FinishSession(a.ctx, "h", false)
	s, err := a.GetLibraryStats([]string{"visible", "visible", "unknown"}, "all", "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if s.AllTimeSeconds != 12*60 || s.Disk.Bytes != 100 || len(s.TopGames) != 1 || s.TopGames[0].GameID != "visible" {
		t.Fatal(s)
	}
	if g, err := a.GetGameSessions("hidden", []string{"visible"}, "UTC"); err != nil || g != nil {
		t.Fatal("hidden game leaked", g, err)
	}
	g, err := a.GetGameSessions("visible", []string{"visible"}, "UTC")
	if err != nil || g.Count != 1 || g.LongestSeconds != 120 || len(g.Weeks) != 8 {
		t.Fatal(g, err)
	}
	empty, err := a.GetLibraryStats(nil, "month", "UTC")
	if err != nil || empty.AllTimeSeconds != 0 || len(empty.TopGames) != 0 {
		t.Fatal(empty, err)
	}
	reveal, err := a.GetLibraryStats([]string{"visible", "hidden"}, "all", "UTC")
	if err != nil || reveal.AllTimeSeconds != 1021*60 {
		t.Fatal(reveal, err)
	}
	if _, err := a.GetLibraryStats(nil, "bad", "UTC"); apperr.Code(err) != "sessions.period" {
		t.Fatal(err)
	}
	if _, err := a.GetLibraryStats(nil, "week", "invalid/zone"); apperr.Code(err) != "sessions.timezone" {
		t.Fatal(err)
	}
	a.priv.mu.Lock()
	a.priv.locked = true
	a.priv.mu.Unlock()
	for _, app := range []*App{a, {ctx: context.Background()}} {
		if s, err := app.GetLibraryStats([]string{"hidden"}, "bad", "bad"); s != nil || err != nil {
			t.Fatal(s, err)
		}
		if s, err := app.GetGameSessions("hidden", []string{"hidden"}, "bad"); s != nil || err != nil {
			t.Fatal(s, err)
		}
		if rev, err := app.GetSessionRevision(); rev != nil || err != nil {
			t.Fatal(rev, err)
		}
	}
}

type lockingSnapshotRepo struct {
	db.GameRepository
	lock func()
}

func (r lockingSnapshotRepo) ReadSessionSnapshot(ctx context.Context, ids []string) (*db.SessionSnapshot, error) {
	s, err := r.GameRepository.ReadSessionSnapshot(ctx, ids)
	r.lock()
	return s, err
}

func TestStatsDiscardResponseAfterLock(t *testing.T) {
	for _, game := range []bool{false, true} {
		a := newTestApp(t)
		a.repo.SaveGame(a.ctx, &models.Game{ID: "g", TimePlayed: 100})
		original := a.repo
		a.repo = lockingSnapshotRepo{original, func() { a.priv.mu.Lock(); a.priv.locked = true; a.priv.mu.Unlock() }}
		if game {
			s, err := a.GetGameSessions("g", []string{"g"}, "UTC")
			if s != nil || err != nil {
				t.Fatal(s, err)
			}
		} else {
			s, err := a.GetLibraryStats([]string{"g"}, "all", "UTC")
			if s != nil || err != nil {
				t.Fatal(s, err)
			}
		}
		a.repo = original
	}
}

func TestActiveMonitorGuardsDataAndUpdate(t *testing.T) {
	a := newTestApp(t)
	a.launcher = launcher.NewController()
	a.repo.SaveGame(a.ctx, &models.Game{ID: "g"})
	appSession(t, a, "active", "g", 90)
	l, err := sessionlock.Try(a.dataDir, "run-active")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for name, fn := range map[string]func() error{
		"import":    func() error { _, err := a.ImportLibrary("does-not-exist.zip"); return err },
		"move":      func() error { return a.ChangeDataDir(filepath.Join(t.TempDir(), "destination")) },
		"configure": func() error { return a.ConfigureDataDir(t.TempDir()) },
		"update":    func() error { return a.ApplyLauncherUpdate(false) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := fn(); apperr.Code(err) != "sessions.busy" {
				t.Fatal(err)
			}
		})
	}
	if a.GetLauncherRestartBlocker() != "game" {
		t.Fatal("reopened launcher missed monitor")
	}
	if applyPendingAtStartup(a.dataDir) {
		t.Fatal("startup installed over active monitor")
	}
	// Removing the game doesn't release the monitor's lifetime lock.
	if err := a.RemoveGame("g"); err != nil {
		t.Fatal(err)
	}
	if release, err := a.guardSessionData(); err == nil {
		release()
		t.Fatal("deleted running game allowed data move")
	}
	l.Close()
	release, err := a.guardSessionData()
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestAllAppRemovalPathsClearSessions(t *testing.T) {
	for name, remove := range map[string]func(*App) error{
		"single": func(a *App) error { return a.RemoveGame("g") },
		"bulk":   func(a *App) error { _, err := a.RemoveGames([]string{"g"}); return err },
		"ignore": func(a *App) error { _, err := a.IgnoreGames([]string{"g"}); return err },
		"clear":  func(a *App) error { return a.ClearData() },
	} {
		t.Run(name, func(t *testing.T) {
			a := newTestApp(t)
			a.repo.SaveGame(a.ctx, &models.Game{ID: "g", FolderPath: t.TempDir()})
			appSession(t, a, "done", "g", 90)
			r := a.repo.(*db.SQLiteRepo)
			r.FinishSession(a.ctx, "done", false)
			appSession(t, a, "active", "g", 90)
			if err := remove(a); err != nil {
				t.Fatal(err)
			}
			r.CheckpointSession(a.ctx, "active", 200, true)
			r.FinishSession(a.ctx, "active", false)
			s, err := r.ReadSessionSnapshot(a.ctx, []string{"g"})
			ps, e := r.PendingSessions(a.ctx)
			if err != nil || e != nil || len(s.Games) != 0 || len(s.Sessions) != 0 || len(ps) != 0 {
				t.Fatal(s, ps, err, e)
			}
		})
	}
}

func TestSessionAppBackupRoundTrip(t *testing.T) {
	src := newTestApp(t)
	src.repo.SaveGame(src.ctx, &models.Game{ID: "g", TimePlayed: 20})
	appSession(t, src, "done", "g", 90)
	src.repo.(*db.SQLiteRepo).FinishSession(src.ctx, "done", false)
	appSession(t, src, "pending", "g", 120)
	archive := filepath.Join(t.TempDir(), "sessions.zip")
	snap := filepath.Join(t.TempDir(), "snapshot.db")
	if err := src.repo.(*db.SQLiteRepo).BackupTo(src.ctx, snap); err != nil {
		t.Fatal(err)
	}
	if err := writeBackupZip(archive, []byte(`{"app":"pLauncher"}`), snap, src.dataDir); err != nil {
		t.Fatal(err)
	}
	dst := newTestApp(t)
	if _, err := dst.ImportLibrary(archive); err != nil {
		t.Fatal(err)
	}
	s, err := dst.repo.ReadSessionSnapshot(dst.ctx, []string{"g"})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Sessions) != 2 || s.Games[0].TimePlayed != 23 {
		t.Fatal(s)
	}
	interrupted := 0
	for _, p := range s.Sessions {
		if p.Interrupted {
			interrupted++
		}
	}
	if interrupted != 1 {
		t.Fatal("pending import wasn't marked interrupted")
	}
}
