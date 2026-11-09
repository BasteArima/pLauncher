package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"game-launcher/internal/apperr"
	"game-launcher/internal/models"
	"game-launcher/internal/parser"
)

type collectionServer struct {
	mu     sync.Mutex
	data   []byte
	pages  atomic.Int32
	images atomic.Int32
	server *httptest.Server
}

func newCollectionServer(t *testing.T) *collectionServer {
	t.Helper()
	b, err := os.ReadFile("internal/parser/testdata/pornolab-collection-3006469.html")
	if err != nil {
		t.Fatal(err)
	}
	s := &collectionServer{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/media/") {
			s.images.Add(1)
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n0000"))
			return
		}
		s.pages.Add(1)
		s.mu.Lock()
		b := append([]byte{}, s.data...)
		s.mu.Unlock()
		_, _ = w.Write(b)
	}))
	t.Cleanup(s.server.Close)
	s.data = []byte(strings.ReplaceAll(string(b), "https://media.example", s.server.URL+"/media"))
	return s
}
func (s *collectionServer) url() string {
	return s.server.URL + "/pornolab.net/forum/viewtopic.php?t=1"
}
func (s *collectionServer) replace(old, new string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = []byte(strings.ReplaceAll(string(s.data), old, new))
}
func saveCollectionTarget(t *testing.T, a *App) *models.Game {
	t.Helper()
	g := &models.Game{ID: "collection", Title: "My title", FolderPath: filepath.Join(t.TempDir(), "QuietTown_v2.0"), Notes: "Keep notes", Rating: 4, Tags: []string{"Manual"}, CoverPath: "covers/old.jpg"}
	if err := a.repo.SaveGame(a.ctx, g); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestCollectionAppFlowAndUpdates(t *testing.T) {
	a := newTestApp(t)
	g := saveCollectionTarget(t, a)
	s := newCollectionServer(t)
	choices, err := a.ResolvePornolabCollection(g.ID, s.url(), true)
	if err != nil || len(choices.Candidates) != 2 || choices.Candidates[0].Index != 2 || !choices.Candidates[0].Best {
		t.Fatal(choices, err)
	}
	preview, err := a.PreviewPornolabGame(g.ID, s.url(), choices.Snapshot, 2)
	if err != nil || preview.Title != "Quiet Town" || !strings.HasPrefix(preview.CoverPath, s.server.URL) {
		t.Fatal(preview, err)
	}
	before, _ := a.gameByID(g.ID)
	if before.Title != "My title" || s.images.Load() != 0 {
		t.Fatal("preview writes")
	}
	if err = a.ApplyPornolabGame(g.ID, s.url(), choices.Snapshot, 2, a.BeginPornolabApply()); err != nil {
		t.Fatal(err)
	}
	saved, _ := a.gameByID(g.ID)
	if s.pages.Load() != 1 || saved.Title != "Quiet Town" || saved.Notes != "Keep notes" || saved.Rating != 4 || len(saved.Sources) != 1 || saved.Sources[0].SelectedTitle != "Quiet Town" || saved.Sources[0].URL != s.url()+"#game=2" || saved.Sources[0].LastVersion != "2.0" {
		t.Fatal(saved, s.pages.Load())
	}
	if strings.HasPrefix(saved.CoverPath, "http") || !strings.HasPrefix(saved.CoverPath, "covers/") {
		t.Fatal("remote media persisted", saved.CoverPath)
	}
	images := s.images.Load()
	cover := saved.CoverPath
	s.replace("2.0", "2.1")
	checked, err := a.CheckSourceUpdate(g.ID, "Pornolab")
	if err != nil || !checked.UpdateAvailable || checked.UpdateVersion != "2.1" || checked.Version != "2.0" || checked.CoverPath != cover || s.images.Load() != images || s.pages.Load() != 2 {
		t.Fatal(checked, err)
	}
	// Accepting a discovered update must not reuse the older pre-check cached page.
	if err = a.UpdateGameMetadata(g.ID, checked.Sources[0].URL); err != nil {
		t.Fatal(err)
	}
	checked, _ = a.gameByID(g.ID)
	if checked.Version != "2.1" || checked.UpdateAvailable || checked.Sources[0].LastVersion != "2.1" {
		t.Fatal("stale update acceptance", checked)
	}
	// The editable library title never becomes the selection identity.
	checked.Title = "Personal title"
	checked.Sources[0].SelectedTitle = ""
	if err = a.UpdateGame(*checked); err != nil {
		t.Fatal(err)
	}
	checked, _ = a.gameByID(g.ID)
	if checked.Sources[0].SelectedTitle != "Quiet Town" {
		t.Fatal(checked.Sources)
	}
	s.replace("Quiet Town", "Removed game")
	_, err = a.CheckGameUpdates(g.ID)
	if apperr.Code(err) != "parse.collection_game_missing" {
		t.Fatal(err)
	}
	after, _ := a.gameByID(g.ID)
	if !reflect.DeepEqual(checked, after) {
		t.Fatal("failed check changed library")
	}
	// Changing the source URL intentionally clears the identity, even when the editor echoed it.
	after.Sources[0].URL = s.url() + "#game=1"
	if err = a.UpdateGame(*after); err != nil {
		t.Fatal(err)
	}
	after, _ = a.gameByID(g.ID)
	if after.Sources[0].SelectedTitle != "" {
		t.Fatal(after.Sources)
	}
}

func TestCollectionLegacyAndBatchManual(t *testing.T) {
	a := newTestApp(t)
	g := saveCollectionTarget(t, a)
	s := newCollectionServer(t)
	g.Sources = []models.GameSource{{Source: "Pornolab", URL: s.url(), LastVersion: "Old"}}
	g.PrimarySource = "Pornolab"
	if err := a.repo.SaveGame(a.ctx, g); err != nil {
		t.Fatal(err)
	}
	checked, err := a.CheckGameUpdates(g.ID)
	if err != nil || checked.UpdateVersion != "Final" {
		t.Fatal(checked, err)
	}
	before, _ := a.gameByID(g.ID)
	if err = a.UpdateGameMetadata(g.ID, s.url()); apperr.Code(err) != "parse.collection_choice_required" {
		t.Fatal(err)
	}
	token := a.BeginSmartSearch()
	result, err := a.ReparseGameInfo(token, g.ID)
	if err != nil || !result.Manual || result.Applied || s.images.Load() != 0 {
		t.Fatal(result, err, s.images.Load())
	}
	after, _ := a.gameByID(g.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("unselected batch changed library")
	}
	manual := a.GetManualGameChoices()
	if len(manual) != 1 || manual[0].GameID != g.ID || len(manual[0].Collection.Candidates) != 2 {
		t.Fatal(manual)
	}
	a.StopSmartSearch(token)
	result, err = a.ReparseGameInfo(token, g.ID)
	if err != nil || result.Applied || result.Manual {
		t.Fatal(result, err)
	}
	a.setLocked(true)
	a.setLocked(false)
	if len(a.GetManualGameChoices()) != 0 {
		t.Fatal("manual handoff survived lock")
	}
	// Existing smart lookup treats the collection as manual even with a confident result.
	g.Sources = nil
	g.PrimarySource = ""
	if err = a.repo.SaveGame(a.ctx, g); err != nil {
		t.Fatal(err)
	}
	token = a.BeginSmartSearch()
	applied, err := a.ApplySmartGameInfo(token, g.ID, "Quiet Town", parser.SearchResult{Hits: []parser.SearchHit{{URL: s.url(), Title: "Quiet Town", Source: "Pornolab", Best: true, Score: 1}}})
	if err != nil || applied || len(a.GetManualGameChoices()) != 1 {
		t.Fatal(applied, err)
	}
}

func TestCollectionSavedReparseAndReorder(t *testing.T) {
	a := newTestApp(t)
	g := saveCollectionTarget(t, a)
	s := newCollectionServer(t)
	// Index now points elsewhere, but original source title survives a restart/read.
	g.Sources = []models.GameSource{{Source: "Pornolab", URL: s.url() + "#game=1", SelectedTitle: "Quiet Town", LastVersion: "1.0"}}
	g.PrimarySource = "Pornolab"
	if err := a.repo.SaveGame(a.ctx, g); err != nil {
		t.Fatal(err)
	}
	r, err := a.ReparseGameInfo(a.BeginSmartSearch(), g.ID)
	if err != nil || !r.Applied || r.Manual {
		t.Fatal(r, err)
	}
	saved, _ := a.gameByID(g.ID)
	if saved.Title != "Quiet Town" || saved.Sources[0].URL != s.url()+"#game=2" || saved.Sources[0].SelectedTitle != "Quiet Town" || saved.Sources[0].LastVersion != "2.0" {
		t.Fatal(saved)
	}
}

func TestCollectionPrivacyAndUnconfigured(t *testing.T) {
	a := newTestApp(t)
	g := saveCollectionTarget(t, a)
	s := newCollectionServer(t)
	choices, err := a.ResolvePornolabCollection(g.ID, s.url(), true)
	if err != nil {
		t.Fatal(err)
	}
	a.setLocked(true)
	a.setLocked(false)
	if _, err = a.PreviewPornolabGame(g.ID, s.url(), choices.Snapshot, 1); err == nil {
		t.Fatal("stale privacy snapshot accepted")
	}
	for _, app := range []*App{{ctx: context.Background()}, a} {
		app.priv.locked = true
		before := s.pages.Load()
		r, e := app.ResolvePornolabCollection(g.ID, s.url(), true)
		if e != nil || len(r.Candidates) != 0 {
			t.Fatal(r, e)
		}
		if p, e := app.PreviewPornolabGame(g.ID, s.url(), choices.Snapshot, 1); e != nil || p != nil {
			t.Fatal(p, e)
		}
		if e := app.ApplyPornolabGame(g.ID, s.url(), choices.Snapshot, 1, app.BeginPornolabApply()); e != nil {
			t.Fatal(e)
		}
		if s.pages.Load() != before {
			t.Fatal("locked request")
		}
	}
}

func TestCollectionLockDuringRequest(t *testing.T) {
	a := newTestApp(t)
	g := saveCollectionTarget(t, a)
	started := make(chan struct{})
	release := make(chan struct{})
	bytes, err := os.ReadFile("internal/parser/testdata/pornolab-collection-3006469.html")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-release; w.Write(bytes) }))
	defer server.Close()
	done := make(chan error, 1)
	go func() { _, e := a.ResolvePornolabCollection(g.ID, server.URL+"/pornolab.net", true); done <- e }()
	<-started
	a.setLocked(true)
	a.setLocked(false)
	close(release)
	if err = <-done; err == nil {
		t.Fatal("late result after fast unlock")
	}
	after, _ := a.gameByID(g.ID)
	if after.Title != "My title" {
		t.Fatal(after)
	}
}

