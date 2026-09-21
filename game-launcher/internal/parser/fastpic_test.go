package parser

import "testing"

func TestFastpicFullviewURL(t *testing.T) {
	got, err := fastpicFullviewURL("https://i128.fastpic.org/big/2026/0913/3d/_f565aa880256e0e1954be44aed63443d.jpg")
	want := "https://fastpic.org/fullview/128/2026/0913/_f565aa880256e0e1954be44aed63443d.jpg.html"
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
	for _, bad := range []string{
		"https://i128.fastpic.org/thumb/2026/0913/3d/x.jpeg",
		"https://imgbox.com/big/2026/0913/3d/x.jpg",
		"https://i128.fastpic.org/big/x.jpg",
	} {
		if _, err := fastpicFullviewURL(bad); err == nil {
			t.Errorf("%q: ожидалась ошибка", bad)
		}
	}
}

func TestFastpicSignedFromHTML(t *testing.T) {
	page := `<style>.x{background:url(https://i124.fastpic.org/big/2026/0810/5e/other.jpg)}</style>
<script>var loading_img = 'https://i128.fastpic.org/big/2026/0913/3d/_f565.jpg?md5=U8zMrCooSSK5iSPXjsFyMA&amp;expires=1790002800';</script>
<a href="https://i128.fastpic.org/big/2026/0913/aa/_other.jpg?md5=zzz&expires=1">`
	got := fastpicSignedFromHTML(page, "_f565.jpg")
	want := "https://i128.fastpic.org/big/2026/0913/3d/_f565.jpg?md5=U8zMrCooSSK5iSPXjsFyMA&expires=1790002800"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := fastpicSignedFromHTML(page, "_missing.jpg"); got != "" {
		t.Errorf("для чужого файла ожидалась пустая строка, got %q", got)
	}
}
