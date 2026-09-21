package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"game-launcher/internal/db"
	"game-launcher/internal/models"
)

func TestPortableMediaPath(t *testing.T) {
	cases := map[string]string{
		"covers/a.jpg":                    "covers/a.jpg",
		`C:\Users\x\data\covers\id\a.jpg`: "covers/id/a.jpg",
		"data/covers/a.jpg":               "covers/a.jpg",
		"":                                "",
		"https://example.com/img.jpg":     "https://example.com/img.jpg",
	}
	for in, want := range cases {
		if got := portableMediaPath(in); got != want {
			t.Errorf("portableMediaPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// Экспорт из одной папки данных и импорт в другую: игры, коллекции и обложки
// переезжают, абсолютные пути медиа становятся относительными.
func TestBackupRoundTrip(t *testing.T) {
	ctx := context.Background()
	srcDir := t.TempDir()
	src := &App{ctx: ctx}
	if err := src.initServices(srcDir); err != nil {
		t.Fatal(err)
	}
	mkfile(t, filepath.Join(srcDir, "covers", "g1", "cover.jpg"))
	g := &models.Game{
		ID: "g1", Title: "Test Game", FolderPath: `D:\Games\Test`,
		CoverPath: filepath.Join(srcDir, "covers", "g1", "cover.jpg"),
		Images:    []string{}, Languages: []string{}, Tags: []string{"vn"},
	}
	if err := src.repo.SaveGame(ctx, g); err != nil {
		t.Fatal(err)
	}
	if err := src.SaveCollections([]models.Collection{{ID: "c1", Name: "Fav", Type: "manual", GameIDs: []string{"g1"}}}); err != nil {
		t.Fatal(err)
	}

	snap := filepath.Join(t.TempDir(), "snap.db")
	if err := src.repo.(*db.SQLiteRepo).BackupTo(ctx, snap); err != nil {
		t.Fatal(err)
	}
	manifest, _ := json.Marshal(backupManifest{App: "pLauncher", Games: 1})
	archive := filepath.Join(t.TempDir(), "backup.zip")
	if err := writeBackupZip(archive, manifest, snap, srcDir); err != nil {
		t.Fatal(err)
	}

	dstDir := t.TempDir()
	dst := &App{ctx: ctx}
	if err := dst.initServices(dstDir); err != nil {
		t.Fatal(err)
	}
	n, err := dst.ImportLibrary(archive)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("games = %d, want 1", n)
	}
	games, _ := dst.repo.GetAllGames(ctx)
	if len(games) != 1 || games[0].Title != "Test Game" {
		t.Fatalf("imported games = %+v", games)
	}
	if games[0].CoverPath != "covers/g1/cover.jpg" {
		t.Errorf("cover path = %q, want covers/g1/cover.jpg", games[0].CoverPath)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "covers", "g1", "cover.jpg")); err != nil {
		t.Errorf("cover file not imported: %v", err)
	}
	cols, _ := dst.GetCollections()
	if len(cols) != 1 || cols[0].Name != "Fav" {
		t.Errorf("collections = %+v", cols)
	}
	for _, leftover := range []string{".import-tmp", ".pre-import"} {
		if _, err := os.Stat(filepath.Join(dstDir, leftover)); err == nil {
			t.Errorf("%s left behind", leftover)
		}
	}
	dst.shutdown(ctx)
	src.shutdown(ctx)
}

func TestExtractBackupRejectsForeignArchive(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "evil.zip")
	f, _ := os.Create(archive)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("../evil.txt")
	w.Write([]byte("x"))
	zw.Close()
	f.Close()

	out := t.TempDir()
	if err := extractBackup(archive, filepath.Join(out, "x")); err == nil {
		t.Fatal("архив без games.db должен отклоняться")
	}
	if _, err := os.Stat(filepath.Join(out, "evil.txt")); err == nil {
		t.Fatal("zip-slip: файл вышел за пределы папки распаковки")
	}
}
