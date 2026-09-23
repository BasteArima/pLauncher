package main

import (
	"os"
	"path/filepath"
	"testing"

	"game-launcher/internal/models"
)

func TestFolderSize(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.bin"), make([]byte, 1000), 0644)
	os.MkdirAll(filepath.Join(root, "sub"), 0755)
	os.WriteFile(filepath.Join(root, "sub", "b.bin"), make([]byte, 234), 0644)
	got, err := folderSize(root)
	if err != nil || got != 1234 {
		t.Fatalf("folderSize = %d, %v; want 1234", got, err)
	}
	if _, err := folderSize(filepath.Join(root, "nope")); err == nil {
		t.Error("для несуществующей папки ожидалась ошибка")
	}
}

func TestMassActions(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	withHTML := filepath.Join(dir, "HtmlGame")
	os.MkdirAll(withHTML, 0755)
	os.WriteFile(filepath.Join(withHTML, "index.html"), []byte("x"), 0644)

	for _, g := range []*models.Game{
		{ID: "a", Title: "A", FolderPath: withHTML},
		{ID: "b", Title: "B", FolderPath: dir, ExecPath: "keep.exe"},
		{ID: "c", Title: "C"},
	} {
		if err := a.repo.SaveGame(a.ctx, g); err != nil {
			t.Fatal(err)
		}
	}
	a.SaveCollections([]models.Collection{{ID: "col", Name: "X", Type: "manual", GameIDs: []string{"a", "c"}}})

	// Автопоиск: заполняет только пустой файл запуска
	if n, _ := a.DetectLaunchFiles(nil); n != 1 {
		t.Fatalf("DetectLaunchFiles = %d, want 1", n)
	}
	ga, _ := a.gameByID("a")
	gb, _ := a.gameByID("b")
	if filepath.Base(ga.ExecPath) != "index.html" || gb.ExecPath != "keep.exe" {
		t.Errorf("exec: a=%q b=%q", ga.ExecPath, gb.ExecPath)
	}

	if n, _ := a.SetFavorites([]string{"a", "b"}, true); n != 2 {
		t.Errorf("SetFavorites = %d, want 2", n)
	}
	if n, _ := a.SetFavorites([]string{"a"}, true); n != 0 {
		t.Errorf("повторное избранное не должно ничего менять, got %d", n)
	}

	// Массовое удаление вычищает игры из коллекций
	if n, _ := a.RemoveGames([]string{"a", "c"}); n != 2 {
		t.Fatalf("RemoveGames = %d, want 2", n)
	}
	cols, _ := a.GetCollections()
	if len(cols[0].GameIDs) != 0 {
		t.Errorf("в коллекции остались удалённые игры: %v", cols[0].GameIDs)
	}

	// Размер пишется отдельно и не затирается обычным сохранением
	a.repo.SetGameSize(a.ctx, "b", 555, 1)
	gb, _ = a.gameByID("b")
	gb.SizeBytes = 0
	a.repo.SaveGame(a.ctx, gb)
	gb, _ = a.gameByID("b")
	if gb.SizeBytes != 555 {
		t.Errorf("SaveGame затёр размер: %d", gb.SizeBytes)
	}
}
