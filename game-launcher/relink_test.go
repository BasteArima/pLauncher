package main

import (
	"os"
	"path/filepath"
	"testing"

	"game-launcher/internal/models"
)

func mkfile(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestMatchMoved(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "gone")

	// Перенос на другой диск: то же имя папки, регистр не важен
	moved := filepath.Join(root, "D", "My Game")
	os.MkdirAll(moved, 0755)
	g := &models.Game{FolderPath: filepath.Join(old, "my game")}
	if got := matchMoved(g, []string{moved}, map[string]bool{}); got != moved {
		t.Errorf("по имени: got %q, want %q", got, moved)
	}

	// Переименование: уникальный exe по тому же относительному пути
	renamed := filepath.Join(root, "D", "Renamed v2")
	mkfile(t, filepath.Join(renamed, "bin", "Unique.exe"))
	other := filepath.Join(root, "D", "Other")
	os.MkdirAll(other, 0755)
	g2 := &models.Game{
		FolderPath: filepath.Join(old, "Renamed v1"),
		ExecPath:   filepath.Join(old, "Renamed v1", "bin", "Unique.exe"),
	}
	if got := matchMoved(g2, []string{other, renamed}, map[string]bool{}); got != renamed {
		t.Errorf("по exe: got %q, want %q", got, renamed)
	}

	// Типовой exe (RPG Maker Game.exe) не опознаёт игру
	rpg := filepath.Join(root, "D", "Some RPG")
	mkfile(t, filepath.Join(rpg, "Game.exe"))
	g3 := &models.Game{
		FolderPath: filepath.Join(old, "Another RPG"),
		ExecPath:   filepath.Join(old, "Another RPG", "Game.exe"),
	}
	if got := matchMoved(g3, []string{rpg}, map[string]bool{}); got != "" {
		t.Errorf("типовой exe: got %q, want пусто", got)
	}

	// Неоднозначный exe — не угадываем
	dupA := filepath.Join(root, "E", "A")
	dupB := filepath.Join(root, "E", "B")
	mkfile(t, filepath.Join(dupA, "Same.exe"))
	mkfile(t, filepath.Join(dupB, "Same.exe"))
	g4 := &models.Game{FolderPath: filepath.Join(old, "X"), ExecPath: filepath.Join(old, "X", "Same.exe")}
	if got := matchMoved(g4, []string{dupA, dupB}, map[string]bool{}); got != "" {
		t.Errorf("неоднозначно: got %q, want пусто", got)
	}
}

func TestRebaseExe(t *testing.T) {
	root := t.TempDir()
	oldF := filepath.Join(root, "old", "Game")
	newF := filepath.Join(root, "new", "Game")
	mkfile(t, filepath.Join(newF, "sub", "Play.exe"))

	got := rebaseExe(oldF, filepath.Join(oldF, "sub", "Play.exe"), newF)
	if want := filepath.Join(newF, "sub", "Play.exe"); got != want {
		t.Errorf("rebase: got %q, want %q", got, want)
	}

	// exe вне папки игры и всё ещё существует — остаётся как есть
	outside := filepath.Join(root, "tools", "emu.exe")
	mkfile(t, outside)
	if got := rebaseExe(oldF, outside, newF); got != outside {
		t.Errorf("внешний exe: got %q, want %q", got, outside)
	}
}
