package launcher

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"game-launcher/internal/sessionlock"
)

func TestUntrackedLaunchParent(t *testing.T) {
	dir := os.Getenv("PLAUNCHER_TEST_UNTRACKED_PARENT")
	if dir == "" {
		return
	}
	t.Setenv("PLAUNCHER_TEST_GAME", filepath.Join(dir, "release"))
	self, _ := os.Executable()
	if err := NewController().LaunchUntracked(self, dir); err != nil {
		t.Fatal(err)
	}
}

func TestUntrackedSurvivesParentWithoutSessions(t *testing.T) {
	r, dir := monitorRepo(t)
	self, _ := os.Executable()
	parent := exec.Command(self, "-test.run=^TestUntrackedLaunchParent$")
	parent.Env = append(os.Environ(), "PLAUNCHER_TEST_UNTRACKED_PARENT="+dir)
	if out, err := parent.CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	marker := filepath.Join(dir, "release")
	defer func() {
		os.WriteFile(marker, []byte("exit"), 0600)
		if runtime.GOOS == "windows" {
			b, _ := os.ReadFile(marker + ".started")
			pid, _ := strconv.Atoi(string(b))
			if pid > 0 {
				if p, err := os.FindProcess(pid); err == nil {
					p.Wait()
				}
			}
		}
	}()
	eventually(t, func() bool { _, e := os.Stat(marker + ".started"); return e == nil })
	if active, err := sessionlock.Active(dir); err != nil || active {
		t.Fatal(active, err)
	}
	ps, err := r.PendingSessions(context.Background())
	if err != nil || len(ps) != 0 {
		t.Fatal(ps, err)
	}
	s, err := r.ReadSessionSnapshot(context.Background(), []string{"g"})
	if err != nil || len(s.Sessions) != 0 || s.Games[0].TimePlayed != 5 || s.Games[0].LastLaunchedAt != 0 {
		t.Fatal(s, err)
	}
}

func TestUntrackedAssociationAndStartFailure(t *testing.T) {
	dir := t.TempDir()
	called := ""
	c := &Controller{associate: func(path, folder string) error {
		if folder != dir {
			t.Fatal(folder)
		}
		called = path
		return nil
	}}
	for _, name := range []string{"guide.pdf", "config.txt", "guide & notes.html"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("fixture"), 0755); err != nil {
			t.Fatal(err)
		}
		if CanTrack(path) {
			t.Fatal("document with exec bit classified direct", path)
		}
		if err := c.LaunchUntracked(path, dir); err != nil || called != path {
			t.Fatal(called, err)
		}
	}
	c.associate = func(string, string) error { return errors.New("fake opener failure") }
	if c.LaunchUntracked(filepath.Join(dir, "guide.pdf"), dir) == nil {
		t.Fatal("opener error hidden")
	}
	self, _ := os.Executable()
	c.command = func(string, ...string) *exec.Cmd { return exec.Command(filepath.Join(dir, "nonexistent.exe")) }
	if c.LaunchUntracked(self, dir) == nil {
		t.Fatal("start error hidden")
	}
}
