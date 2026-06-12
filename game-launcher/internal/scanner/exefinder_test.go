package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

// mkExe создаёт фейковый .exe заданного размера
func mkExe(t *testing.T, dir, name string, sizeKB int) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, sizeKB*1024)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestFindBestExecutable(t *testing.T) {
	tests := []struct {
		name  string
		setup func(root string, t *testing.T)
		want  string // ожидаемое имя файла (basename)
	}{
		{
			name: "skips vc_redist, picks game exe",
			setup: func(root string, t *testing.T) {
				mkExe(t, root, "MyGame.exe", 8000)
				mkExe(t, root, "vc_redist.x64.exe", 6000)
			},
			want: "MyGame.exe",
		},
		{
			name: "prefers name matching folder over generic",
			setup: func(root string, t *testing.T) {
				mkExe(t, root, "MyGame.exe", 8000)
				mkExe(t, root, "tool.exe", 8000)
			},
			want: "MyGame.exe",
		},
		{
			name: "skips unitycrashhandler",
			setup: func(root string, t *testing.T) {
				mkExe(t, root, "MyGame.exe", 8000)
				mkExe(t, root, "UnityCrashHandler64.exe", 8000)
			},
			want: "MyGame.exe",
		},
		{
			name: "root exe preferred over nested",
			setup: func(root string, t *testing.T) {
				mkExe(t, root, "start.exe", 8000)
				mkExe(t, filepath.Join(root, "engine", "bin"), "core.exe", 8000)
			},
			want: "start.exe",
		},
		{
			name: "finds nested exe when root has none",
			setup: func(root string, t *testing.T) {
				mkExe(t, filepath.Join(root, "bin"), "Game.exe", 8000)
			},
			want: "Game.exe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "MyGame")
			if err := os.MkdirAll(root, 0755); err != nil {
				t.Fatal(err)
			}
			tt.setup(root, t)

			got := FindBestExecutable(root)
			if filepath.Base(got) != tt.want {
				t.Errorf("FindBestExecutable() = %q, want basename %q", got, tt.want)
			}
		})
	}
}

func TestCleanFolderTitle(t *testing.T) {
	cases := map[string]string{
		"My_Game [v1.2] [RUS]": "My Game",
		"Some.Game.Name":       "Some Game Name",
		"Plain Title":          "Plain Title",
		"[OnlyTags]":           "[OnlyTags]",
	}
	for in, want := range cases {
		if got := cleanFolderTitle(in); got != want {
			t.Errorf("cleanFolderTitle(%q) = %q, want %q", in, got, want)
		}
	}
}
