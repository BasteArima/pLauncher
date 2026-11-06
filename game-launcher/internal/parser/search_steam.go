package parser

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

var steamSearchURL = "https://store.steampowered.com/api/storesearch/"

func searchSteam(ctx context.Context, client *http.Client, query string) ([]SearchHit, error) {
	q := url.Values{"term": {query}, "l": {"english"}, "cc": {"US"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, steamSearchURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, requestErr("Steam", err)
	}
	var body struct {
		Items []struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Image string `json:"tiny_image"`
			Type  string `json:"type"`
		} `json:"items"`
	}
	if err = requestJSON(client, req, "Steam", &body); err != nil {
		return nil, err
	}
	var hits []SearchHit
	for _, r := range body.Items {
		if r.ID <= 0 || r.Name == "" || (r.Type != "" && r.Type != "app") {
			continue
		}
		hits = append(hits, SearchHit{Source: "Steam", URL: fmt.Sprintf("https://store.steampowered.com/app/%d/", r.ID), Title: cleanText(r.Name), Thumb: r.Image})
		if len(hits) == 8 {
			break
		}
	}
	return hits, nil
}
