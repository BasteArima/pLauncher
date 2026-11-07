package parser

import "testing"

func TestConfidentSearchURL(t *testing.T) {
	good := SearchHit{Title: "The Café — Moon", URL: "one", Source: "F95zone", Author: "Studio", Score: .95, Best: true}
	for _, tc := range []struct {
		name, query string
		hits        []SearchHit
		want        string
	}{
		{"normalized", "Cafe Moon", []SearchHit{good}, "one"},
		{"empty", "", []SearchHit{good}, ""},
		{"no results", "Cafe Moon", nil, ""},
		{"sequel", "Cafe Moon 2", []SearchHit{good}, ""},
		{"best required", "Cafe Moon", []SearchHit{{Title: "Cafe Moon", URL: "one", Score: .9}}, ""},
		{"highest must qualify", "Cafe Moon", []SearchHit{good, {Title: "Other", URL: "two", Score: 1}}, ""},
		{"conflicting source winner", "Cafe Moon", []SearchHit{good, {Title: "Other", URL: "two", Source: "Steam", Best: true, Score: .9}}, ""},
		{"different authors", "Cafe Moon", []SearchHit{good, {Title: "Cafe Moon", URL: "two", Source: "Steam", Author: "Other Studio", Score: .8}}, ""},
		{"same source different records", "Cafe Moon", []SearchHit{good, {Title: "Cafe Moon", URL: "two", Source: "F95zone", Best: true, Score: .9}}, ""},
		{"same game cross site", "Cafe Moon", []SearchHit{good, {Title: "Cafe Moon", URL: "two", Source: "Steam", Author: "studio", Best: true, Score: .9}}, "one"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ConfidentSearchURL(tc.query, SearchResult{Hits: tc.hits}); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	// Existing ranking must keep tied exact titles for manual selection.
	hits := []SearchHit{{Title: "Cafe Moon", URL: "one"}, {Title: "Cafe Moon", URL: "two"}}
	rankSearchHits(hits, "Cafe Moon", SearchHint{})
	if ConfidentSearchURL("Cafe Moon", SearchResult{Hits: hits}) != "" {
		t.Fatal("tie was auto-applied")
	}
}
