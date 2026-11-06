package parser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Test seam for optional thread metadata requests.
var f95SearchThreadURL = "https://f95zone.to/threads/"

func searchF95(ctx context.Context, client *http.Client, query string) ([]SearchHit, error) {
	for _, term := range f95SearchTerms(query) {
		rows, err := f95LatestSearch(ctx, client, term)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			continue
		}
		hits := make([]SearchHit, 0, 8)
		var engineRows []int
		for _, r := range rows {
			if r.ThreadID <= 0 || r.Title == "" {
				continue
			}
			year, updatedAt := "", ""
			if r.Timestamp > 0 {
				year = time.Unix(r.Timestamp, 0).UTC().Format("2006")
				updatedAt = time.Unix(r.Timestamp, 0).UTC().Format(time.RFC3339)
			}
			hits = append(hits, SearchHit{Source: "F95zone", URL: fmt.Sprintf("https://f95zone.to/threads/%d/", r.ThreadID), Title: cleanText(r.Title), Author: cleanText(r.Creator), Version: r.Version, Year: year, UpdatedAt: updatedAt, Thumb: f95FullImage(r.Cover)})
			// Prefix IDs are site-specific; resolve their actual labels from the thread.
			if len(r.Prefixes) > 0 {
				engineRows = append(engineRows, len(hits)-1)
			}
			if len(hits) == 8 {
				break
			}
		}
		enrichF95SearchEngines(ctx, client, hits, engineRows)
		return hits, nil
	}
	return nil, nil
}

// Optional enrichment cannot turn a successful search into a failed source.
// Four concurrent requests share a three-second budget inside the source timeout.
func enrichF95SearchEngines(ctx context.Context, client *http.Client, hits []SearchHit, indices []int) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	slots := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, index := range indices {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-slots }()
			id := f95ThreadID(hits[i].URL)
			hits[i].Engine = f95SearchEngine(ctx, client, fmt.Sprintf("%s%d/", f95SearchThreadURL, id))
		}(index)
	}
	wg.Wait()
}

func f95SearchEngine(ctx context.Context, client *http.Client, pageURL string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return ""
	}
	return f95DocumentEngine(doc)
}
