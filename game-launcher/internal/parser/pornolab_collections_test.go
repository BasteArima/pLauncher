package parser

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
	"time"

	"game-launcher/internal/apperr"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/charmap"
)

func collectionFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func collectionDoc(t *testing.T, b []byte) *goquery.Document {
	t.Helper()
	d, err := goquery.NewDocumentFromReader(charmap.Windows1251.NewDecoder().Reader(strings.NewReader(string(b))))
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func encodedCollection(t *testing.T, s string) []byte {
	t.Helper()
	b, err := charmap.Windows1251.NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPornolabSectionFixtures(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
	}{{"pornolab-collection-3006469.html", 2}, {"pornolab-collection-three.html", 3}, {"pornolab-collection-spoilers.html", 2}, {"pornolab-single.html", 1}, {"pornolab-collection-ambiguous.html", 3}, {"pornolab-collection-no-cover.html", 2}} {
		t.Run(tc.name, func(t *testing.T) {
			doc := collectionDoc(t, collectionFixture(t, tc.name))
			ss := pornolabSections(doc)
			if len(ss) != tc.count {
				t.Fatalf("sections=%d want=%d", len(ss), tc.count)
			}
			if !reflect.DeepEqual(ss, pornolabSections(doc)) {
				t.Fatal("extraction mutates document")
			}
			g := ss[0].game
			if g.Version != "1.0" || g.Author != "Studio 1" || ss[0].year != "2020" || g.Engine != "Unity" || g.Description != "Neutral story for game 1. Explore a quiet place." {
				t.Fatalf("%+v", g)
			}
			if !reflect.DeepEqual(g.Languages, []string{"Русский", "English", "日本語"}) || !reflect.DeepEqual(g.Tags, []string{"Adventure", "Unity"}) {
				t.Fatal(g.Languages, g.Tags)
			}
			if len(g.Images) != 15 {
				t.Fatal("cap", len(g.Images))
			}
			for _, image := range append(g.Images, g.CoverPath) {
				if !strings.Contains(image, "game1-") {
					t.Fatal("foreign image", image)
				}
			}
			if tc.name == "pornolab-collection-no-cover.html" {
				if !strings.HasSuffix(g.CoverPath, "screen1.jpg") || strings.HasSuffix(g.Images[0], "screen1.jpg") {
					t.Fatal(g.CoverPath, g.Images)
				}
			}
			if tc.name == "pornolab-collection-ambiguous.html" && g.Title != "" {
				t.Fatal("invented title", g.Title)
			}
		})
	}
}

func TestPornolabSelectionAndLegacy(t *testing.T) {
	p := &PornlabParser{client: http.DefaultClient}
	doc := collectionDoc(t, collectionFixture(t, "pornolab-collection-3006469.html"))
	ctx := context.Background()
	if _, err := p.parseDocument(ctx, doc, "https://pornolab.net/forum/viewtopic.php?t=1", "", PornolabOptions{}); apperr.Code(err) != "parse.collection_choice_required" {
		t.Fatal(err)
	}
	legacy, err := p.parseDocument(ctx, doc, "https://pornolab.net/forum/viewtopic.php?t=1", "", PornolabOptions{LegacyUpdate: true})
	if err != nil || legacy.Version != "Final" {
		t.Fatal(legacy, err)
	}
	g, err := p.parseDocument(ctx, doc, "https://pornolab.net/forum/viewtopic.php?t=1#game=2", "", PornolabOptions{})
	if err != nil || g.Title != "Quiet Town" || g.Version != "2.0" || g.Sources[0].SelectedTitle != "Quiet Town" {
		t.Fatal(g, err)
	}
	// Stored title repairs both a changed index and an index now beyond the end.
	for _, index := range []int{1, 9} {
		g, err = p.parseDocument(ctx, doc, fmt.Sprintf("https://pornolab.net/?t=1#game=%d", index), "", PornolabOptions{SavedSelection: true, SelectedTitle: "Quiet Town"})
		if err != nil || g.Title != "Quiet Town" || !strings.HasSuffix(g.Sources[0].URL, "#game=2") {
			t.Fatal(g, err)
		}
	}
	for _, title := range []string{"", "Gone", "Quiet Town: Afterlight"} {
		_, err = p.parseDocument(ctx, doc, "https://pornolab.net/?t=1#game=2", "", PornolabOptions{SavedSelection: true, SelectedTitle: title})
		if apperr.Code(err) != "parse.collection_game_missing" {
			t.Fatal(title, err)
		}
	}
	ambiguous := collectionDoc(t, collectionFixture(t, "pornolab-collection-ambiguous.html"))
	if _, err = p.parseDocument(ctx, ambiguous, "https://pornolab.net/?t=1#game=2", "", PornolabOptions{SavedSelection: true, SelectedTitle: "Moonlit Harbor"}); apperr.Code(err) != "parse.collection_game_missing" {
		t.Fatal(err)
	}
	for _, fragment := range []string{"game=0", "game=-1", "game=no", "game=01", "game=1&game=2", "game", "game=999"} {
		_, err = p.parseDocument(ctx, doc, "https://pornolab.net/?t=1#"+fragment, "", PornolabOptions{})
		if apperr.Code(err) != "parse.collection_game_missing" {
			t.Fatal(fragment, err)
		}
	}
	single := collectionDoc(t, collectionFixture(t, "pornolab-single.html"))
	ordinary, e := p.parseDocument(ctx, single, "https://pornolab.net/?t=1", "", PornolabOptions{})
	original, e2 := p.parseLegacy(ctx, single, "https://pornolab.net/?t=1", "")
	if e != nil || e2 != nil || !reflect.DeepEqual(ordinary, original) {
		t.Fatal("single regression", ordinary, original, e, e2)
	}
	if ordinary.Title != "Moonlit Harbor" || ordinary.Version != "1.0" || ordinary.Author != "Studio 1" || ordinary.CoverPath != "https://media.example/game1-cover.jpg" {
		t.Fatalf("legacy baseline: %+v", ordinary)
	}
}

func TestPornolabBoundariesAndFallbacks(t *testing.T) {
	body := `<h1 class="maintitle">[Сборник] Shared [v5] (Shared Studio) [2020, RPG, Unity] [eng]</h1><div class="post-user-message"><div align="center"><span style="font-size:24px">First</span><span class="post-b">Описание</span>: First prose.<span class="post-b">Примечание</span>: not description<span class="post-hr"></span><span style="font-size: 24px">Second</span><span class="post-b">Версия</span>: <br><span class="post-b">Описание</span>: <div class="sp-wrap"><div class="sp-head">Screenshots</div><div class="sp-body"><span class="post-hr"></span><var class="postImg" title="https://images.example/a.jpg"></var><var class="postImg" title="https://images.example/a.jpg"></var><var class="postImg" title="https://images.example/b.jpg"></var></div></div></div><blockquote><span class="post-b">Описание</span>: quote</blockquote></div>`
	ss := pornolabSections(collectionDoc(t, encodedCollection(t, body)))
	if len(ss) != 2 {
		t.Fatal(len(ss))
	}
	first, second := ss[0].game, ss[1].game
	if first.Version != "v5" || first.Author != "Shared Studio" || first.Description != "First prose." || first.Engine != "Unity" || !reflect.DeepEqual(first.Languages, []string{"English"}) {
		t.Fatalf("%+v", first)
	}
	if second.Version != "" || second.Description != "" || second.CoverPath != "https://images.example/a.jpg" || !reflect.DeepEqual(second.Images, []string{"https://images.example/b.jpg"}) {
		t.Fatalf("%+v", second)
	}
}

func TestPornolabCacheAndSnapshots(t *testing.T) {
	data := collectionFixture(t, "pornolab-collection-3006469.html")
	var pages, images atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/media/") {
			images.Add(1)
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n0000"))
			return
		}
		pages.Add(1)
		_, _ = w.Write(data)
	}))
	defer srv.Close()
	data = []byte(strings.ReplaceAll(string(data), "https://media.example", srv.URL+"/media"))
	cache := &PornolabCache{}
	now := time.Now()
	cache.now = func() time.Time { return now }
	p := &PornlabParser{client: srv.Client()}
	raw := srv.URL + "/?t=1"
	ctx := WithPornolab(context.Background(), PornolabOptions{Cache: cache})
	list, err := p.InspectCollection(ctx, raw, "Quiet Town", SearchHint{Version: "2.0"})
	if err != nil || len(list.Candidates) != 2 || !list.Candidates[0].Best || list.Candidates[0].Index != 2 {
		t.Fatal(list, err)
	}
	selected := WithPornolab(context.Background(), PornolabOptions{Cache: cache, Snapshot: list.Snapshot})
	for _, dir := range []string{"", "", t.TempDir()} {
		g, e := p.Parse(selected, raw+"#game=2", dir)
		if e != nil || g.Title != "Quiet Town" {
			t.Fatal(g, e)
		}
	}
	if pages.Load() != 1 || images.Load() == 0 {
		t.Fatal("page/media requests", pages.Load(), images.Load())
	}
	now = now.Add(9 * time.Minute)
	_, _ = p.Parse(selected, raw+"#game=1", "")
	now = now.Add(9 * time.Minute)
	_, _ = p.Parse(selected, raw+"#game=1", "")
	if pages.Load() != 1 {
		t.Fatal("idle lifetime not refreshed")
	}
	now = now.Add(11 * time.Minute)
	_, err = p.Parse(selected, raw+"#game=1", "")
	if err != nil || pages.Load() != 2 {
		t.Fatal("expiry", err, pages.Load())
	}
	// Changed versions do not reopen the list; changed membership does.
	data = []byte(strings.ReplaceAll(string(data), "1.0", "1.1"))
	now = now.Add(11 * time.Minute)
	g, err := p.Parse(selected, raw+"#game=1", "")
	if err != nil || g.Version != "1.1" {
		t.Fatal(g, err)
	}
	data = []byte(strings.ReplaceAll(string(data), "Quiet Town", "Different Game"))
	now = now.Add(11 * time.Minute)
	if _, err = p.Parse(selected, raw+"#game=1", ""); apperr.Code(err) != "parse.collection_changed" {
		t.Fatal(err)
	}
	for i := 0; i < 7; i++ {
		_, _ = p.InspectCollection(ctx, fmt.Sprintf("%s/?t=%d", srv.URL, i), "", SearchHint{})
		now = now.Add(time.Second)
	}
	if len(cache.pages) != 5 {
		t.Fatal("capacity", len(cache.pages))
	}
	before := pages.Load()
	_, _ = p.InspectCollection(ctx, raw, "", SearchHint{})
	if pages.Load() != before+1 {
		t.Fatal("LRU not evicted")
	}
	before = pages.Load()
	_, _ = p.InspectCollection(WithPornolab(context.Background(), PornolabOptions{Cache: cache, Fresh: true}), raw, "", SearchHint{})
	if pages.Load() != before+1 {
		t.Fatal("explicit refresh")
	}
	cache.Clear()
	if len(cache.pages) != 0 {
		t.Fatal("clear")
	}
	if _, err = p.Parse(selected, raw+"#game=1", ""); err == nil {
		t.Fatal("old privacy snapshot accepted")
	}
}

