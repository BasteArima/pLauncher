package parser

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"

	"game-launcher/internal/models"

	"github.com/PuerkitoBio/goquery"
)

// IslandParser реализация парсера для island-of-pleasure.site
type IslandParser struct {
	client *http.Client
}

// Parse скачивает страницу и извлекает метаданные игры
func (p *IslandParser) Parse(ctx context.Context, pageURL string, saveDir string) (*models.Game, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса к island-of-pleasure: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("island-of-pleasure вернул статус %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения HTML: %w", err)
	}

	game := &models.Game{
		Languages: []string{},
	}

	// 1. Парсим заголовок (У них есть очень удобная табличка finfo)
	title := doc.Find("div.finfo-title:contains('Название:')").NextFiltered(".finfo-text").Text()
	if title == "" {
		// Запасной вариант, если таблички нет
		title = doc.Find("h1.fstory-h1 b").Text()
	}
	game.Title = cleanText(title)

	// Берем большой блок с текстом для парсинга версии и описания
	htmlContent, err := doc.Find("div.ss-fstory-content").Html()
	if err == nil {
		// 2. Парсим Версию (ищем текст после <b>Версия:</b>)
		reVer := regexp.MustCompile(`(?i)Версия:?</b>\s*(.*?)\s*<br`)
		verMatch := reVer.FindStringSubmatch(htmlContent)
		if len(verMatch) > 1 {
			game.Version = cleanText(stripHTMLTags(verMatch[1]))
		}

		// 3. Парсим Описание (от <b>Описание:</b> до двойного переноса строки или следующего <b>)
		reDesc := regexp.MustCompile(`(?is)Описание:?</b>\s*<br[^>]*>\s*(.*?)\s*(?:<br[^>]*>\s*<br[^>]*>\s*<b>|<br[^>]*>\s*<b>|<div|$)`)
		descMatch := reDesc.FindStringSubmatch(htmlContent)
		if len(descMatch) > 1 {
			rawDesc := stripHTMLTags(descMatch[1])
			game.Description = cleanText(html.UnescapeString(rawDesc))
		} else {
			game.Description = "Описание не найдено."
		}
	}

	// 4. Ищем обложку (афишу) отдельно!
	var coverURL string
	coverSrc, exists := doc.Find("div.fstory-poster img").Attr("src")
	if exists {
		if strings.HasPrefix(coverSrc, "/") {
			coverSrc = "https://island-of-pleasure.site" + coverSrc
		}
		coverURL = coverSrc
	}

	// 5. Ищем фулл-сайз скриншоты в отдельный массив
	var screenshotURLs []string
	doc.Find("ul.xfieldimagegallery.screens li a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			if strings.HasPrefix(href, "/") {
				href = "https://island-of-pleasure.site" + href
			}

			// Проверка на дубликаты (чтобы скриншот не совпадал с обложкой или другим скрином)
			isDup := false
			if href == coverURL {
				isDup = true
			}
			for _, u := range screenshotURLs {
				if u == href {
					isDup = true
					break
				}
			}
			if !isDup {
				screenshotURLs = append(screenshotURLs, href)
			}
		}
	})

	// 6. СКАЧИВАЕМ ОБЛОЖКУ (Гарантированно записываем её куда нужно)
	if coverURL != "" {
		coverPaths := DownloadImagesAsync(ctx, p.client, []string{coverURL}, saveDir, pageURL)
		if len(coverPaths) > 0 {
			game.CoverPath = coverPaths[0]
		}
	}

	// 7. СКАЧИВАЕМ СКРИНШОТЫ
	if len(screenshotURLs) > 0 {
		limit := 10 // Берем 5 скриншотов
		if len(screenshotURLs) < limit {
			limit = len(screenshotURLs)
		}

		urlsToDownload := screenshotURLs[:limit]
		screenshotPaths := DownloadImagesAsync(ctx, p.client, urlsToDownload, saveDir, pageURL)

		game.Images = screenshotPaths
	}

	return game, nil
}
