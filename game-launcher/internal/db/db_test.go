package db

import (
	"context"
	"path/filepath"
	"testing"

	"game-launcher/internal/models"
)

// Личные поля (статус, оценка, заметки) сохраняются, читаются и обновляются при повторном сохранении.
func TestPersonalFieldsRoundTrip(t *testing.T) {
	repo, err := NewSQLiteRepo(filepath.Join(t.TempDir(), "games.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	ctx := context.Background()

	g := &models.Game{ID: "g1", Title: "Test", FolderPath: "D:/Games/Test", Status: "playing", Rating: 4, Notes: "save before day 23", CoverZoom: 1.5}
	if err := repo.SaveGame(ctx, g); err != nil {
		t.Fatal(err)
	}
	got := mustGet(t, repo, "g1")
	if got.Status != "playing" || got.Rating != 4 || got.Notes != "save before day 23" || got.CoverZoom != 1.5 {
		t.Fatalf("read back %q/%d/%q", got.Status, got.Rating, got.Notes)
	}

	got.Status, got.Rating, got.Notes = "finished", 5, ""
	if err := repo.SaveGame(ctx, got); err != nil {
		t.Fatal(err)
	}
	got = mustGet(t, repo, "g1")
	if got.Status != "finished" || got.Rating != 5 || got.Notes != "" {
		t.Fatalf("after update %q/%d/%q", got.Status, got.Rating, got.Notes)
	}

	// Старая строка без этих колонок (NULL) читается как пустые значения
	if err := repo.SaveGame(ctx, &models.Game{ID: "old", Title: "Old", FolderPath: "D:/Games/Old"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.db.Exec(`UPDATE games SET status = NULL, rating = NULL, notes = NULL WHERE id = 'old'`); err != nil {
		t.Fatal(err)
	}
	old := mustGet(t, repo, "old")
	if old.Status != "" || old.Rating != 0 || old.Notes != "" {
		t.Fatalf("old row %q/%d/%q", old.Status, old.Rating, old.Notes)
	}
}

func mustGet(t *testing.T, repo *SQLiteRepo, id string) *models.Game {
	t.Helper()
	games, err := repo.GetAllGames(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range games {
		if g.ID == id {
			return g
		}
	}
	t.Fatalf("game %s not found", id)
	return nil
}
