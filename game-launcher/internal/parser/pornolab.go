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
		return nil, fmt.Errorf("ошибка запроса к pornolab: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("pornolab вернул статус %d", resp.StatusCode)
	}

	decoder := charmap.Windows1251.NewDecoder()
	reader := decoder.Reader(resp.Body)

	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения HTML: %w", err)
	}

	game := &models.Game{
		Languages: []string{},
	}

	// 1. УМНЫЙ ПАРСИНГ ЗАГОЛОВКА
	// Ищем текст с размером шрифта 24px (стандарт для заголовков раздач на трекере)
	titleSelection := doc.Find("span[style*='font-size: 24px']").First()
	if titleSelection.Length() > 0 {
		game.Title = cleanText(titleSelection.Text())
	} else {
		// Резервный вариант, если стили не прописаны
		game.Title = cleanText(doc.Find("div.post-user-message > span").First().Text())
	}

	htmlContent, err := doc.Find("div.post-user-message").Html()
	if err == nil {
		// Парсим Версию
		reVersion := regexp.MustCompile(`(?i)(?:<span class="post-b">)?\s*Версия\s*(?:</span>)?:\s*(.*?)\s*<br`)
		versionMatch := reVersion.FindStringSubmatch(htmlContent)
		if len(versionMatch) > 1 {
			game.Version = cleanText(stripHTMLTags(versionMatch[1]))
		}

		// Умная регулярка на Описание (устойчивая к вложенным тегам и hr)
		reDesc := regexp.MustCompile(`(?is)Описание.*?(?:</span>)*\s*:\s*(?:</span>)*\s*(.*?)\s*(?:<span class="post-hr">|<hr|<div class="sp-wrap">|$)`)
		descMatch := reDesc.FindStringSubmatch(htmlContent)
		if len(descMatch) > 1 {
			rawDesc := stripHTMLTags(descMatch[1])
			game.Description = cleanText(html.UnescapeString(rawDesc))
		} else {
			game.Description = "Описание не найдено."
		}
	}

	var coverURL string
	var screenshotURLs []string

	// Собираем картинки
	doc.Find("var.postImg, img.postImg").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("title")
		if src == "" {
			src, _ = s.Attr("src")
		}

		if src != "" {
			// АНТИ-МУСОР
			if strings.Contains(src, "static.pornolab.net") || strings.Contains(src, "smilies") {
				return
			}

			// ХАК ДЛЯ FASTPIC
			src = strings.ReplaceAll(src, "/thumb/", "/big/")

			// ХАК ДЛЯ IMGBOX
			if strings.Contains(src, "imgbox.com") && strings.HasSuffix(src, "_t.jpg") {
				src = strings.Replace(src, "thumbs2.imgbox.com", "images2.imgbox.com", 1)
				src = strings.Replace(src, "thumbs.imgbox.com", "images.imgbox.com", 1)
				src = strings.Replace(src, "_t.jpg", "_o.jpg", 1)
			}
			if strings.Contains(src, "imgbox.com") && strings.HasSuffix(src, "_t.png") {
				src = strings.Replace(src, "thumbs2.imgbox.com", "images2.imgbox.com", 1)
				src = strings.Replace(src, "thumbs.imgbox.com", "images.imgbox.com", 1)
				src = strings.Replace(src, "_t.png", "_o.png", 1)
			}

			if strings.HasPrefix(src, "//") {
				src = "https:" + src
			} else if strings.HasPrefix(src, "/") {
				src = "https://pornolab.net" + src
			}

			// Первая картинка — обложка
			if coverURL == "" {
				coverURL = src
			} else {
				// Остальные — скриншоты
				isDup := (src == coverURL)
				for _, u := range screenshotURLs {
					if u == src {
						isDup = true
						break
					}
				}
				if !isDup {
					screenshotURLs = append(screenshotURLs, src)
				}
			}
		}
	})

	// Скачиваем обложку
	if coverURL != "" {
		coverPaths := DownloadImagesAsync(ctx, p.client, []string{coverURL}, saveDir, pageURL)
		if len(coverPaths) > 0 {
			game.CoverPath = coverPaths[0]
		}
	}

	// Скачиваем скриншоты
	if len(screenshotURLs) > 0 {
		limit := 9
		if len(screenshotURLs) < limit {
			limit = len(screenshotURLs)
		}

		urlsToDownload := screenshotURLs[:limit]
		screenshotPaths := DownloadImagesAsync(ctx, p.client, urlsToDownload, saveDir, pageURL)
		game.Images = screenshotPaths
	}

	return game, nil
}
