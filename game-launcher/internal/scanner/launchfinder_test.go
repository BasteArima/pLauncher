package scanner

import (
	"path/filepath"
	goruntime "runtime"
	"testing"
)

func TestFindOtherLaunchers(t *testing.T) {
	tests := []struct {
		name    string
		files   []string // пути относительно папки игры
		want    string   // ожидаемый путь относительно папки ("" — ничего)
		winOnly bool     // .bat/.cmd запускаются только на Windows
	}{
		{"twine index.html", []string{"index.html", "readme.html"}, "index.html", false},
		{"rpg maker mv www", []string{"www/index.html", "www/js/rpg_core.js", "Credits.html"}, "www/index.html", false},
		{"html named like folder", []string{"Cool Game.html", "manual.html"}, "Cool Game.html", false},
		{"random html ignored", []string{"changelog.html", "notes.html"}, "", false},
		{"start.bat over install.bat", []string{"install.bat", "start.bat"}, "start.bat", true},
		{"qsp beats bat", []string{"game.qsp", "run.bat"}, "game.qsp", false},
		{"jar", []string{"lib/x.jar", "MyGame.jar"}, "MyGame.jar", false},
		{"swf", []string{"game.swf"}, "game.swf", false},
		{"saves skipped", []string{"saves/index.html"}, "", false},
		{"too deep", []string{"a/b/index.html"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.winOnly && goruntime.GOOS != "windows" {
				t.Skip("windows-only launcher type")
			}
			root := filepath.Join(t.TempDir(), "Cool Game")
			for _, f := range tt.files {
				dir, name := filepath.Split(filepath.Join(root, filepath.FromSlash(f)))
				mkExe(t, dir, name, 1)
			}
			got := FindBestExecutable(root)
			want := ""
			if tt.want != "" {
				want = filepath.Join(root, filepath.FromSlash(tt.want))
			}
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// .exe всегда приоритетнее прочих запускалок.
func TestExePreferredOverOthers(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Game")
	mkExe(t, root, "index.html", 1)
	mkExe(t, root, "Game.exe", 8000)
	if got := FindBestExecutable(root); filepath.Base(got) != "Game.exe" {
		t.Errorf("got %q, want Game.exe", got)
	}
}