func TestPornolabSnapshotReorderAndManyCandidates(t *testing.T) {
	ss := pornolabSections(collectionDoc(t, collectionFixture(t, "pornolab-collection-3006469.html")))
	snapshot := sectionSnapshot(ss)
	ss[0], ss[1] = ss[1], ss[0]
	if i, e := resolveSnapshot(snapshot, ss, 1); e != nil || i != 2 {
		t.Fatal(i, e)
	}
	ambiguous := pornolabSections(collectionDoc(t, collectionFixture(t, "pornolab-collection-ambiguous.html")))
	ambiguousSnapshot := sectionSnapshot(ambiguous)
	ambiguous[1], ambiguous[2] = ambiguous[2], ambiguous[1]
	if _, e := resolveSnapshot(ambiguousSnapshot, ambiguous, 2); apperr.Code(e) != "parse.collection_game_missing" {
		t.Fatal("ambiguous reorder guessed", e)
	}
	ambiguous[0].game.Description = "Changed description"
	if _, e := resolveSnapshot(ambiguousSnapshot, ambiguous, 1); apperr.Code(e) != "parse.collection_game_missing" {
		t.Fatal("untitled change guessed", e)
	}
	var body strings.Builder
	body.WriteString(`<div class="post-user-message">`)
	for i := 1; i <= 10; i++ {
		fmt.Fprintf(&body, `<span style="font-size:24px">Title %d</span><span class="post-b">Описание</span>: Description %d<span class="post-hr"></span>`, i, i)
	}
	body.WriteString(`</div>`)
	bytes := encodedCollection(t, body.String())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(bytes) }))
	defer srv.Close()
	p := &PornlabParser{client: srv.Client()}
	list, err := p.InspectCollection(context.Background(), srv.URL, "Title 10", SearchHint{})
	if err != nil || len(list.Candidates) != 10 || !list.Candidates[0].Best || list.Candidates[0].Index != 10 {
		t.Fatal(list, err)
	}
	bytes = collectionFixture(t, "pornolab-collection-ambiguous.html")
	list, err = p.InspectCollection(context.Background(), srv.URL, "Moonlit Harbor", SearchHint{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range list.Candidates {
		if !c.FallbackTitle || c.Best {
			t.Fatal(c)
		}
	}
}

func TestPornolabCacheCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = w.Write([]byte("<html/>"))
	}))
	defer srv.Close()
	cache := &PornolabCache{}
	p := &PornlabParser{client: srv.Client()}
	done := make(chan error, 1)
	go func() {
		_, err := p.InspectCollection(WithPornolab(context.Background(), PornolabOptions{Cache: cache}), srv.URL, "", SearchHint{})
		done <- err
	}()
	<-started
	cache.Clear()
	close(release)
	if err := <-done; err == nil {
		t.Fatal("late reply accepted")
	}
	if len(cache.pages) != 0 {
		t.Fatal("late reply cached")
	}
	ctx, cancel := cache.Context(context.Background())
	defer cancel()
	cache.Clear()
	if cache.Valid(ctx) {
		t.Fatal("old generation valid")
	}
}

