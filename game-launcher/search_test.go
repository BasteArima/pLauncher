package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"game-launcher/internal/models"
)

func TestFindInfoUnconfiguredAndLocked(t *testing.T) {
	locked := newTestApp(t)
	locked.priv.mu.Lock()
	locked.priv.locked = true
	locked.priv.mu.Unlock()
	for _, a := range []*App{{ctx: context.Background()}, locked} {
		if s, err := a.SuggestSearchName("missing"); err != nil || s.Title != "" || s.Version != "" {
			t.Fatal(s, err)
		}
		if r, err := a.SearchGameInfo("secret", nil, "missing"); err != nil || len(r.Hits)+len(r.Failed) != 0 {
			t.Fatal(r, err)
		}
		if g, err := a.PreviewGameInfo(":invalid:"); err != nil || g != nil {
			t.Fatal(g, err)
		}
		if err := a.ApplyGameInfo("missing", ":invalid:"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFindInfoPreviewApplyAndUpdateIsolation(t *testing.T) {
	a := newTestApp(t)
	g := &models.Game{ID: "find", Title: "Original", FolderPath: filepath.Join(t.TempDir(), "MoonlitHarbor_v0.3"), ExecPath: "Game.exe", CoverPath: "covers/old.png", Images: []string{"covers/old-shot.png"}, Tags: []string{"Manual"}, Engine: "Unity", Status: "playing", Rating: 4, Notes: "Private", Favorite: true, TimePlayed: 42, UpdateAvailable: true, UpdateVersion: "old", UpdateSource: "F95zone", UpdateFoundAt: 123}
	if err := a.repo.SaveGame(a.ctx, g); err != nil {
		t.Fatal(err)
	}
	var base string
	var imageRequests atomic.Int32
	version := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/image") {
			imageRequests.Add(1)
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("\x89PNG\r\n\x1a\n0000"))
			return
		}
		fmt.Fprintf(w, `<h1 class="p-title-value">Moonlit Harbor%s</h1><div class="js-tagList"><a class="tagItem">Novel</a></div><article class="message-body"><div class="bbWrapper"><b>Overview:</b><br>A quiet harbor.<br><b>Other:</b><img class="bbImage" src="%s/image-cover"><img class="bbImage" src="%s/image-shot"></div></article>`, version, base, base)
	}))
	defer srv.Close()
	base = srv.URL
	link := base + "/f95zone/threads/1/"
	suggestion, err := a.SuggestSearchName(g.ID)
	if err != nil || suggestion.Title != "Moonlit Harbor" || suggestion.Version != "0.3" {
		t.Fatal(suggestion, err)
	}
	before, err := a.gameByID(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := a.PreviewGameInfo(link)
	if err != nil || preview.CoverPath != base+"/image-cover" || len(preview.Images) != 1 || imageRequests.Load() != 0 {
		t.Fatal(preview, err, imageRequests.Load())
	}
	after, _ := a.gameByID(g.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("preview changed library")
	}
	if err := a.ApplyGameInfo(g.ID, link); err != nil {
		t.Fatal(err)
	}
	saved, _ := a.gameByID(g.ID)
	if saved.Title != "Moonlit Harbor" || saved.Version != "0.3" || saved.Notes != g.Notes || saved.Status != g.Status || saved.Rating != g.Rating || !saved.Favorite || saved.TimePlayed != 42 || saved.ExecPath != g.ExecPath || saved.Engine != "Unity" || saved.UpdateAvailable || saved.UpdateVersion != "" || saved.UpdateSource != "" || saved.UpdateFoundAt != 0 || saved.NotesUpdatedAt != before.NotesUpdatedAt || !reflect.DeepEqual(saved.Tags, []string{"Manual", "Novel"}) {
		t.Fatalf("%+v", saved)
	}
	if len(saved.Sources) != 1 || saved.Sources[0].URL != link || saved.Sources[0].LastVersion != "" {
		t.Fatal(saved.Sources)
	}
	if !strings.HasPrefix(saved.CoverPath, "covers/") || imageRequests.Load() != 2 {
		t.Fatal(saved.CoverPath, imageRequests.Load())
	}
	if _, err := os.Stat(filepath.Join(a.dataDir, filepath.FromSlash(saved.CoverPath))); err != nil {
		t.Fatal(err)
	}
	cover, images := saved.CoverPath, append([]string{}, saved.Images...)
	version = " [v0.5]"
	if _, err := a.CheckSourceUpdate(g.ID, saved.Sources[0].Source); err != nil {
		t.Fatal(err)
	}
	checked, _ := a.gameByID(g.ID)
	if checked.CoverPath != cover || !reflect.DeepEqual(checked.Images, images) || imageRequests.Load() != 2 {
		t.Fatal("update check replaced/downloaded media", checked)
	}
	// Новая версия сайта побеждает фолбэк папки; версия самой игры не стирается сайтом без версии.
	if err := a.ApplyGameInfo(g.ID, link); err != nil {
		t.Fatal(err)
	}
	saved, _ = a.gameByID(g.ID)
	if saved.Version != "v0.5" {
		t.Fatal(saved.Version)
	}
	version = ""
	if err := a.ApplyGameInfo(g.ID, link); err != nil {
		t.Fatal(err)
	}
	saved, _ = a.gameByID(g.ID)
	if saved.Version != "v0.5" {
		t.Fatal(saved.Version)
	}
}

