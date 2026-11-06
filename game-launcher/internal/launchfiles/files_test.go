package launchfiles

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"

	"game-launcher/internal/apperr"
	"game-launcher/internal/models"
)

func TestLabels(t *testing.T) {
	for _, tc := range []struct {
		name, key string
		hint      bool
	}{
		{"Game_Config.exe", "launch.label_settings", true}, {"settings-x64.exe", "launch.label_settings", true},
		{"GameSettings.exe", "launch.label_settings", true}, {"Walkthrough_EN.PDF", "launch.label_walkthrough", true},
		{"my-guide.txt", "launch.label_walkthrough", true}, {"GuidebookishGame.exe", "", false},
		{"reconfigure.exe", "", false}, {"setup.exe", "", true}, {"manual.pdf", "", false},
		{"guide_settings.html", "launch.label_walkthrough", true},
	} {
		if LabelKey(tc.name) != tc.key || DocumentHint(tc.name) != tc.hint {
			t.Errorf("%s: %q", tc.name, LabelKey(tc.name))
		}
	}
	if f := Describe("config.exe", "My utility", ""); f.LabelKey != "" || f.Label != "My utility" {
		t.Fatal(f)
	}
}

func TestIdentityValidationAndMerge(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "Game.exe")
	alias := dir + string(filepath.Separator) + "sub" + string(filepath.Separator) + ".." + string(filepath.Separator) + "Game.exe"
	if Key(p) != Key(alias) {
		t.Fatal("clean identity")
	}
	if (Key(p) == Key(filepath.Join(dir, "game.exe"))) != (runtime.GOOS == "windows") {
		t.Fatal("platform case semantics")
	}
	g := &models.Game{ExecPath: p, ExecLabel: "Main", LaunchFiles: []models.LaunchFile{{Path: alias}}}
	if apperr.Code(Validate(g, nil, true)) != "launch.duplicate" {
		t.Fatal("duplicate accepted")
	}
	g.LaunchFiles = []models.LaunchFile{{Path: " "}}
	if apperr.Code(Validate(g, nil, true)) != "launch.invalid_path" {
		t.Fatal("blank accepted")
	}
	g.LaunchFiles = []models.LaunchFile{{Path: "guide.txt", Label: "Keep"}}
	dup := &models.Game{ExecPath: "other.exe", ExecLabel: "Other", LaunchFiles: []models.LaunchFile{{Path: "guide.txt", Label: "Discard"}, {Path: "config.exe", Label: "Config"}}}
	Merge(g, dup)
	if g.ExecLabel != "Main" || len(g.LaunchFiles) != 3 || g.LaunchFiles[0].Label != "Keep" || g.LaunchFiles[1].Path != "other.exe" {
		t.Fatal(g)
	}
	g.ExecPath = ""
	g.ExecLabel = ""
	SetDetectedPrimary(g, "config.exe")
	if g.ExecLabel != "Config" || len(g.LaunchFiles) != 2 {
		t.Fatal(g)
	}
	old, newDir := filepath.Join(dir, "old"), filepath.Join(dir, "new")
	if RebaseExtra(old, filepath.Join(old, "docs", "guide.pdf"), newDir) != filepath.Join(newDir, "docs", "guide.pdf") {
		t.Fatal("rebase")
	}
	if RebaseExtra(old, p, newDir) != p {
		t.Fatal("external changed")
	}
	for len(g.LaunchFiles) < 10 {
		g.LaunchFiles = append(g.LaunchFiles, models.LaunchFile{Path: fmt.Sprintf("%d.txt", len(g.LaunchFiles))})
	}
	if apperr.Code(Validate(g, nil, true)) != "launch.limit" {
		t.Fatal("new overflow accepted")
	}
	prior := *g
	if err := Validate(g, &prior, true); err != nil {
		t.Fatal(err)
	}
	g.LaunchFiles = append([]models.LaunchFile{}, g.LaunchFiles...)
	g.LaunchFiles[0].Path = "new.txt"
	if apperr.Code(Validate(g, &prior, true)) != "launch.limit" {
		t.Fatal("addition in overflow accepted")
	}
}

func fixtureFile(t *testing.T, root, name string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("fixture"), 0755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDiscovery(t *testing.T) {
	dir := t.TempDir()
	want := []string{"Game.exe", "GameConfig.pdf", "config.exe", "docs/guide.pdf", "docs/nested/walkthrough.txt", "index.html", "setup.exe"}
	for _, f := range append(append([]string{}, want...), "readme.txt", "manual.pdf", "guidebook.txt", "docs/deep/deeper/guide.pdf", "images/guide.pdf", "assets/guide.pdf", "unins000.exe", "CrashReporter.exe", ".hidden.exe", "noise.png") {
		fixtureFile(t, dir, f)
	}
	got, err := Find(context.Background(), dir)
	if err != nil || got.Incomplete {
		t.Fatal(got, err)
	}
	names := []string{}
	for _, f := range got.Files {
		names = append(names, filepath.ToSlash(f.RelativePath))
	}
	sort.Slice(want, func(i, j int) bool { return Key(filepath.Join(dir, want[i])) < Key(filepath.Join(dir, want[j])) })
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v want %v", names, want)
	}
	for _, tc := range []struct {
		lim    limits
		reason string
	}{
		{limits{1, 200, time.Second}, "entries"}, {limits{10000, 1, time.Second}, "candidates"}, {limits{10000, 200, -1}, "time"},
	} {
		r, e := find(context.Background(), dir, tc.lim)
		if e != nil || !r.Incomplete || r.Reason != tc.reason {
			t.Fatal(r, e)
		}
	}
	if _, err := Find(context.Background(), filepath.Join(dir, "gone")); err == nil {
		t.Fatal("missing root accepted")
	}
	if _, err := Find(context.Background(), filepath.Join(dir, "Game.exe")); err == nil {
		t.Fatal("file root accepted")
	}
	if Availability(filepath.Join(dir, "gone")) != "missing" || Availability(dir) != "unsupported" || Availability(filepath.Join(dir, "Game.exe")) != "available" {
		t.Fatal("availability")
	}
}

func TestDiscoverySymlinkAndUnreadable(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	fixtureFile(t, outside, "guide.pdf")
	if err := os.Symlink(outside, filepath.Join(dir, "linked")); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	r, err := Find(context.Background(), dir)
	if err != nil || len(r.Files) != 0 {
		t.Fatal(r, err)
	}
}

func TestUnreadableRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission test; Windows uses ACLs")
	}
	dir := t.TempDir()
	os.Chmod(dir, 0000)
	defer os.Chmod(dir, 0755)
	if f, err := os.Open(dir); err == nil {
		f.Close()
		t.Skip("test process can bypass permissions")
	}
	if _, err := Find(context.Background(), dir); err == nil {
		t.Fatal("unreadable root accepted")
	}
}
