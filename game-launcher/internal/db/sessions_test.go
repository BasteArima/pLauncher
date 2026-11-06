package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"game-launcher/internal/models"
)

var sessionCtx = context.Background()

func sessionRepo(t *testing.T) *SQLiteRepo {
	t.Helper()
	r, err := NewSQLiteRepo(filepath.Join(t.TempDir(), "games.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if err := r.SaveGame(sessionCtx, &models.Game{ID: "g", TimePlayed: 120, LastLaunchedAt: 10, Notes: "keep"}); err != nil {
		t.Fatal(err)
	}
	return r
}

func pending(t *testing.T, r *SQLiteRepo, id, game string, seconds int64) {
	t.Helper()
	if err := r.BeginSession(sessionCtx, PendingSession{ID: id, GameID: game}); err != nil {
		t.Fatal(err)
	}
	if ok, err := r.StartSession(sessionCtx, id, 1700000000); err != nil || !ok {
		t.Fatalf("start: %v %v", ok, err)
	}
	if err := r.CheckpointSession(sessionCtx, id, seconds, false); err != nil {
		t.Fatal(err)
	}
}

func snapshot(t *testing.T, r *SQLiteRepo, ids ...string) *SessionSnapshot {
	t.Helper()
	s, err := r.ReadSessionSnapshot(sessionCtx, ids)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSessionThresholdExactlyOnceAndStaleSave(t *testing.T) {
	for _, seconds := range []int64{0, 59, 60, 61, 90} {
		t.Run(fmt.Sprint(seconds), func(t *testing.T) {
			r := sessionRepo(t)
			stale := snapshot(t, r, "g").Games[0]
			pending(t, r, "s", "g", seconds)
			if err := r.CheckpointSession(sessionCtx, "s", seconds, true); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 3; i++ {
				if err := r.FinishSession(sessionCtx, "s", false); err != nil {
					t.Fatal(err)
				}
			}
			stale.Title = "edited"
			if err := r.SaveGame(sessionCtx, stale); err != nil {
				t.Fatal(err)
			}
			s := snapshot(t, r, "g")
			want := 0
			if seconds >= 60 {
				want = 1
			}
			if len(s.Sessions) != want || s.Games[0].TimePlayed != 120+int(seconds/60) || s.Games[0].LastLaunchedAt != 1700000000 || s.Games[0].Title != "edited" || s.Games[0].Notes != "keep" {
				t.Fatalf("unexpected snapshot: %+v %+v", s, s.Games[0])
			}
			if want == 1 && (s.Sessions[0].DurationSeconds != seconds || s.Sessions[0].Interrupted) {
				t.Fatal(s.Sessions[0])
			}
		})
	}
}

func TestConcurrentCompletionsAndScope(t *testing.T) {
	r := sessionRepo(t)
	for i := 0; i < 12; i++ {
		pending(t, r, fmt.Sprint(i), "g", 90)
	}
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := r.FinishSession(sessionCtx, fmt.Sprint(i%12), false); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	ids := make([]string, 40000)
	for i := range ids {
		ids[i] = "g"
	}
	ids = append(ids, "missing")
	s := snapshot(t, r, ids...)
	if len(s.Games) != 1 || len(s.Sessions) != 12 || s.Games[0].TimePlayed != 132 {
		t.Fatalf("duplicate totals: %+v", s)
	}
	if s := snapshot(t, r); len(s.Games) != 0 || len(s.Sessions) != 0 {
		t.Fatal("empty scope leaked data")
	}
}

func TestSessionRollbackAndRecovery(t *testing.T) {
	r := sessionRepo(t)
	pending(t, r, "s", "g", 90)
	_, err := r.db.Exec(`CREATE TRIGGER fail_time BEFORE UPDATE OF time_played ON games BEGIN SELECT RAISE(ABORT,'test write failure'); END;`)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishSession(sessionCtx, "s", true); err == nil {
		t.Fatal("expected rollback")
	}
	ps, _ := r.PendingSessions(sessionCtx)
	if len(ps) != 1 || len(snapshot(t, r, "g").Sessions) != 0 {
		t.Fatal("partial commit")
	}
	r.db.Exec(`DROP TRIGGER fail_time`)
	if err := r.RecoverImportedSessions(sessionCtx); err != nil {
		t.Fatal(err)
	}
	s := snapshot(t, r, "g")
	if len(s.Sessions) != 1 || !s.Sessions[0].Interrupted || s.Games[0].TimePlayed != 121 {
		t.Fatal(s)
	}
	if err := r.RecoverImportedSessions(sessionCtx); err != nil {
		t.Fatal(err)
	}
	if snapshot(t, r, "g").Games[0].TimePlayed != 121 {
		t.Fatal("double recovery")
	}
}

func TestSessionDeletionAndMerge(t *testing.T) {
	r := sessionRepo(t)
	if err := r.SaveGame(sessionCtx, &models.Game{ID: "dup", TimePlayed: 9, LastLaunchedAt: 1700000010}); err != nil {
		t.Fatal(err)
	}
	cols := []models.Collection{{ID: "c", GameIDs: []string{"g", "dup", "dup"}}}
	raw, _ := json.Marshal(cols)
	r.SetSetting(sessionCtx, "collections", string(raw))
	pending(t, r, "done", "dup", 120)
	r.FinishSession(sessionCtx, "done", false)
	pending(t, r, "active", "dup", 90)
	stale := snapshot(t, r, "g").Games[0]
	pending(t, r, "other", "g", 60)
	r.FinishSession(sessionCtx, "other", false)
	if err := r.SaveRelink(sessionCtx, stale, "dup"); err != nil {
		t.Fatal(err)
	}
	if err := r.FinishSession(sessionCtx, "active", false); err != nil {
		t.Fatal(err)
	}
	s := snapshot(t, r, "g", "dup")
	if len(s.Games) != 1 || len(s.Sessions) != 3 || s.Games[0].TimePlayed != 133 || s.Games[0].LastLaunchedAt != 1700000010 {
		t.Fatalf("bad merge: %+v %+v", s, s.Games[0])
	}
	for _, s := range s.Sessions {
		if s.GameID != "g" {
			t.Fatal("orphan session")
		}
	}
	rawCols, _ := r.GetSetting(sessionCtx, "collections")
	json.Unmarshal([]byte(rawCols), &cols)
	if len(cols[0].GameIDs) != 1 || cols[0].GameIDs[0] != "g" {
		t.Fatal(cols)
	}
	// Retrying an already merged duplicate must not add any time again.
	if err := r.SaveRelink(sessionCtx, stale, "dup"); err != nil {
		t.Fatal(err)
	}
	pending(t, r, "deleted", "g", 90)
	if err := r.DeleteGame(sessionCtx, "g"); err != nil {
		t.Fatal(err)
	}
	r.CheckpointSession(sessionCtx, "deleted", 120, true)
	r.FinishSession(sessionCtx, "deleted", false)
	ps, _ := r.PendingSessions(sessionCtx)
	if len(ps) != 0 || len(snapshot(t, r, "g").Sessions) != 0 || len(snapshot(t, r, "g").Games) != 0 {
		t.Fatal("deletion resurrected game")
	}
}

func TestDeleteAllAndForeignKeysEveryConnection(t *testing.T) {
	r := sessionRepo(t)
	pending(t, r, "s", "g", 70)
	r.FinishSession(sessionCtx, "s", false)
	pending(t, r, "pending", "g", 100)
	c1, err := r.db.Conn(sessionCtx)
	if err != nil {
		t.Fatal(err)
	}
	defer c1.Close()
	c2, err := r.db.Conn(sessionCtx)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	for _, c := range []*sql.Conn{c1, c2} {
		var enabled int
		if err := c.QueryRowContext(sessionCtx, "PRAGMA foreign_keys").Scan(&enabled); err != nil || enabled != 1 {
			t.Fatalf("foreign keys: %d %v", enabled, err)
		}
	}
	if err := r.DeleteAllGames(sessionCtx); err != nil {
		t.Fatal(err)
	}
	ps, _ := r.PendingSessions(sessionCtx)
	if len(ps) != 0 || len(snapshot(t, r, "g").Sessions) != 0 {
		t.Fatal("delete all left history")
	}
}

func TestSessionBackupAndOldMigration(t *testing.T) {
	r := sessionRepo(t)
	pending(t, r, "done", "g", 90)
	r.FinishSession(sessionCtx, "done", false)
	pending(t, r, "active", "g", 120)
	backup := filepath.Join(t.TempDir(), "games.db")
	if err := r.BackupTo(sessionCtx, backup); err != nil {
		t.Fatal(err)
	}
	b, err := NewSQLiteRepo(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if err := b.RecoverImportedSessions(sessionCtx); err != nil {
		t.Fatal(err)
	}
	s := snapshot(t, b, "g")
	if len(s.Sessions) != 2 || s.Games[0].TimePlayed != 123 {
		t.Fatal(s)
	}
	if len(snapshot(t, r, "g").Sessions) != 1 {
		t.Fatal("import changed original")
	}
	// Simulate an archive predating sessions, then reopen twice to prove idempotent migration.
	_, err = r.db.Exec(`DROP TABLE pending_sessions; DROP TABLE play_sessions; DELETE FROM settings WHERE key IN ('sessions_since','sessions_revision')`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := r.initSchema(); err != nil {
			t.Fatal(err)
		}
	}
	s = snapshot(t, r, "g")
	if len(s.Sessions) != 0 || s.Games[0].TimePlayed != 121 || s.HistorySince <= 0 {
		t.Fatal("old library was reset")
	}
}

func TestRelinkRollback(t *testing.T) {
	r := sessionRepo(t)
	r.SaveGame(sessionCtx, &models.Game{ID: "dup", TimePlayed: 5})
	pending(t, r, "s", "dup", 90)
	_, err := r.db.Exec(`CREATE TRIGGER fail_delete BEFORE DELETE ON games BEGIN SELECT RAISE(ABORT,'test delete failure'); END;`)
	if err != nil {
		t.Fatal(err)
	}
	g := snapshot(t, r, "g").Games[0]
	if err := r.SaveRelink(sessionCtx, g, "dup"); err == nil {
		t.Fatal("expected failure")
	}
	ps, _ := r.PendingSessions(sessionCtx)
	s := snapshot(t, r, "g", "dup")
	if len(s.Games) != 2 || ps[0].GameID != "dup" || snapshot(t, r, "g").Games[0].TimePlayed != 120 {
		t.Fatal("partial relink")
	}
}