func TestCollectionIdentityRelink(t *testing.T) {
	a := newTestApp(t)
	g := saveCollectionTarget(t, a)
	g.Sources = []models.GameSource{{Source: "Pornolab", URL: "https://pornolab.net/?t=1#game=2", SelectedTitle: "Original title", LastVersion: "2.0"}}
	if err := a.repo.SaveGame(a.ctx, g); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	duplicate := &models.Game{ID: "duplicate", Title: "Scan duplicate", FolderPath: target}
	if err := a.repo.SaveGame(a.ctx, duplicate); err != nil {
		t.Fatal(err)
	}
	moved, err := a.RelinkGame(g.ID, target)
	if err != nil || !reflect.DeepEqual(moved.Sources, g.Sources) {
		t.Fatal(moved, err)
	}
	read, err := a.gameByID(g.ID)
	if err != nil || !reflect.DeepEqual(read.Sources, g.Sources) {
		t.Fatal(read, err)
	}
}

func TestCollectionBatchStopDuringRequest(t *testing.T) {
	a := newTestApp(t)
	g := saveCollectionTarget(t, a)
	started, release := make(chan struct{}), make(chan struct{})
	b, err := os.ReadFile("internal/parser/testdata/pornolab-collection-3006469.html")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-release; _, _ = w.Write(b) }))
	defer srv.Close()
	g.Sources = []models.GameSource{{Source: "Pornolab", URL: srv.URL + "/pornolab.net"}}
	if err = a.repo.SaveGame(a.ctx, g); err != nil {
		t.Fatal(err)
	}
	token := a.BeginSmartSearch()
	done := make(chan MetadataApplyResult, 1)
	errs := make(chan error, 1)
	go func() { r, e := a.ReparseGameInfo(token, g.ID); done <- r; errs <- e }()
	<-started
	a.StopSmartSearch(token)
	close(release)
	r, e := <-done, <-errs
	if e != nil || r.Applied || r.Manual {
		t.Fatal(r, e)
	}
	after, _ := a.gameByID(g.ID)
	if after.Title != g.Title || len(a.GetManualGameChoices()) != 0 {
		t.Fatal("stopped batch published results")
	}
	files, _ := filepath.Glob(filepath.Join(a.dataDir, "covers", "reparse-*"))
	if len(files) != 0 {
		t.Fatal("stopped batch left media", files)
	}
}
