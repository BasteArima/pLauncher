package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSearchSourceFixtures(t *testing.T) {
	f95, steam, vndb := fixture(t, "f95-search-live"), fixture(t, "steam-search-live"), fixture(t, "vndb-search-live")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing user agent")
		}
		switch r.URL.Path {
		case "/f95":
			if r.URL.Query().Get("search") != "Summertime" {
				t.Error(r.URL.RawQuery)
			}
			w.Write(f95)
		case "/steam":
			q := r.URL.Query()
			if q.Get("term") != "Being a DIK" || q.Get("l") != "english" || q.Get("cc") != "US" {
				t.Error(q)
			}
			w.Write(steam)
		case "/vndb":
			var body struct {
				Filters      []string
				Fields, Sort string
				Results      int
			}
			json.NewDecoder(r.Body).Decode(&body)
			if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" || strings.Join(body.Filters, "|") != "search|=|Katawa Shoujo" || body.Sort != "searchrank" || body.Results != 8 || !strings.Contains(body.Fields, "image.thumbnail") {
				t.Errorf("bad VNDB request: %+v", body)
			}
			w.Write(vndb)
		default:
			t.Error(r.URL)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	oldF, oldS, oldV := f95LatestURL, steamSearchURL, vndbAPIURL
	f95LatestURL, steamSearchURL, vndbAPIURL = srv.URL+"/f95", srv.URL+"/steam", srv.URL+"/vndb"
	t.Cleanup(func() { f95LatestURL, steamSearchURL, vndbAPIURL = oldF, oldS, oldV })
	for _, tt := range []struct {
		source, query, title, author, version, year, url string
		count                                            int
	}{
		{"F95zone", "Summertime Saga", "Blue, Summertime Blue", "GIGA", "v1.04", "2026", "https://f95zone.to/threads/319338/", 4},
		{"Steam", "Being a DIK", "Being a DIK - Season 1", "", "", "", "https://store.steampowered.com/app/1126320/", 6},
		{"VNDB", "Katawa Shoujo", "Katawa Shoujo", "Four Leaf Studios", "", "2009", "https://vndb.org/v945", 8},
	} {
		t.Run(tt.source, func(t *testing.T) {
			fn := map[string]func(context.Context, *http.Client, string) ([]SearchHit, error){"F95zone": searchF95, "Steam": searchSteam, "VNDB": searchVNDB}[tt.source]
			hits, err := fn(context.Background(), srv.Client(), tt.query)
			if err != nil || len(hits) != tt.count {
				t.Fatalf("%+v %v", hits, err)
			}
			h := hits[0]
			if h.Source != tt.source || h.Title != tt.title || h.Author != tt.author || h.Version != tt.version || h.Year != tt.year || h.URL != tt.url || h.Thumb == "" {
				t.Fatalf("%+v", h)
			}
			if tt.source == "F95zone" && !strings.Contains(h.Thumb, "attachments.f95zone.to") {
				t.Fatal(h.Thumb)
			}
			if tt.source == "VNDB" && hits[1].Year != "" {
				t.Fatal("null release date became a year")
			}
		})
	}
}

func TestSearchScoring(t *testing.T) {
	for _, tt := range []struct {
		query, title, version, hint string
		want                        float64
	}{
		{"The Café Moon!", "cafe moon", "", "", .9},
		{"A Moon", "Moon", "v0.3", "0.3", 1},
		{"Moon", "Moon", "v0.4", "0.3", .9},
		{"Moon Bay", "Moon Bay Nights", "", "", .55},
		{"Moon Bay", "Bay Sun", "", "", .25},
		{"Moon Moon", "Moon", "", "", .8},
		{"Moon", "Stars", "", "", 0},
		{"", "", "", "", 0},
		{"!!!", "...", "0.3", "0.3", 0},
	} {
		if got := scoreSearchHit(tt.query, SearchHit{Title: tt.title, Version: tt.version}, SearchHint{Version: tt.hint}); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("%+v: %v", tt, got)
		}
	}
	for _, tt := range []struct {
		titles []string
		best   bool
	}{
		{[]string{"Moon Bay"}, true}, {[]string{"Moon Bay", "Other"}, true},
		{[]string{"Moon Bay", "The Moon Bay"}, false}, {[]string{"Moon Bay Nights"}, false},
	} {
		var hits []SearchHit
		for _, title := range tt.titles {
			hits = append(hits, SearchHit{Title: title, Best: true})
		}
		rankSearchHits(hits, "Moon Bay", SearchHint{})
		if hits[0].Best != tt.best {
			t.Errorf("%+v", hits)
		}
		for _, h := range hits[1:] {
			if h.Best {
				t.Error("runner-up is Best")
			}
		}
	}
}

