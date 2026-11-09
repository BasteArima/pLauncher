package main

import (
	"context"
	"game-launcher/internal/models"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCollectionApplyCancellationKeepsLibraryAndCache(t *testing.T) {
	a := newTestApp(t)
	g := saveCollectionTarget(t, a)
	data, err := os.ReadFile("internal/parser/testdata/pornolab-collection-3006469.html")
	if err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/media/") {
			once.Do(func() { close(started) })
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
			_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n0000"))
			return
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	defer close(release)
	body = []byte(strings.ReplaceAll(string(data), "https://media.example", srv.URL+"/media"))
	url := srv.URL + "/pornolab.net/?t=1"
	choices, err := a.ResolvePornolabCollection(g.ID, url, true)
	if err != nil {
		t.Fatal(err)
	}
	token := a.BeginPornolabApply()
	done := make(chan error, 1)
	go func() { done <- a.ApplyPornolabGame(g.ID, url, choices.Snapshot, 1, token) }()
	<-started
	a.StopPornolabApply(token)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled media request kept running")
	}
	current, err := a.gameByID(g.ID)
	if err != nil || current.Title != g.Title || current.CoverPath != g.CoverPath || len(current.Sources) != 0 {
		t.Fatal("cancelled save committed", current, err)
	}
	files, err := os.ReadDir(a.dataDir + "/covers")
	if err != nil || len(files) != 0 {
		t.Fatal("cancelled media retained", files, err)
	}
	// Stopping this save must not invalidate page snapshots or a newer save token.
	newToken := a.BeginPornolabApply()
	a.StopPornolabApply(token)
	a.collectionApply.mu.Lock()
	valid := a.collectionApply.ctx != nil && a.collectionApply.ctx.Err() == nil
	a.collectionApply.mu.Unlock()
	if !valid {
		t.Fatal("old cleanup cancelled newer operation")
	}
	a.StopPornolabApply(newToken)
	if _, err = a.PreviewPornolabGame(g.ID, url, choices.Snapshot, 2); err != nil {
		t.Fatal("cancel invalidated page", err)
	}
	if err = a.ApplyPornolabGame(g.ID, url, choices.Snapshot, 1, token); err != context.Canceled {
		t.Fatal("cancelled token accepted", err)
	}
}

func TestCollectionUpdatePreservesConcurrentLibraryChanges(t *testing.T) {
	for _, mode := range []string{"game", "source", "all"} {
		for _, edit := range []string{"notes", "remove", "source"} {
			t.Run(mode+"/"+edit, func(t *testing.T) {
				a := newTestApp(t)
				g := saveCollectionTarget(t, a)
				data, err := os.ReadFile("internal/parser/testdata/pornolab-collection-3006469.html")
				if err != nil {
					t.Fatal(err)
				}
				started, release := make(chan struct{}), make(chan struct{})
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-release; _, _ = w.Write(data) }))
				defer srv.Close()
				url := srv.URL + "/pornolab.net/?t=1#game=2"
				g.Sources = []models.GameSource{{Source: "Pornolab", URL: url, SelectedTitle: "Quiet Town", LastVersion: "1.0"}}
				g.PrimarySource = "Pornolab"
				if err = a.repo.SaveGame(a.ctx, g); err != nil {
					t.Fatal(err)
				}
				done := make(chan struct{})
				go func() {
					defer close(done)
					switch mode {
					case "game":
						_, _ = a.CheckGameUpdates(g.ID)
					case "source":
						_, _ = a.CheckSourceUpdate(g.ID, "Pornolab")
					case "all":
						_, _ = a.CheckAllUpdates()
					}
				}()
				<-started
				if edit == "remove" {
					err = a.repo.DeleteGame(a.ctx, g.ID)
				} else {
					g.Notes = "Edited while checking"
					if edit == "source" {
						g.Sources[0].URL = strings.Replace(url, "t=1", "t=2", 1)
					}
					err = a.repo.SaveGame(a.ctx, g)
				}
				close(release)
				<-done
				if err != nil {
					t.Fatal(err)
				}
				current, e := a.gameByID(g.ID)
				if edit == "remove" {
					if e == nil {
						t.Fatal("update resurrected removed game")
					}
					return
				}
				if e != nil {
					t.Fatal(e)
				}
				if current.Notes != g.Notes || current.Sources[0].URL != g.Sources[0].URL {
					t.Fatal("late update overwrote changes", current)
				}
				if edit == "source" && current.UpdateAvailable {
					t.Fatal("old source update applied to replacement")
				}
				if edit == "notes" && !current.UpdateAvailable {
					t.Fatal("valid update lost")
				}
			})
		}
	}
}

func TestCollectionMalformedSelectionHasManualHandoff(t *testing.T) {
	a := newTestApp(t)
	g := saveCollectionTarget(t, a)
	s := newCollectionServer(t)
	g.Sources = []models.GameSource{{Source: "Pornolab", URL: s.url() + "#game=bad"}}
	if err := a.repo.SaveGame(a.ctx, g); err != nil {
		t.Fatal(err)
	}
	result, err := a.ReparseGameInfo(a.BeginSmartSearch(), g.ID)
	if err != nil || !result.Manual {
		t.Fatal(result, err)
	}
	choices := a.GetManualGameChoices()
	if len(choices) != 1 || len(choices[0].Collection.Candidates) != 2 || strings.Contains(choices[0].URL, "#") {
		t.Fatal("missing usable manual handoff", choices)
	}
}
