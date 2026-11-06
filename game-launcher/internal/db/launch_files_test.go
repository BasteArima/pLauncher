package db

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"game-launcher/internal/apperr"
	"game-launcher/internal/models"
)

func TestLaunchFilesPersistenceAndMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "games.db")
	r, err := NewSQLiteRepo(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r != nil {
			r.Close()
		}
	}()
	ctx := context.Background()
	g := &models.Game{ID: "g", ExecPath: "main.exe", ExecLabel: "Game", Notes: "keep", LaunchFiles: []models.LaunchFile{{Path: "guide.pdf", Label: "Hints"}, {Path: "config.exe"}}}
	if err := r.SaveGame(ctx, g); err != nil {
		t.Fatal(err)
	}
	got := mustGet(t, r, "g")
	date := got.NotesUpdatedAt
	if !reflect.DeepEqual(got.LaunchFiles, g.LaunchFiles) || got.ExecLabel != "Game" {
		t.Fatal(got)
	}
	got.ExecPath, got.ExecLabel = "guide.pdf", "Hints"
	got.LaunchFiles[0] = models.LaunchFile{Path: "main.exe", Label: "Game"}
	if err := r.SaveGame(ctx, got); err != nil {
		t.Fatal(err)
	}
	got = mustGet(t, r, "g")
	if got.ExecPath != "guide.pdf" || got.LaunchFiles[0].Label != "Game" || got.NotesUpdatedAt != date {
		t.Fatal(got)
	}
	got.ExecPath = ""
	got.ExecLabel = ""
	if err := r.SaveGame(ctx, got); err != nil {
		t.Fatal(err)
	}
	got = mustGet(t, r, "g")
	if got.ExecPath != "" || len(got.LaunchFiles) != 2 {
		t.Fatal("primary removal promoted an extra", got)
	}
	got.LaunchFiles = nil
	got.Title = "legacy client save"
	if err := r.SaveGame(ctx, got); err != nil {
		t.Fatal(err)
	}
	if len(mustGet(t, r, "g").LaunchFiles) != 2 {
		t.Fatal("omitted array erased extras")
	}
	got.LaunchFiles = []models.LaunchFile{}
	if err := r.SaveGame(ctx, got); err != nil {
		t.Fatal(err)
	}
	if len(mustGet(t, r, "g").LaunchFiles) != 0 {
		t.Fatal("explicit empty did not clear")
	}
	if _, err := r.db.Exec(`UPDATE games SET exec_label=NULL, launch_files=NULL WHERE id='g'`); err != nil {
		t.Fatal(err)
	}
	if v := mustGet(t, r, "g"); v.ExecLabel != "" || len(v.LaunchFiles) != 0 {
		t.Fatal(v)
	}
	for _, col := range []string{"exec_label", "launch_files"} {
		if _, err := r.db.Exec(`ALTER TABLE games DROP COLUMN ` + col); err != nil {
			t.Fatal(err)
		}
	}
	r.Close()
	r, err = NewSQLiteRepo(path)
	if err != nil {
		t.Fatal(err)
	}
	if v := mustGet(t, r, "g"); v.ExecLabel != "" || len(v.LaunchFiles) != 0 {
		t.Fatal("migration", v)
	}
	if err := r.SaveGame(ctx, g); err != nil {
		t.Fatal(err)
	}
	r.Close()
	r, err = NewSQLiteRepo(path)
	if err != nil {
		t.Fatal(err)
	}
	results, err := r.SearchGames(ctx, "")
	if err != nil || len(results) != 1 || !reflect.DeepEqual(results[0].LaunchFiles, g.LaunchFiles) {
		t.Fatal(results, err)
	}
}

func TestLaunchFilesRelinkOverflowAndRollback(t *testing.T) {
	r, err := NewSQLiteRepo(filepath.Join(t.TempDir(), "games.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()
	g := &models.Game{ID: "g", ExecPath: "main.exe", ExecLabel: "Keep"}
	d := &models.Game{ID: "d", ExecPath: "other.exe", ExecLabel: "Other"}
	for i := 0; i < 8; i++ {
		g.LaunchFiles = append(g.LaunchFiles, models.LaunchFile{Path: fmt.Sprintf("%d.txt", i)})
	}
	d.LaunchFiles = []models.LaunchFile{{Path: "extra.txt", Label: "Extra"}, {Path: "0.txt", Label: "Fill empty"}}
	for _, game := range []*models.Game{g, d} {
		if err := r.SaveGame(ctx, game); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.SetSetting(ctx, "collections", "broken-json"); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveRelink(ctx, g, "d"); err == nil {
		t.Fatal("bad collections didn't roll back")
	}
	if len(mustGet(t, r, "g").LaunchFiles) != 8 || mustGet(t, r, "d").ExecLabel != "Other" {
		t.Fatal("partial merge")
	}
	r.SetSetting(ctx, "collections", "[]")
	g = mustGet(t, r, "g")
	if err := r.SaveRelink(ctx, g, "d"); err != nil {
		t.Fatal(err)
	}
	got := mustGet(t, r, "g")
	if len(got.LaunchFiles) != 10 || got.ExecLabel != "Keep" || got.LaunchFiles[0].Label != "Fill empty" {
		t.Fatal(got)
	}
	got.Title = "unrelated edit"
	if err := r.SaveGame(ctx, got); err != nil {
		t.Fatal("existing overflow rejected", err)
	}
	got.LaunchFiles[0].Path = "new.txt"
	if apperr.Code(r.SaveGame(ctx, got)) != "launch.limit" {
		t.Fatal("new path accepted while over capacity")
	}
	if mustGet(t, r, "g").LaunchFiles[0].Path == "new.txt" {
		t.Fatal("failed save wrote data")
	}
	got = mustGet(t, r, "g")
	got.LaunchFiles = got.LaunchFiles[:8]
	if err := r.SaveGame(ctx, got); err != nil {
		t.Fatal(err)
	}
	got.LaunchFiles[1].Path = got.ExecPath
	if apperr.Code(r.SaveGame(ctx, got)) != "launch.duplicate" {
		t.Fatal("duplicate accepted")
	}
}
