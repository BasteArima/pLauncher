package parser

import (
	"context"
	"net/http"
	"strings"
)

func searchVNDB(ctx context.Context, client *http.Client, query string) ([]SearchHit, error) {
	rows, err := queryVNDB(ctx, client, "search", query, "id,title,released,developers.name,image.thumbnail", "searchrank", 8)
	if err != nil {
		return nil, err
	}
	var hits []SearchHit
	for _, r := range rows {
		if !vndbIDPattern.MatchString(r.ID) || r.Title == "" {
			continue
		}
		year := ""
		if len(r.Released) >= 4 && r.Released[0] >= '0' && r.Released[0] <= '9' {
			year = r.Released[:4]
		}
		thumb := ""
		if r.Image != nil {
			thumb = r.Image.Thumbnail
		}
		var authors []string
		for _, d := range r.Developers {
			if name := cleanText(d.Name); name != "" {
				authors = append(authors, name)
			}
		}
		hits = append(hits, SearchHit{Source: "VNDB", URL: "https://vndb.org/" + r.ID, Title: cleanText(r.Title), Author: strings.Join(authors, ", "), Year: year, Thumb: thumb})
		if len(hits) == 8 {
			break
		}
	}
	return hits, nil
}
