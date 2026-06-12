package parser

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"game-launcher/internal/models"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/charmap"
)

// PornlabParser реализация парсера для pornolab.net
type PornlabParser struct {
	client *http.Client
}

func (p *PornlabParser) Parse(ctx context.Context, pageURL string, saveDir string) (*models.Game, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pornolab request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("pornolab returned status %d", resp.StatusCode)
	}

	decoder := charmap.Windows1251.NewDecoder()
	doc, err := goquery.NewDocumentFromReader(decoder.Reader(resp.Body))
	if err != nil {
		return nil, fmt.Errorf("HTML read error: %w", err)
	}

	game := &models.Game{
		Languages: []string{},
	}

	// Заголовок раздачи (#topic-title / h1.maintitle) имеет стабильный формат:
	//   "Название [версия] (разработчик) [cen] [год, теги] [языки]"
	// Из него надёжно достаются название, версия и языки — в отличие от
	// разнородного оформления самого поста.
	rawTitle := cleanText(doc.Find("h1.maintitle, #topic-title").First().Text())
	if rawTitle != "" {
		game.Title = cleanReleaseTitle(rawTitle)
		game.Version = versionFromTitle(rawTitle)
		game.Languages = parseReleaseLanguages(rawTitle)
		game.Tags = parseReleaseTags(rawTitle)
	}

	// Запасной вариант названия — крупный заголовок внутри поста
	if game.Title == "" {
		game.Title = cleanText(doc.Find("span[style*='font-size: 24px'], span[style*='font-size:24px']").First().Text())
	}

	post := doc.Find("div.post-user-message").First()

	// Описание берём из тела поста, вырезав спойлеры/картинки/разделители
	game.Description = extractPornolabDescription(post)
	if game.Description == "" {
		game.Description = ""
	}

	// Языки: если в заголовке не нашлись — пробуем по тексту поста
	if len(game.Languages) == 0 {
		game.Languages = detectLanguages(post.Text())
	}
	if game.Languages == nil {
		game.Languages = []string{}
	}

	// Картинки: реальный URL хранится в атрибуте title тега var.postImg
	// (в src на сохранённых страницах подставляется локальный путь).
	var coverURL string
	var screenshotURLs []string

	doc.Find("var.postImg, img.postImg").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("title")
		if src == "" {
			src, _ = s.Attr("src")
		}
		if src == "" {
			return
		}

		// АНТИ-МУСОР: баннеры трекера и смайлы
		if strings.Contains(src, "static.pornolab.net") || strings.Contains(src, "smilies") {
			return
		}

		// ХАК ДЛЯ FASTPIC: thumb -> big
		src = strings.ReplaceAll(src, "/thumb/", "/big/")

		// ХАК ДЛЯ IMGBOX: thumbnail -> original
		if strings.Contains(src, "imgbox.com") {
			src = strings.Replace(src, "thumbs2.imgbox.com", "images2.imgbox.com", 1)
			src = strings.Replace(src, "thumbs.imgbox.com", "images.imgbox.com", 1)
			src = strings.Replace(src, "_t.jpg", "_o.jpg", 1)
			src = strings.Replace(src, "_t.png", "_o.png", 1)
		}

		if strings.HasPrefix(src, "//") {
			src = "https:" + src
		} else if strings.HasPrefix(src, "/") {
			src = "https://pornolab.net" + src
		}

		if coverURL == "" {
			coverURL = src
			return
		}
		if src == coverURL {
			return
		}
		for _, u := range screenshotURLs {
			if u == src {
				return
			}
		}
		screenshotURLs = append(screenshotURLs, src)
	})

	downloadInto(ctx, p.client, game, coverURL, screenshotURLs, saveDir, pageURL)
	return game, nil
}

// extractPornolabDescription достаёт прозу описания из тела поста.
// Спойлеры (.sp-wrap) — это «Скриншоты», «Патчноут», «Порядок установки» —
// и картинки удаляются, после чего берётся текст после метки «Описание»
// либо после рамки спецификаций (╚════╝).
func extractPornolabDescription(post *goquery.Selection) string {
	if post.Length() == 0 {
		return ""
	}
	clone := post.Clone()
	clone.Find(".sp-wrap, var.postImg, img, .post-hr").Remove()
	text := clone.Text()

	start := 0
	if idx := caseIndex(text, "Описание"); idx >= 0 {
		// начинаем после двоеточия, следующего за меткой
		rest := text[idx:]
		if c := strings.IndexByte(rest, ':'); c >= 0 {
			start = idx + c + 1
		} else {
			start = idx + len("Описание")
		}
	} else if idx := strings.Index(text, "╚"); idx >= 0 {
		// рамка спецификаций закрыта — описание идёт следующей строкой
		if nl := strings.IndexByte(text[idx:], '\n'); nl >= 0 {
			start = idx + nl + 1
		} else {
			start = idx + len("╚")
		}
	}

	desc := text[start:]

	// Отсекаем хвост со скриншотами, если он всё же просочился
	for _, marker := range []string{"Скриншот", "Screenshot", "Доп. скрин"} {
		if i := caseIndex(desc, marker); i >= 0 {
			desc = desc[:i]
		}
	}

	return truncateText(cleanText(desc), 2000)
}
