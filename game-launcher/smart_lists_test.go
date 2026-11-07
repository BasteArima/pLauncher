package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"game-launcher/internal/models"
	"game-launcher/internal/parser"
)

func TestSmartLaunchMissingOnReadAndDetection(t *testing.T) {
	a := newTestApp(t)
	folder := t.TempDir()
	exe := filepath.Join(folder, "Game.exe")
	if err := os.WriteFile(exe, []byte("fixture, never executed"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, g := range []*models.Game{
		{ID: "empty", Title: "Empty", FolderPath: folder},
		{ID: "lost", Title: "Lost", FolderPath: folder, ExecPath: filepath.Join(folder, "lost.exe")},
		{ID: "good", Title: "Good", FolderPath: folder, ExecPath: exe},
		{ID: "folder", Title: "Folder", FolderPath: filepath.Join(folder, "absent")},
	} {
		if err := a.repo.SaveGame(a.ctx, g); err != nil {
			t.Fatal(err)
		}
	}
	for _, read := range []func() ([]*models.Game, error){a.GetGames, func() ([]*models.Game, error) { return a.SearchGames("") }, func() ([]*models.Game, error) { return a.SearchGames("Lost") }} {
		games, err := read()
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range games {
			want := g.ID == "empty" || g.ID == "lost"
			if g.LaunchMissing != want {
				t.Fatalf("%s missing=%v", g.ID, g.LaunchMissing)
			}
		}
	}
	n, err := a.DetectLaunchFiles([]string{"empty", "lost", "folder"})
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	empty, _ := a.gameByID("empty")
	lost, _ := a.gameByID("lost")
	if empty.ExecPath != exe || lost.ExecPath != filepath.Join(folder, "lost.exe") {
		t.Fatal(empty.ExecPath, lost.ExecPath)
	}
	if err := os.Remove(exe); err != nil {
		t.Fatal(err)
	}
	games, _ := a.GetGames()
	for _, g := range games {
		if g.ID == "good" && !g.LaunchMissing {
			t.Fatal("disk change not reflected")
		}
	}
	stored, _ := a.repo.GetAllGames(a.ctx)
	for _, g := range stored {
		if g.LaunchMissing {
			t.Fatal("computed flag persisted")
		}
	}
}

func TestSmartApplyCancellationAndSave(t *testing.T) {
	for _, mode := range []string{"save", "stop", "lock", "privacy", "new run"} {
		t.Run(mode, func(t *testing.T) {
			a := newTestApp(t)
			g := &models.Game{ID: "smart", Title: "Original", FolderPath: filepath.Join(t.TempDir(), "Moon_v1.2"), Notes: "keep", Status: "playing"}
			if err := a.repo.SaveGame(a.ctx, g); err != nil {
				t.Fatal(err)
			}
			started := make(chan struct{})
			release := make(chan struct{})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/image-cover" {
					w.Header().Set("Content-Type", "image/png")
					_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n0000"))
					return
				}
				close(started)
				<-release
				fmt.Fprintf(w, `<h1 class="p-title-value">Moon</h1><article class="message-body"><div class="bbWrapper"><b>Overview:</b><br>Test.<img class="bbImage" src="http://%s/image-cover"></div></article>`, r.Host)
			}))
			defer srv.Close()
			url := srv.URL + "/f95zone/threads/1/"
			result := parser.SearchResult{Hits: []parser.SearchHit{{Title: "Moon", URL: url, Best: true, Score: .9}}}
			token := a.BeginSmartSearch()
			done := make(chan struct {
				applied bool
				err     error
			}, 1)
			go func() {
				ok, err := a.ApplySmartGameInfo(token, g.ID, "Moon", result)
				done <- struct {
					applied bool
					err     error
				}{ok, err}
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				close(release)
				t.Fatal("parse not started")
			}
			switch mode {
			case "save":
				g.Notes = "edited during lookup"
				if err := a.repo.SaveGame(a.ctx, g); err != nil {
					t.Fatal(err)
				}
			case "stop":
				a.StopSmartSearch(token)
			case "lock":
				a.setLocked(true)
				a.setLocked(false)
			case "privacy":
				a.StopSmartSearch(0)
			case "new run":
				next := a.BeginSmartSearch()
				a.StopSmartSearch(token)
				if a.smartSearch.id != next || a.smartSearch.ctx == nil {
					t.Fatal("old stop cancelled new run")
				}
			}
			close(release)
			select {
			case r := <-done:
				if r.err != nil || r.applied != (mode == "save") {
					t.Fatal(r)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("apply hung")
			}
			saved, _ := a.gameByID(g.ID)
			if (len(saved.Sources) > 0) != (mode == "save") || saved.Notes != g.Notes || saved.Status != "playing" {
				t.Fatalf("%+v", saved)
			}
			if mode == "save" && saved.Version != "1.2" {
				t.Fatal(saved.Version)
			}
			media, err := filepath.Glob(filepath.Join(a.dataDir, "covers", "smart-*"))
			if err != nil {
				t.Fatal(err)
			}
			if mode == "save" {
				if len(media) != 1 || saved.CoverPath == "" {
					t.Fatal("applied media not retained", media, saved.CoverPath)
				}
				if _, err := os.Stat(filepath.Join(a.dataDir, filepath.FromSlash(saved.CoverPath))); err != nil {
					t.Fatal(err)
				}
			} else if len(media) != 0 {
				t.Fatal("cancelled media not cleaned", media)
			}
		})
	}
}

func TestSmartApplyRejectsUnconfidentAndUnconfigured(t *testing.T) {
	a := newTestApp(t)
	if ok, err := a.ApplySmartGameInfo(a.BeginSmartSearch(), "missing", "Moon", parser.SearchResult{}); ok || err != nil {
		t.Fatal(ok, err)
	}
	if (&App{}).BeginSmartSearch() != 0 {
		t.Fatal("unconfigured run")
	}
	a.setLocked(true)
	if a.BeginSmartSearch() != 0 {
		t.Fatal("locked run")
	}
	a.setLocked(false)
	a.priv.panicHidden = true
	if a.BeginSmartSearch() != 0 {
		t.Fatal("panic-hidden run")
	}
}