func TestFindInfoErrors(t *testing.T) {
	a := newTestApp(t)
	if _, err := a.SuggestSearchName("missing"); err == nil || !strings.Contains(err.Error(), "[app.game_not_found") {
		t.Fatal(err)
	}
	if _, err := a.SearchGameInfo("Moon", []string{}, "missing"); err == nil || !strings.Contains(err.Error(), "[app.game_not_found") {
		t.Fatal(err)
	}
	if err := a.ApplyGameInfo("missing", "bad"); err == nil || !strings.Contains(err.Error(), "[app.game_not_found") {
		t.Fatal(err)
	}
	if _, err := a.PreviewGameInfo("bad"); err == nil || !strings.Contains(err.Error(), "[parse.unsupported") {
		t.Fatal(err)
	}
	if got := folderSearchName(`C:\Games\MoonlitHarbor_v0.3\`); got.Title != "Moonlit Harbor" || got.Version != "0.3" {
		t.Fatal(got)
	}
}

func TestFindInfoLockDuringRequest(t *testing.T) {
	a := newTestApp(t)
	g := &models.Game{ID: "locked-during-request", Title: "Original", FolderPath: t.TempDir()}
	if err := a.repo.SaveGame(a.ctx, g); err != nil {
		t.Fatal(err)
	}
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/image") {
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("\x89PNG\r\n\x1a\n0000"))
			return
		}
		a.priv.mu.Lock()
		a.priv.locked = true
		a.priv.mu.Unlock()
		fmt.Fprintf(w, `<h1 class="p-title-value">Secret title</h1><article class="message-body"><div class="bbWrapper"><img class="bbImage" src="%s/image-cover"><img class="bbImage" src="%s/image-shot"></div></article>`, base, base)
	}))
	defer srv.Close()
	base = srv.URL
	link := base + "/f95zone/threads/1/"
	if preview, err := a.PreviewGameInfo(link); err != nil || preview != nil {
		t.Fatal(preview, err)
	}
	a.priv.mu.Lock()
	a.priv.locked = false
	a.priv.mu.Unlock()
	if err := a.ApplyGameInfo(g.ID, link); err != nil {
		t.Fatal(err)
	}
	saved, err := a.gameByID(g.ID)
	if err != nil || saved.Title != "Original" || saved.CoverPath != "" {
		t.Fatal(saved, err)
	}
}
