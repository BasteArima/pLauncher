package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

// makeFiles создаёт во временной папке указанные файлы (с промежуточными каталогами).
func makeFiles(t *testing.T, paths ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, p := range paths {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestDetectEngine(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  string
	}{
		{"renpy", []string{"renpy/__init__.py", "game/script.rpa", "Game.exe"}, "Ren'Py"},
		{"renpy_rpa_only", []string{"game/data.rpa", "lib/python.exe"}, "Ren'Py"},
		{"rpgm_mv", []string{"nw.dll", "package.json", "www/data/Map001.json", "www/img/x.png", "index.html"}, "RPG Maker"},
		{"rpgm_vxace", []string{"Game.exe", "Game.ini", "Game.rgss3a"}, "RPG Maker"},
		{"wolf", []string{"Game.exe", "Data/BasicData/x.dat", "GuruguruSMF4.dll"}, "Wolf RPG"},
		{"kirikiri", []string{"data.xp3", "Game.exe"}, "KiriKiri"},
		{"unity", []string{"Game.exe", "UnityPlayer.dll", "Game_Data/globalgamemanagers"}, "Unity"},
		{"unreal", []string{"Game/Content/Paks/pakchunk0.pak", "Engine/Binaries/Win64/x.dll"}, "Unreal Engine"},
		{"godot", []string{"Game.exe", "Game.pck"}, "Godot"},
		{"gamemaker", []string{"Game.exe", "data.win"}, "Game Maker"},
		{"construct", []string{"nw.dll", "c3runtime.js", "data.json", "index.html"}, "Construct"},
		{"flash", []string{"game.swf", "player.exe"}, "Flash"},
		{"qsp", []string{"game.qsp", "qspgui.exe"}, "QSP"},
		{"html", []string{"index.html", "css/style.css"}, "HTML"},
		{"unknown", []string{"readme.txt", "setup.bin"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := makeFiles(t, c.files...)
			if got := DetectEngine(root); got != c.want {
				t.Errorf("DetectEngine(%s) = %q, want %q", c.name, got, c.want)
			}
		})
	}
}
