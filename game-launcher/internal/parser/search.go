package parser

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"game-launcher/internal/apperr"
	"golang.org/x/text/unicode/norm"
)

type SearchHit struct {
	// Optional source metadata. UpdatedAt is UTC RFC3339, never a release date.
	Engine    string  `json:"engine"`
	UpdatedAt string  `json:"updated_at"`
	Source    string  `json:"source"`
	URL       string  `json:"url"`
	Title     string  `json:"title"`
	Author    string  `json:"author"`
	Version   string  `json:"version"`
	Year      string  `json:"year"`
	Thumb     string  `json:"thumb"`
	Score     float64 `json:"score"`
	Best      bool    `json:"best"`
}
type SearchFailure struct {
	Source string `json:"source"`
	Error  string `json:"error"`
}
type SearchResult struct {
	Hits   []SearchHit     `json:"hits"`
	Failed []SearchFailure `json:"failed"`
}
type SearchHint struct {
	Version string `json:"version"`
	Engine  string `json:"engine"`
}

type searchSource struct {
	name   string
	search func(context.Context, *http.Client, string) ([]SearchHit, error)
}

// Search собирает независимые результаты; ошибка одного сайта не скрывает остальные.
// nil sources включает все сайты, пустой список — ни одного.
func Search(ctx context.Context, client *http.Client, query string, sources []string, hint SearchHint) SearchResult {
	return searchWithSources(ctx, client, query, sources, hint, 10*time.Second, []searchSource{{"F95zone", searchF95}, {"Steam", searchSteam}, {"VNDB", searchVNDB}})
}

func searchWithSources(ctx context.Context, client *http.Client, query string, sources []string, hint SearchHint, timeout time.Duration, available []searchSource) SearchResult {
	result := SearchResult{Hits: []SearchHit{}, Failed: []SearchFailure{}}
	query = strings.TrimSpace(query)
	if query == "" {
		return result
	}
	if client == nil {
		client = newHTTPClient("")
		defer client.CloseIdleConnections()
	}
	if sources == nil {
		for _, s := range available {
			sources = append(sources, s.name)
		}
	}
	type response struct {
		hits []SearchHit
		err  error
	}
	var selected []searchSource
	seen := map[string]bool{}
	for _, name := range sources {
		key := strings.ToLower(strings.TrimSpace(name))
		if seen[key] {
			continue
		}
		seen[key] = true
		found := false
		for _, s := range available {
			if strings.EqualFold(key, s.name) {
				selected = append(selected, s)
				found = true
				break
			}
		}
		if !found {
			result.Failed = append(result.Failed, SearchFailure{name, apperr.New("parse.unsupported", nil, "unknown source or unsupported site").Error()})
		}
	}
	responses := make([]response, len(selected))
	var wg sync.WaitGroup
	for i, s := range selected {
		wg.Add(1)
		go func(i int, s searchSource) {
			defer wg.Done()
			sourceCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			responses[i].hits, responses[i].err = s.search(sourceCtx, client, query)
		}(i, s)
	}
	wg.Wait()
	for i, r := range responses {
		if r.err != nil {
			result.Failed = append(result.Failed, SearchFailure{selected[i].name, r.err.Error()})
			continue
		}
		if len(r.hits) > 8 {
			r.hits = r.hits[:8]
		}
		for _, h := range r.hits {
			h.Source = selected[i].name
			result.Hits = append(result.Hits, h)
		}
	}
	rankSearchHits(result.Hits, query, hint)
	if len(result.Hits) > 8 {
		result.Hits = result.Hits[:8]
	}
	return result
}

func normTitle(title string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(title)) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	words := strings.Fields(b.String())
	if len(words) > 0 && (words[0] == "the" || words[0] == "a") {
		words = words[1:]
	}
	return strings.Join(words, " ")
}

func scoreSearchHit(query string, hit SearchHit, hint SearchHint) float64 {
	a, b := normTitle(query), normTitle(hit.Title)
	if a == "" || b == "" {
		return 0
	}
	union := map[string]bool{}
	left := map[string]bool{}
	for _, s := range strings.Fields(a) {
		union[s] = true
		left[s] = true
	}
	intersection := 0
	for _, s := range strings.Fields(b) {
		if left[s] {
			intersection++
			delete(left, s)
		}
		union[s] = true
	}
	score := 0.75 * float64(intersection) / float64(len(union))
	if a == b {
		score += 0.15
	}
	version := func(s string) string {
		return strings.TrimPrefix(strings.ToLower(strings.Join(strings.Fields(s), " ")), "v")
	}
	if v := version(hint.Version); v != "" && v == version(hit.Version) {
		score += 0.10
	}
	if a != b && (strings.HasPrefix(a, b+" ") || strings.HasPrefix(b, a+" ")) {
		score += 0.05
	}
	return math.Min(1, score)
}

func rankSearchHits(hits []SearchHit, query string, hint SearchHint) {
	for i := range hits {
		hits[i].Score = scoreSearchHit(query, hits[i], hint)
		hits[i].Best = false
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > 0 {
		hits[0].Best = hits[0].Score >= 0.8 && (len(hits) == 1 || hits[0].Score-hits[1].Score >= 0.15)
	}
}

// requestJSON сохраняет локализованные ошибки и контекст отмены для всех API.
func requestJSON(client *http.Client, req *http.Request, site string, out any) error {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return requestErr(site, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return httpStatusErr(site, resp.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out); err != nil {
		return readErr(err)
	}
	return nil
}
