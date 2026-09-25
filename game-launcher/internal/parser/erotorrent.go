package parser

import (
	"context"
	"net/http"
	"strings"

	"game-launcher/internal/models"

	"github.com/PuerkitoBio/goquery"
)

// ErotorrentParser реализация парсера для erotorrent.ru
type ErotorrentParser struct {
	client *http.Client
}

// Parse скачивает страницу erotorrent и извлекает метаданные
func (p *ErotorrentParser) Parse(ctx context.Context, pageURL string, saveDir string) (*models.Game, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, requestErr("Erotorrent", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, httpStatusErr("Erotorrent", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, readErr(err)
	}

	game := &models.Game{
		Languages: []string{},
	}

	// 1. Парсим заголовок (h1 внутри div.left_full_title)
	title := doc.Find("div.left_full_title h1").Text()
	game.Title = cleanText(title)

	// 2. Парсим версию
	version := doc.Find("span.file_left_1.bold_1").First().Text()
	game.Version = cleanText(version)

	// 3. Парсим описание
	// Берем только первый абзац <p> из блока описания, чтобы отсечь SEO-мусор сайта ("На данной странице вы сможете скачать...")
	desc := doc.Find("div.body_box_text p").First().Text()
	if desc != "" {
		game.Description = cleanText(desc)
	} else {
		game.Description = ""
	}

	// Автор — поле «Разработчик / Издатель» (ссылка на компанию). Движка на сайте нет.
	doc.Find("span.data_1").EachWithBreak(func(i int, s *goquery.Selection) bool {
		if !strings.Contains(s.Find("span.data_b").Text(), "Разработчик") {
			return true
		}
		if a := cleanText(s.Find("a").First().Text()); a != "" {
			game.Author = a
		} else {
			full := cleanText(s.Text())
			game.Author = cleanText(strings.TrimPrefix(full, cleanText(s.Find("span.data_b").Text())))
		}
		return false
	})

	// Языки: сначала пробуем коды в заголовке, затем по тексту описания
	game.Languages = parseReleaseLanguages(title)
	if len(game.Languages) == 0 {
		game.Languages = detectLanguages(doc.Find("div.body_box_text").Text())
	}

	var coverURL string
	var screenshotURLs []string

	// 4. Ищем обложку (афишу)
	if coverSrc, exists := doc.Find("div.left_full_img img.poster").Attr("src"); exists {
		coverURL = absoluteURL(coverSrc, "https://erotorrent.ru")
	}

	// 5. Ищем фулл-сайз скриншоты
	doc.Find("div.body_screen ul.screen li a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		href = absoluteURL(href, "https://erotorrent.ru")
		if href == coverURL {
			return
		}
		for _, u := range screenshotURLs {
			if u == href {
				return
			}
		}
		screenshotURLs = append(screenshotURLs, href)
	})

	downloadInto(ctx, p.client, game, coverURL, screenshotURLs, saveDir, pageURL)
	return game, nil
}
