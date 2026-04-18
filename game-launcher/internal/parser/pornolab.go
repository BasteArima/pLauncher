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

	var imageURLs []string

	// Собираем картинки
	doc.Find("var.postImg, img.postImg").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("title")
		if src == "" {
			src, _ = s.Attr("src")
		}

		if src != "" {
			// АНТИ-МУСОР: Игнорируем иконки флагов, движков и плашки самого порнолаба
			if strings.Contains(src, "static.pornolab.net") || strings.Contains(src, "smilies") {
				return // Пропускаем эту итерацию
			}

			// ХАК ДЛЯ FASTPIC
			src = strings.ReplaceAll(src, "/thumb/", "/big/")

			// ХАК ДЛЯ IMGBOX (превращаем миниатюры _t в оригиналы _o)
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

			isDup := false
			for _, u := range imageURLs {
				if u == src {
					isDup = true
					break
				}
			}
			if !isDup {
				imageURLs = append(imageURLs, src)
			}
		}
	})

	if len(imageURLs) > 0 {
		limit := 10
		if len(imageURLs) < limit {
			limit = len(imageURLs)
		}

		urlsToDownload := imageURLs[:limit]
		localPaths := DownloadImagesAsync(ctx, p.client, urlsToDownload, saveDir, pageURL)

		if len(localPaths) > 0 {
			game.CoverPath = localPaths[0]
			if len(localPaths) > 1 {
				game.Images = localPaths[1:]
			} else {
				game.Images = []string{}
			}
		}
	}

	return game, nil
}