func TestSearchParallelTimeoutAndFailures(t *testing.T) {
	started := make(chan struct{}, 3)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		if r.URL.Path == "/slow" {
			<-r.Context().Done()
			return
		}
		// Все источники должны стартовать до ответа любого из них.
		for len(started) < 3 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(time.Millisecond):
			}
		}
		if r.URL.Path == "/blocked" {
			w.WriteHeader(429)
			return
		}
		fmt.Fprint(w, `{"items":[{"id":1,"name":"Moon Bay"}]}`)
	}))
	defer srv.Close()
	makeSource := func(name string) searchSource {
		return searchSource{name, func(ctx context.Context, c *http.Client, q string) ([]SearchHit, error) {
			req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/"+name, nil)
			var body any
			err := requestJSON(c, req, name, &body)
			return []SearchHit{{Title: q}}, err
		}}
	}
	r := searchWithSources(context.Background(), srv.Client(), "Moon Bay", nil, SearchHint{}, 150*time.Millisecond, []searchSource{makeSource("slow"), makeSource("ok"), makeSource("blocked")})
	if len(r.Hits) != 1 || !r.Hits[0].Best || len(r.Failed) != 2 || r.Failed[0].Source != "slow" || !strings.Contains(r.Failed[0].Error, "[parse.request") || !strings.Contains(r.Failed[1].Error, "[parse.blocked") {
		t.Fatalf("%+v", r)
	}
}

func TestSearchLimitsSelectionAndCancellation(t *testing.T) {
	var calls atomic.Int32
	s := searchSource{"Steam", func(ctx context.Context, c *http.Client, q string) ([]SearchHit, error) {
		calls.Add(1)
		hits := make([]SearchHit, 12)
		for i := range hits {
			hits[i].Title = fmt.Sprintf("Moon %d", i)
		}
		hits[10].Title = "Moon"
		return hits, nil
	}}
	r := searchWithSources(context.Background(), nil, "Moon", []string{"steam", "Steam", "Unknown"}, SearchHint{}, time.Second, []searchSource{s})
	if len(r.Hits) != 8 || len(r.Failed) != 1 || calls.Load() != 1 || r.Hits[0].Title == "Moon" {
		t.Fatal(r, calls.Load())
	}
	for _, q := range []string{"", " "} {
		r = Search(context.Background(), nil, q, nil, SearchHint{})
		if len(r.Hits)+len(r.Failed) != 0 {
			t.Fatal(r)
		}
	}
	r = Search(context.Background(), nil, "Moon", []string{}, SearchHint{})
	if len(r.Hits)+len(r.Failed) != 0 {
		t.Fatal(r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = Search(ctx, nil, "Moon", nil, SearchHint{})
	if len(r.Failed) != 3 {
		t.Fatal(r)
	}
}

func TestSearchSourceErrorsAndF95Fallback(t *testing.T) {
	oldF, oldS, oldV := f95LatestURL, steamSearchURL, vndbAPIURL
	t.Cleanup(func() { f95LatestURL, steamSearchURL, vndbAPIURL = oldF, oldS, oldV })
	var terms []string
	mode := "fallback"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if mode == "fallback" {
			term := r.URL.Query().Get("search")
			terms = append(terms, term)
			if term == "Longest" {
				fmt.Fprint(w, `{"msg":{"data":[]}}`)
			} else {
				fmt.Fprint(w, `{"msg":{"data":[{"thread_id":1,"title":"Longest Moon"}]}}`)
			}
			return
		}
		if mode == "blocked" {
			w.WriteHeader(403)
			return
		}
		fmt.Fprint(w, `broken json`)
	}))
	defer srv.Close()
	f95LatestURL, steamSearchURL, vndbAPIURL = srv.URL, srv.URL, srv.URL
	hits, err := searchF95(context.Background(), srv.Client(), "Moon Longest")
	if err != nil || len(hits) != 1 || strings.Join(terms, ",") != "Longest,Moon" {
		t.Fatal(hits, err, terms)
	}
	for _, m := range []string{"blocked", "invalid"} {
		mode = m
		for _, fn := range []func(context.Context, *http.Client, string) ([]SearchHit, error){searchF95, searchSteam, searchVNDB} {
			_, err := fn(context.Background(), srv.Client(), "Moon")
			code := "[parse.read"
			if m == "blocked" {
				code = "[parse.blocked"
			}
			if err == nil || !strings.Contains(err.Error(), code) {
				t.Fatal(err)
			}
		}
	}
}
