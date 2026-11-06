package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"time"

	"game-launcher/internal/apperr"
	"game-launcher/internal/db"
	"game-launcher/internal/launcher"
	"game-launcher/internal/models"
	"game-launcher/internal/sessionlock"
)

// Child tests only run this harmless file-signal helper, never a user game or OS document viewer.
func TestMain(m *testing.M) {
	if len(os.Args) == 4 && os.Args[1] == launcher.MonitorFlag {
		if err := launcher.RunMonitor(os.Args[2], os.Args[3], os.Stdout); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	if marker := os.Getenv("PLAUNCHER_LAUNCH_FILE_HELPER"); marker != "" {
		_ = os.WriteFile(marker+".started", []byte(strconv.Itoa(os.Getpid())), 0600)
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(marker); err == nil {
				os.Exit(0)
			}
			time.Sleep(10 * time.Millisecond)
		}
		os.Exit(1)
	}
	os.Exit(m.Run())
}

type fakeFileLauncher struct {
	*launcher.Controller
	untracked, associated []string
	fail                  error
}

func (f *fakeFileLauncher) LaunchUntracked(p, dir string) error {
	f.untracked = append(f.untracked, p)
	return f.fail
}
func (f *fakeFileLauncher) LaunchAssociated(p, dir string) error {
	f.associated = append(f.associated, p)
	return f.fail
}

func waitLaunchFile(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for harmless helper")
}

func TestLaunchFilesRoutingAndPrivacy(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	main, extra, doc := filepath.Join(dir, "index.html"), filepath.Join(dir, "config.exe"), filepath.Join(dir, "guide.pdf")
	for _, p := range []string{main, extra, doc} {
		mkfile(t, p)
	}
	g := &models.Game{ID: "g", ExecPath: main, FolderPath: dir, TimePlayed: 7, LastLaunchedAt: 123, LaunchFiles: []models.LaunchFile{{Path: extra}, {Path: doc, Label: "Custom"}}}
	if err := a.repo.SaveGame(a.ctx, g); err != nil {
		t.Fatal(err)
	}
	fake := &fakeFileLauncher{Controller: launcher.NewController()}
	a.launcher = fake
	before, _ := a.repo.ReadSessionSnapshot(a.ctx, []string{"g"})
	for _, p := range []string{extra, doc} {
		if err := a.Launch("g", p, "wrong folder ignored"); err != nil {
			t.Fatal(err)
		}
	}
	after, _ := a.repo.ReadSessionSnapshot(a.ctx, []string{"g"})
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(fake.untracked, []string{extra, doc}) {
		t.Fatal("extra affected recording", after)
	}
	if active, err := sessionlock.Active(a.dataDir); err != nil || active {
		t.Fatal("extra owns game blocker", active, err)
	}
	if err := a.LaunchGameFile("g", main); err != nil {
		t.Fatal(err)
	}
	saved, _ := a.gameByID("g")
	if saved.TimePlayed != 7 || saved.LastLaunchedAt <= 123 || len(fake.associated) != 1 {
		t.Fatal("primary association policy", saved)
	}
	if apperr.Code(a.LaunchGameFile("g", filepath.Join(dir, "unlisted.exe"))) != "launch.unlisted" {
		t.Fatal("unlisted accepted")
	}
	os.Remove(extra)
	if apperr.Code(a.LaunchGameFile("g", extra)) != "launch.missing" {
		t.Fatal("missing extra accepted")
	}
	fake.fail = errors.New("fake open failure")
	if apperr.Code(a.LaunchGameFile("g", doc)) != "launch.failed" {
		t.Fatal("open error not localized")
	}
	files, err := a.GetLaunchFileAvailability("g")
	if err != nil || len(files) != 3 || files[1].Availability != "missing" || files[1].LabelKey != "launch.label_settings" || files[2].LabelKey != "" || files[2].Label != "Custom" {
		t.Fatal(files, err)
	}
	if got := a.GetLaunchFileLabel("walkthrough.txt"); got != "launch.label_walkthrough" {
		t.Fatal(got)
	}
	a.priv.mu.Lock()
	a.priv.locked = true
	a.priv.mu.Unlock()
	for _, app := range []*App{a, {ctx: context.Background()}} {
		if err := app.LaunchGameFile("missing", "missing"); err != nil {
			t.Fatal(err)
		}
		if r, e := app.GetLaunchFileAvailability("missing"); r != nil || e != nil {
			t.Fatal(r, e)
		}
		if r, e := app.FindLaunchFileCandidates("missing"); r != nil || e != nil {
			t.Fatal(r, e)
		}
		if p, e := app.SelectLaunchFile("missing"); p != "" || e != nil {
			t.Fatal(p, e)
		}
		if app.GetLaunchFileLabel("config.exe") != "" {
			t.Fatal("locked label leaked")
		}
	}
}

