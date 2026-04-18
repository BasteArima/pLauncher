package parser

import (
	"context"
	"fmt"
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
		return nil, fmt.Errorf("ошибка запроса к erotorrent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("erotorrent вернул статус %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения HTML: %w", err)
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
		game.Description = "Описание не найдено."
	}

	var imageURLs []string

	// 4. Ищем обложку (афишу)
	coverSrc, exists := doc.Find("div.left_full_img img.poster").Attr("src")
	if exists {
		// erotorrent часто отдает относительные пути (начинаются с /), делаем их абсолютными
		if strings.HasPrefix(coverSrc, "/") {
			coverSrc = "https://erotorrent.ru" + coverSrc
		}
		imageURLs = append(imageURLs, coverSrc)
	}

	// 5. Ищем фулл-сайз скриншоты (достаем из атрибута href у тега <a>, а не из img)
	doc.Find("div.body_screen ul.screen li a").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			if strings.HasPrefix(href, "/") {
				href = "https://erotorrent.ru" + href
			}
			imageURLs = append(imageURLs, href)
		}
	})

	// 6. Скачиваем картинки (обложка + скриншоты)
	if len(imageURLs) > 0 {
		limit := 10
		if len(imageURLs) < limit {
			limit = len(imageURLs)
		}

		urlsToDownload := imageURLs[:limit]
		localPaths := DownloadImagesAsync(ctx, p.client, urlsToDownload, saveDir, pageURL)

		if len(localPaths) > 0 {
			game.CoverPath = localPaths[0] // Первое изображение всегда идет на обложку
			if len(localPaths) > 1 {
				game.Images = localPaths[1:] // Остальные в галерею
			} else {
				game.Images = []string{}
			}
		}
	}

	return game, nil
}
