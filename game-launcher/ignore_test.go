package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreFolders(t *testing.T) {
	a := newTestApp(t)
	root := t.TempDir()
	game := filepath.Join(root, "Game")
	videos := filepath.Join(root, "Videos")
	os.MkdirAll(game, 0755)
	os.MkdirAll(videos, 0755)
	a.AddScanPath(root)

	if n, _ := a.ScanAllFolders(); n != 2 {
		t.Fatalf("первый скан: %d игр, want 2", n)
	}
	games, _ := a.GetGames()
	var videoID string
	for _, g := range games {
		if g.FolderPath == videos {
			videoID = g.ID
		}
	}

	// Игнор: убирается из лаунчера и не возвращается при скане
	if n, err := a.IgnoreGames([]string{videoID}); err != nil || n != 1 {
		t.Fatalf("IgnoreGames = %d, %v", n, err)
	}
	if n, _ := a.ScanAllFolders(); n != 0 {
		t.Fatalf("игнорируемая папка вернулась при скане (+%d)", n)
	}
	if got := a.GetIgnoredPaths(); len(got) != 1 || got[0] != videos {
		t.Fatalf("список игнора = %v", got)
	}

	// Повтор не создаёт дубликатов
	a.AddIgnoredPath(videos)
	if got := a.GetIgnoredPaths(); len(got) != 1 {
		t.Fatalf("дубликат в списке игнора: %v", got)
	}

	// Добавление из настроек убирает уже добавленную игру
	if n, err := a.AddIgnoredPath(game); n != 1 {
		t.Errorf("AddIgnoredPath должен убрать игру из библиотеки, убрано %d (%v)", n, err)
	}

	// Убрали из игнора — скан добавляет снова
	a.RemoveIgnoredPath(videos)
	if n, _ := a.ScanAllFolders(); n != 1 {
		t.Fatalf("после снятия игнора скан добавил %d, want 1", n)
	}

	// Перетаскивание игнорируемой папки: добавляет и снимает игнор
	if n, _ := a.AddGamesFromDrop([]string{game}); n != 1 {
		t.Fatalf("drop игнорируемой папки: добавлено %d, want 1", n)
	}
	if got := a.GetIgnoredPaths(); len(got) != 0 {
		t.Errorf("после drop папка осталась в игноре: %v", got)
	}
}
