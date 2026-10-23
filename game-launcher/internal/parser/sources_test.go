package parser

import "testing"

// erotorrent переехал с .ru на .org — оба адреса должны разбираться одним парсером
func TestErotorrentDomains(t *testing.T) {
	for _, u := range []string{
		"https://erotorrent.org/anime/6481-the-edge-of-definitive-edition.html",
		"https://erotorrent.ru/anime/6481-the-edge-of-definitive-edition.html",
		"https://www.erotorrent.org/x.html",
	} {
		p, err := GetParser(u, "")
		if err != nil {
			t.Errorf("GetParser(%s): %v", u, err)
			continue
		}
		if _, ok := p.(*ErotorrentParser); !ok {
			t.Errorf("GetParser(%s) = %T, want *ErotorrentParser", u, p)
		}
		if got := SourceName(u); got != "Erotorrent" {
			t.Errorf("SourceName(%s) = %q, want Erotorrent", u, got)
		}
	}
	if isErotorrent("https://example.com/?ref=erotorrent.org") {
		t.Error("домен в параметрах запроса не должен считаться erotorrent")
	}
}

func TestPageOrigin(t *testing.T) {
	origin := pageOrigin("https://erotorrent.org/anime/6481-x.html", "https://fallback")
	if origin != "https://erotorrent.org" {
		t.Errorf("pageOrigin = %q", origin)
	}
	if got := absoluteURL("/uploads/posts/a.webp", origin); got != "https://erotorrent.org/uploads/posts/a.webp" {
		t.Errorf("absoluteURL = %q", got)
	}
	if got := pageOrigin("not a url", "https://fallback"); got != "https://fallback" {
		t.Errorf("pageOrigin(bad) = %q", got)
	}
}
