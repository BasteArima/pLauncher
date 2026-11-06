package parser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// AVIF-заголовок: сервер F95 отдаёт такие данные и по ссылкам вида .png
var avifBytes = append([]byte{0, 0, 0, 0x20}, []byte("ftypavif----")...)

// Страница темы глазами гостя: вместо вложений — призыв войти, картинок в посте нет.
const f95GuestPage = `<html><body>
<h1 class="p-title-value"><span class="p-title-prefix"><a class="labelLink"><span>VN</span></a><a class="labelLink"><span>Ren&#039;Py</span></a></span>Living with Chainsaw [v0.90] [Dr.Ergenecon]</h1>
<dl class="tagList"><dd><span class="js-tagList">
<a href="/tags/2dcg/" class="tagItem" dir="auto"> 2dcg </a>
<a href="/tags/romance/" class="tagItem" dir="auto"> romance </a>
</span></dd></dl>
<article class="message-body js-selectToQuote"><div class="bbWrapper">
Log in or register to see attachments
<b>Overview</b>:<br />A story about a boy and a chainsaw.<br /><b>Language</b>: English, Turkish.<br />
Thanks to the Russian translator!
</div></article>
<article class="message-body"><div class="bbWrapper"><img class="bbImage" src="https://example.com/reply.jpg" /></div></article>
</body></html>`

func TestF95GuestPageUsesLatestJSON(t *testing.T) {
	var searches []string
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/threads/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, f95GuestPage) })
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		term := r.URL.Query().Get("search")
		searches = append(searches, term)
		if term != "Living" { // первое (самое длинное) слово темы не находит
			fmt.Fprint(w, `{"status":"ok","msg":{"data":[{"thread_id":1,"cover":"x","screens":[]}]}}`)
			return
		}
		fmt.Fprintf(w, `{"status":"ok","msg":{"data":[{"thread_id":295551,"cover":"%[1]s/img/cover.png","screens":["%[1]s/img/a.png","%[1]s/img/cover.png","%[1]s/img/b.png"]}]}}`, srv.URL)
	})
	mux.HandleFunc("/img/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/avif")
		w.Write(avifBytes)
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	saved := f95LatestURL
	f95LatestURL = srv.URL + "/latest"
	defer func() { f95LatestURL = saved }()

	pageURL := srv.URL + "/threads/living-with-chainsaw-v0-90-dr-ergenecon.295551/"
	p := &F95Parser{client: srv.Client()}
	dir := t.TempDir()
	g, err := p.Parse(context.Background(), pageURL, dir)
	if err != nil {
		t.Fatal(err)
	}

	if g.Title != "Living with Chainsaw" || g.Version != "v0.90" || g.Author != "Dr.Ergenecon" {
		t.Errorf("текст: %q %q %q", g.Title, g.Version, g.Author)
	}
	if want := []string{"2dcg", "romance"}; !reflect.DeepEqual(g.Tags, want) {
		t.Errorf("tags = %v, want %v", g.Tags, want)
	}
	// Только поле Language: русский упомянут в тексте поста, но языком игры не указан
	if want := []string{"English", "Türkçe"}; !reflect.DeepEqual(g.Languages, want) {
		t.Errorf("languages = %v, want %v", g.Languages, want)
	}
	if want := []string{"Chainsaw", "Living"}; !reflect.DeepEqual(searches, want) {
		t.Errorf("поиски = %v, want %v", searches, want)
	}
	// Обложка не дублируется в скриншотах, расширение соответствует настоящему формату
	if filepath.Base(g.CoverPath) != "cover_0.avif" {
		t.Errorf("cover = %q", g.CoverPath)
	}
	if len(g.Images) != 2 || filepath.Base(g.Images[0]) != "shot_0.avif" {
		t.Errorf("images = %v", g.Images)
	}
	if _, err := os.Stat(g.CoverPath); err != nil {
		t.Error(err)
	}

	// Проверка обновлений (только метаданные) в JSON не ходит
	searches = nil
	g, err = p.Parse(context.Background(), pageURL, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(searches) != 0 || g.Version != "v0.90" {
		t.Errorf("проверка обновлений: поиски %v, версия %q", searches, g.Version)
	}

	// Предпросмотр получает удалённые ссылки через JSON, но не скачивает картинки.
	searches = nil
	g, err = p.Parse(WithPreviewImages(context.Background()), pageURL, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(searches, []string{"Chainsaw", "Living"}) || g.Version != "v0.90" || g.CoverPath != srv.URL+"/img/cover.png" || len(g.Images) != 2 {
		t.Errorf("предпросмотр: поиски %v, версия %q", searches, g.Version)
	}
}

// Ошибка JSON не должна ломать разбор текста игры
func TestF95LatestFailureKeepsText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/threads/") {
			fmt.Fprint(w, f95GuestPage)
			return
		}
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()

	saved := f95LatestURL
	f95LatestURL = srv.URL + "/latest"
	defer func() { f95LatestURL = saved }()

	p := &F95Parser{client: srv.Client()}
	g, err := p.Parse(context.Background(), srv.URL+"/threads/x.295551/", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if g.Title != "Living with Chainsaw" || g.CoverPath != "" {
		t.Errorf("title %q, cover %q", g.Title, g.CoverPath)
	}
}

func TestF95Helpers(t *testing.T) {
	for in, want := range map[string]int{
		"https://f95zone.to/threads/living-with-chainsaw-v0-90-dr-ergenecon.295551/": 295551,
		"https://f95zone.to/threads/295551/":                                         295551,
		"https://f95zone.to/threads/some-game.123/post-456":                          123,
		"https://f95zone.to/forums/games.2/":                                         0,
	} {
		if got := f95ThreadID(in); got != want {
			t.Errorf("f95ThreadID(%s) = %d, want %d", in, got, want)
		}
	}
	if got, want := f95SearchTerms("Living with Chainsaw: Part 2 of it, Chainsaw"), []string{"Chainsaw", "Living", "with"}; !reflect.DeepEqual(got, want) {
		t.Errorf("f95SearchTerms = %v, want %v", got, want)
	}
	if got, want := f95FullImage("https://preview.f95zone.to/2026/08/1_a.png"), "https://attachments.f95zone.to/2026/08/1_a.png"; got != want {
		t.Errorf("f95FullImage = %q", got)
	}
}

func TestFixImageExt(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 8)...)
	jpg := append([]byte("\xff\xd8\xff\xe0"), make([]byte, 12)...)
	webp := []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")

	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"a.png", avifBytes, "a.avif"},
		{"b.jpg", webp, "b.webp"},
		{"c.png", png, "c.png"},
		{"d.jpeg", jpg, "d.jpeg"},
		{"e.jpg", []byte("short"), "e.jpg"}, // не распознано — не трогаем
		{"f.webp", []byte("not an image at all"), "f.webp"},
	}
	for _, c := range cases {
		if got := filepath.Base(fixImageExt(write(c.name, c.data))); got != c.want {
			t.Errorf("fixImageExt(%s) = %s, want %s", c.name, got, c.want)
		}
	}
}