// New saved samples are optional local regressions; normal CI needs only committed neutral fixtures.
func TestPornolabOptionalDownloads(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	files, _ := filepath.Glob(filepath.Join(home, "Downloads", "pornolab*.html"))
	tested := 0
	p := &PornlabParser{client: http.DefaultClient}
	for _, file := range files {
		if strings.Contains(filepath.Base(file), "multi-3006469") {
			continue
		}
		b, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		d := collectionDoc(t, b)
		if len(pornolabSections(d)) > 1 {
			continue
		}
		got, e := p.parseDocument(context.Background(), d, "https://pornolab.net/?t=1", "", PornolabOptions{})
		want, e2 := p.parseLegacy(context.Background(), d, "https://pornolab.net/?t=1", "")
		if e != nil || e2 != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("regression %s: %v / %v", filepath.Base(file), e, e2)
		}
		tested++
	}
	if tested == 0 {
		t.Skip("no additional single-game pornolab*.html in Downloads")
	}
}

func TestPornolabSavedCollectionReference(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	b, err := os.ReadFile(filepath.Join(home, "Downloads", "pornolab-multi-3006469.html"))
	if os.IsNotExist(err) {
		t.Skip("original saved collection not available")
	}
	if err != nil {
		t.Fatal(err)
	}
	doc := collectionDoc(t, b)
	sections := pornolabSections(doc)
	if len(sections) != 2 {
		t.Fatalf("saved reference: %d sections", len(sections))
	}
	for i, s := range sections {
		if s.game.Title == "" || s.game.Description == "" || s.game.Author == "" || s.game.Version == "" || s.game.CoverPath == "" || len(s.game.Images) == 0 || len(s.game.Images) > 15 {
			t.Fatalf("incomplete metadata in saved section %d", i+1)
		}
	}
	if sections[0].game.Title == sections[1].game.Title || sections[0].game.Description == sections[1].game.Description || sections[0].game.CoverPath == sections[1].game.CoverPath {
		t.Fatal("reference sections leaked into each other")
	}
}
