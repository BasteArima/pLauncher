package parser

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

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
		for _, r := range rows {
			if r.ThreadID <= 0 || r.Title == "" {
				continue
			}
			year := ""
			if r.Timestamp > 0 {
				year = time.Unix(r.Timestamp, 0).UTC().Format("2006")
			}
			hits = append(hits, SearchHit{Source: "F95zone", URL: fmt.Sprintf("https://f95zone.to/threads/%d/", r.ThreadID), Title: cleanText(r.Title), Author: cleanText(r.Creator), Version: r.Version, Year: year, Thumb: f95FullImage(r.Cover)})
			if len(hits) == 8 {
				break
			}
		}
		return hits, nil
	}
	return nil, nil
}
