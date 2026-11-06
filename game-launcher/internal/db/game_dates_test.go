package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"game-launcher/internal/models"
)

func TestGameDatesPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "games.db")
	r, err := NewSQLiteRepo(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { r.Close() }()
	ctx := context.Background()
	save := func(g *models.Game) {
		t.Helper()
		if err := r.SaveGame(ctx, g); err != nil {
			t.Fatal(err)
		}
	}
	g := &models.Game{ID: "dates", Notes: "first", UpdateAvailable: true, UpdateFoundAt: 123}
	before := time.Now().Unix()
	save(g)
	got := mustGet(t, r, g.ID)
	if got.NotesUpdatedAt < before || got.UpdateFoundAt != 123 {
		t.Fatalf("insert: %+v", got)
	}
	if _, err := r.db.Exec(`UPDATE games SET notes_updated_at=456 WHERE id=?`, g.ID); err != nil {
		t.Fatal(err)
	}
	g.Title, g.Favorite, g.NotesUpdatedAt = "renamed", true, 999
	save(g)
	if got := mustGet(t, r, g.ID); got.NotesUpdatedAt != 456 {
		t.Fatalf("unrelated save changed note date: %+v", got)
	}
	g.Notes = "edited"
	save(g)
	if got := mustGet(t, r, g.ID); got.NotesUpdatedAt < before {
		t.Fatalf("edit not dated: %+v", got)
	}
	if _, err := r.db.Exec(`UPDATE games SET notes_updated_at=456 WHERE id=?`, g.ID); err != nil {
		t.Fatal(err)
	}
	g.Notes, g.UpdateAvailable = "", false
	save(g)
	if got := mustGet(t, r, g.ID); got.NotesUpdatedAt < before || got.UpdateFoundAt != 0 {
		t.Fatalf("clear: %+v", got)
	}
	save(&models.Game{ID: "empty"})
	if got := mustGet(t, r, "empty"); got.NotesUpdatedAt != 0 {
		t.Fatalf("empty note dated: %+v", got)
	}
	// Relinking copies the original note and its date together, without pretending it was edited.
	save(&models.Game{ID: "duplicate", Notes: "merged", NotesUpdatedAt: 789})
	if err := r.SaveRelink(ctx, &models.Game{ID: "empty", Notes: "merged", NotesUpdatedAt: 789}, "duplicate"); err != nil {
		t.Fatal(err)
	}
	if got := mustGet(t, r, "empty"); got.NotesUpdatedAt != 789 || got.Notes != "merged" {
		t.Fatalf("relink: %+v", got)
	}
	if _, err := r.db.Exec(`UPDATE games SET notes_updated_at=NULL, update_found_at=NULL WHERE id=?`, g.ID); err != nil {
		t.Fatal(err)
	}
	got = mustGet(t, r, g.ID)
	if got.NotesUpdatedAt != 0 || got.UpdateFoundAt != 0 {
		t.Fatalf("NULL defaults: %+v", got)
	}
	// Reopen a pre-migration schema; both ALTER TABLE additions must be idempotent.
	for _, col := range []string{"notes_updated_at", "update_found_at"} {
		if _, err := r.db.Exec(`ALTER TABLE games DROP COLUMN ` + col); err != nil {
			t.Fatal(err)
		}
	}
	r.Close()
	r, err = NewSQLiteRepo(path)
	if err != nil {
		t.Fatal(err)
	}
	got = mustGet(t, r, g.ID)
	if got.NotesUpdatedAt != 0 || got.UpdateFoundAt != 0 {
		t.Fatalf("migration defaults: %+v", got)
	}
	g.Notes, g.UpdateAvailable, g.UpdateFoundAt = "saved after migration", true, 1000
	save(g)
	r.Close()
	r, err = NewSQLiteRepo(path)
	if err != nil {
		t.Fatal(err)
	}
	got = mustGet(t, r, g.ID)
	if got.NotesUpdatedAt < before || got.UpdateFoundAt != 1000 {
		t.Fatalf("reopen: %+v", got)
	}
}