func TestPrimaryMonitorAndRoleChange(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	self, _ := os.Executable()
	marker := filepath.Join(dir, "release")
	t.Setenv("PLAUNCHER_LAUNCH_FILE_HELPER", marker)
	defer func() {
		os.WriteFile(marker, []byte("exit"), 0600)
		// Windows retains the working-directory handle until the process has exited.
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
	g := &models.Game{ID: "g", ExecPath: self, FolderPath: dir, TimePlayed: 11}
	if err := a.repo.SaveGame(a.ctx, g); err != nil {
		t.Fatal(err)
	}
	if err := a.Launch("g", self, dir); err != nil {
		t.Fatal(err)
	}
	r := a.repo.(*db.SQLiteRepo)
	ps, err := r.PendingSessions(a.ctx)
	if err != nil || len(ps) != 1 || ps[0].Executable != self {
		t.Fatal(ps, err)
	}
	if active, _ := sessionlock.Active(a.dataDir); !active {
		t.Fatal("primary has no monitor")
	}
	// Demoting a running primary must not change that already-started session.
	g.ExecPath = ""
	g.LaunchFiles = []models.LaunchFile{{Path: self}}
	if err := a.UpdateGame(*g); err != nil {
		t.Fatal(err)
	}
	ps, err = r.PendingSessions(a.ctx)
	if err != nil || len(ps) != 1 || ps[0].Executable != self {
		t.Fatal(ps, err)
	}
	os.WriteFile(marker, []byte("exit"), 0600)
	waitLaunchFile(t, func() bool {
		a.recoverSessions()
		p, _ := r.PendingSessions(a.ctx)
		active, _ := sessionlock.Active(a.dataDir)
		return len(p) == 0 && !active
	})
	snapshot, _ := a.repo.ReadSessionSnapshot(a.ctx, []string{"g"})
	if snapshot.Games[0].TimePlayed != 11 || len(snapshot.Sessions) != 0 {
		t.Fatal("short helper incorrectly recorded", snapshot)
	}
	// Same file is now supplementary; no monitor, activity timestamp or session intent may be created.
	prior := snapshot.Games[0].LastLaunchedAt
	os.Remove(marker)
	os.Remove(marker + ".started")
	if err := a.LaunchGameFile("g", self); err != nil {
		t.Fatal(err)
	}
	waitLaunchFile(t, func() bool { _, e := os.Stat(marker + ".started"); return e == nil })
	ps, _ = r.PendingSessions(a.ctx)
	active, _ := sessionlock.Active(a.dataDir)
	saved, _ := a.gameByID("g")
	if len(ps) != 0 || active || saved.LastLaunchedAt != prior || saved.TimePlayed != 11 {
		t.Fatal("demoted file recorded", ps, active, saved)
	}
	os.WriteFile(marker, []byte("exit"), 0600)
}

func TestLaunchFilesRelinkAndScan(t *testing.T) {
	a := newTestApp(t)
	root := t.TempDir()
	old := filepath.Join(root, "old")
	newDir := filepath.Join(root, "new")
	main := filepath.Join(newDir, "game.exe")
	mkfile(t, main)
	guide := filepath.Join(newDir, "docs", "guide.pdf")
	mkfile(t, guide)
	external := filepath.Join(root, "external.pdf")
	mkfile(t, external)
	g := &models.Game{ID: "g", FolderPath: old, ExecPath: filepath.Join(old, "game.exe"), ExecLabel: "Primary", LaunchFiles: []models.LaunchFile{{Path: filepath.Join(old, "docs", "guide.pdf"), Label: "Guide"}, {Path: filepath.Join(old, "missing.txt"), Label: "Missing"}, {Path: external, Label: "External"}}}
	d := &models.Game{ID: "dup", FolderPath: newDir, ExecPath: filepath.Join(newDir, "other.exe"), ExecLabel: "Duplicate primary"}
	for i := 0; i < 8; i++ {
		d.LaunchFiles = append(d.LaunchFiles, models.LaunchFile{Path: filepath.Join(newDir, fmt.Sprintf("extra%d.txt", i))})
	}
	for _, v := range []*models.Game{g, d} {
		if err := a.repo.SaveGame(a.ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	merged, err := a.RelinkGame("g", newDir)
	if err != nil {
		t.Fatal(err)
	}
	if merged.ExecPath != main || merged.ExecLabel != "Primary" || len(merged.LaunchFiles) != 12 || merged.LaunchFiles[0].Path != guide || merged.LaunchFiles[1].Path != filepath.Join(newDir, "missing.txt") || merged.LaunchFiles[2].Path != external {
		t.Fatal(merged)
	}
	if err := a.UpdateGame(*merged); err != nil {
		t.Fatal("overflow not editable", err)
	}
	// Scanner fills only an empty primary and consolidates the same extra, keeping its label.
	merged.ExecPath = ""
	merged.ExecLabel = ""
	merged.LaunchFiles = append(merged.LaunchFiles[:7], models.LaunchFile{Path: main, Label: "Detected label"})
	if err := a.UpdateGame(*merged); err != nil {
		t.Fatal(err)
	}
	if _, err := a.scanner.ScanFolder(a.ctx, root); err != nil {
		t.Fatal(err)
	}
	saved, _ := a.gameByID("g")
	if saved.ExecPath != main || saved.ExecLabel != "Detected label" || len(saved.LaunchFiles) != 7 {
		t.Fatal(saved)
	}
}

type lockOnReadRepo struct {
	db.GameRepository
	app *App
}

func (r *lockOnReadRepo) GetAllGames(ctx context.Context) ([]*models.Game, error) {
	rows, err := r.GameRepository.GetAllGames(ctx)
	r.app.priv.mu.Lock()
	r.app.priv.locked = true
	r.app.priv.mu.Unlock()
	return rows, err
}

func TestLaunchFilesLateLock(t *testing.T) {
	a := newTestApp(t)
	g := &models.Game{ID: "g", FolderPath: t.TempDir(), ExecPath: "private.exe"}
	if err := a.repo.SaveGame(a.ctx, g); err != nil {
		t.Fatal(err)
	}
	original := a.repo
	defer func() { a.repo = original }()
	a.repo = &lockOnReadRepo{GameRepository: original, app: a}
	unlock := func() { a.priv.mu.Lock(); a.priv.locked = false; a.priv.mu.Unlock() }
	if rows, err := a.FindLaunchFileCandidates("g"); rows != nil || err != nil {
		t.Fatal(rows, err)
	}
	unlock()
	if rows, err := a.GetLaunchFileAvailability("g"); rows != nil || err != nil {
		t.Fatal(rows, err)
	}
	unlock()
	if path, err := a.SelectLaunchFile("g"); path != "" || err != nil {
		t.Fatal(path, err)
	}
	unlock()
	if err := a.LaunchGameFile("g", g.ExecPath); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchFilesFailedPrimary(t *testing.T) {
	if os.PathSeparator != '\\' {
		t.Skip("invalid PE launch is a Windows dispatch test")
	}
	a := newTestApp(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.exe")
	mkfile(t, path) // Empty fixture, never a real utility.
	g := &models.Game{ID: "g", FolderPath: dir, ExecPath: path, TimePlayed: 3, LastLaunchedAt: 123}
	if err := a.repo.SaveGame(a.ctx, g); err != nil {
		t.Fatal(err)
	}
	if code := apperr.Code(a.LaunchGameFile("g", path)); code != "sessions.launch" {
		t.Fatal(code)
	}
	ps, err := a.repo.(*db.SQLiteRepo).PendingSessions(a.ctx)
	if err != nil || len(ps) != 0 {
		t.Fatal(ps, err)
	}
	s, err := a.repo.ReadSessionSnapshot(a.ctx, []string{"g"})
	if err != nil || len(s.Sessions) != 0 || s.Games[0].TimePlayed != 3 || s.Games[0].LastLaunchedAt != 123 {
		t.Fatal(s, err)
	}
}
