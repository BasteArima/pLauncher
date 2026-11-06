package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"game-launcher/internal/models"
)

func TestVNDBParserRecordedResponse(t *testing.T) {
	data := fixture(t, "vndb-detail-live")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Filters []string
			Fields  string
			Results int
		}
		json.NewDecoder(r.Body).Decode(&body)
		if strings.Join(body.Filters, "|") != "id|=|v945" || body.Results != 1 || !strings.Contains(body.Fields, "tags.spoiler") || !strings.Contains(body.Fields, "screenshots.url") {
			t.Errorf("%+v", body)
		}
		w.Write(data)
	}))
	defer srv.Close()
	old := vndbAPIURL
	vndbAPIURL = srv.URL
	t.Cleanup(func() { vndbAPIURL = old })
	p, err := GetParser("https://vndb.org/v945", "")
	if err != nil {
		t.Fatal(err)
	}
	g, err := p.Parse(context.Background(), "https://vndb.org/v945", "")
	if err != nil {
		t.Fatal(err)
	}
	if g.Title != "Katawa Shoujo" || g.Author != "Four Leaf Studios" || g.Version != "" || g.Engine != "" || len(g.Images) != 10 || len(g.Tags) != 15 || !strings.HasPrefix(g.CoverPath, "https://t.vndb.org/") || !strings.Contains(g.Description, "Hisao Nakai") {
		t.Fatalf("%+v", g)
	}
	if strings.Contains(strings.Join(g.Tags, ","), "Death of Protagonist") {
		t.Fatal("spoiler tag leaked")
	}
	if strings.Count(strings.Join(g.Languages, ","), "中文") != 1 || !strings.Contains(strings.Join(g.Languages, ","), "Русский") {
		t.Fatal(g.Languages)
	}
	if SourceName("https://vndb.org/v945") != "VNDB" {
		t.Fatal("missing source")
	}
}

func TestVNDBParserEdgesAndDownloads(t *testing.T) {
	var endpoint string
	mode := "detail"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/image" {
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("\x89PNG\r\n\x1a\n0000"))
			return
		}
		switch mode {
		case "missing":
			fmt.Fprint(w, `{"results":[]}`)
		case "null":
			fmt.Fprint(w, `{"results":[{"id":"v1","title":"Empty","image":null,"released":null}]}`)
		case "error":
			w.WriteHeader(503)
		default:
			json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"id": "v1", "title": "Moon", "description": "[b]Hello[/b] [spoiler]secret[/spoiler]", "image": map[string]string{"url": endpoint + "/image"}, "screenshots": []any{map[string]string{"url": endpoint + "/image"}}, "tags": []any{map[string]any{"name": "Low", "rating": 1, "spoiler": 0}, map[string]any{"name": "Secret", "rating": 3, "spoiler": 1}, map[string]any{"name": "High", "rating": 2, "spoiler": 0}}}}})
		}
	}))
	defer srv.Close()
	endpoint = srv.URL
	old := vndbAPIURL
	vndbAPIURL = srv.URL
	t.Cleanup(func() { vndbAPIURL = old })
	p := &VNDBParser{client: srv.Client()}
	for _, u := range []string{"https://vndb.org/v1/extra", "https://vndb.org/v0", "https://vndb.org/r1", "https://evil.test/v1", "https://vndb.org.evil.test/v1"} {
		if _, err := p.Parse(context.Background(), u, ""); err == nil || !strings.Contains(err.Error(), "[vndb.id") {
			t.Fatal(u, err)
		}
	}
	dir := t.TempDir()
	g, err := p.Parse(context.Background(), "https://vndb.org/v1", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g.Tags, []string{"High", "Low"}) || strings.Contains(g.Description, "secret") {
		t.Fatal(g)
	}
	for _, file := range append(g.Images, g.CoverPath) {
		if filepath.Dir(file) != dir {
			t.Fatal(file)
		}
		if _, err := os.Stat(file); err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range []string{"null", "missing", "error"} {
		mode = m
		g, err = p.Parse(context.Background(), "https://vndb.org/v1", "")
		if m == "null" {
			if err != nil || g.CoverPath != "" {
				t.Fatal(g, err)
			}
		} else if err == nil {
			t.Fatal("expected error", m)
		}
	}
}

func TestVNDBBBCode(t *testing.T) {
	for _, tt := range []struct{ in, out string }{
		{`[url=https://example.org]label[/url]`, "label"},
		{`[b]Bold[/b] [i]italics[/i] &amp; [u]underline[/u]`, "Bold italics & underline"},
		{`before[spoiler]secret [spoiler]nested[/spoiler] secret[/spoiler]after`, "beforeafter"},
		{`before[SPOILER=hint]secret[/SPOILER]after`, "beforeafter"},
		{`before[spoiler]unfinished`, "before"},
		{`[quote]Text[/quote][br]Next`, "Text\nNext"},
		{`Plain [ordinary] text`, "Plain [ordinary] text"},
	} {
		if got := stripVNDBBBCode(tt.in); got != tt.out {
			t.Errorf("%q => %q, want %q", tt.in, got, tt.out)
		}
	}
}

func TestMetadataOnlyReturnsImagesWithoutRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("preview downloaded an image") }))
	defer srv.Close()
	urls := make([]string, 20)
	for i := range urls {
		urls[i] = fmt.Sprintf("%s/%d", srv.URL, i)
	}
	g := &models.Game{}
	downloadInto(context.Background(), srv.Client(), g, srv.URL+"/cover", urls, "", "")
	if g.CoverPath != srv.URL+"/cover" || !reflect.DeepEqual(g.Images, urls[:15]) {
		t.Fatal(g)
	}
	urls[0] = "changed"
	if g.Images[0] == "changed" {
		t.Fatal("preview aliases caller's slice")
	}
}
