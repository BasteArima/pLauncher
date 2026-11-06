package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Картинки F95 для гостя. Вложения первого поста гостю больше не показываются
// («Log in or register to see attachments»), поэтому в HTML темы их нет. Те же обложка
// и скриншоты есть в публичном JSON страницы «Latest Updates», которую сайт отдаёт без входа.

// f95LatestURL — переменная ради подмены в тестах.
var f95LatestURL = "https://f95zone.to/sam/latest_alpha/latest_data.php"

var (
	f95ThreadIDRe = regexp.MustCompile(`/threads/(?:[^/?#]*\.)?(\d+)`)
	f95WordRe     = regexp.MustCompile(`[\p{L}\p{N}]{3,}`)
)

// f95ThreadID достаёт ID темы из ссылки (.../threads/slug.295551/ или .../threads/295551/), 0 — не нашли.
func f95ThreadID(pageURL string) int {
	m := f95ThreadIDRe.FindStringSubmatch(pageURL)
	if m == nil {
		return 0
	}
	id, _ := strconv.Atoi(m[1])
	return id
}

// f95SearchTerms — слова названия для поиска, самые длинные первыми (до 3 штук).
// Поиск сайта находит по одному слову и ничего не находит по фразе целиком.
func f95SearchTerms(title string) []string {
	var words []string
	seen := map[string]bool{}
	for _, w := range f95WordRe.FindAllString(title, -1) {
		if lw := strings.ToLower(w); !seen[lw] {
			seen[lw] = true
			words = append(words, w)
		}
	}
	sort.SliceStable(words, func(i, j int) bool {
		return utf8.RuneCountInString(words[i]) > utf8.RuneCountInString(words[j])
	})
	if len(words) > 3 {
		words = words[:3]
	}
	return words
}

// f95FullImage: preview.f95zone.to — миниатюры, полный размер лежит по тому же пути на attachments.
func f95FullImage(imgURL string) string {
	return strings.Replace(imgURL, "://preview.f95zone.to/", "://attachments.f95zone.to/", 1)
}

type f95LatestRow struct {
	Title     string   `json:"title"`
	Creator   string   `json:"creator"`
	Version   string   `json:"version"`
	Date      string   `json:"date"`
	Timestamp int64    `json:"ts"`
	ThreadID  int      `json:"thread_id"`
	Cover     string   `json:"cover"`
	Screens   []string `json:"screens"`
}

// f95LatestSearch — один запрос к JSON: строки выдачи по слову из названия.
func f95LatestSearch(ctx context.Context, client *http.Client, term string) ([]f95LatestRow, error) {
	q := url.Values{"cmd": {"list"}, "cat": {"games"}, "page": {"1"}, "rows": {"90"}, "sort": {"date"}, "search": {term}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f95LatestURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, requestErr("F95zone", err)
	}
	var body struct {
		Msg json.RawMessage `json:"msg"`
	}
	if err := requestJSON(client, req, "F95zone", &body); err != nil {
		return nil, err
	}
	var msg struct {
		Data []f95LatestRow `json:"data"`
	}
	if err := json.Unmarshal(body.Msg, &msg); err != nil {
		return nil, readErr(fmt.Errorf("invalid latest updates response"))
	}
	return msg.Data, nil
}

// f95LatestImages находит тему threadID в JSON и возвращает ссылки на полноразмерные
// обложку и скриншоты. Не нашли — пустые значения: текст игры при этом уже разобран.
func f95LatestImages(ctx context.Context, client *http.Client, threadID int, title string) (string, []string, error) {
	var lastErr error
	for _, term := range f95SearchTerms(title) {
		rows, err := f95LatestSearch(ctx, client, term)
		if err != nil {
			lastErr = err
			continue
		}
		for _, row := range rows {
			if row.ThreadID != threadID {
				continue
			}
			cover := f95FullImage(row.Cover)
			var screens []string
			seen := map[string]bool{cover: true}
			for _, s := range row.Screens {
				if s = f95FullImage(s); s != "" && !seen[s] {
					seen[s] = true
					screens = append(screens, s)
				}
			}
			return cover, screens, nil
		}
	}
	return "", nil, lastErr
}
