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

// F95Parser реализация парсера для f95zone
type F95Parser struct {
	client *http.Client
}

// Parse загружает страницу f95, достает данные и вызывает скачивание скриншотов
func (p *F95Parser) Parse(ctx context.Context, pageURL string, saveDir string) (*models.Game, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса к f95: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("f95 вернул статус %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения HTML: %w", err)
	}

	game := &models.Game{
		Languages: []string{},
	}

	// 1. Парсим заголовок и версию
	rawTitle := doc.Find("h1.p-title-value").Text()
	game.Title = cleanText(rawTitle)

	versionTag := doc.Find("h1.p-title-value span.label").Last().Text()
	if versionTag != "" {
		game.Version = cleanText(versionTag)
		game.Title = strings.Replace(game.Title, game.Version, "", -1)
		game.Title = strings.TrimSpace(game.Title)
	}

	// 2. УМНЫЙ парсинг описания через регулярное выражение
	// Ищем текст внутри главного поста
	htmlContent, err := doc.Find("article.message-body div.bbWrapper").First().Html()
	if err == nil {
		// Регулярка: ищет "Overview", берет текст после него и останавливается на следующем <b> или <div>
		re := regexp.MustCompile(`(?is)<b>\s*Overview:?\s*</b>.*?<br[^>]*>\s*(.*?)\s*(?:<br[^>]*>\s*<b>|<div|$)`)
		matches := re.FindStringSubmatch(htmlContent)

		if len(matches) > 1 {
			rawDesc := matches[1]
			// Очищаем от HTML тегов (например ссылок или спойлеров, которые могли попасть)
			rawDesc = stripHTMLTags(rawDesc)
			// Расшифровываем HTML-сущности (например &amp; -> &)
			game.Description = cleanText(html.UnescapeString(rawDesc))
		} else {
			game.Description = "Описание не найдено."
		}
	}

	// 3. Собираем ссылки на картинки (только из первого, главного поста)
	var coverURL string
	var screenshotURLs []string

	doc.Find("article.message-body").First().Find("img.bbImage").Each(func(i int, s *goquery.Selection) {
		src := ""

		// Логика поиска фулл-сайза
		parent := s.Parent()
		if parent.Is("a") {
			if href, exists := parent.Attr("href"); exists && strings.HasPrefix(href, "http") {
				src = href
			}
		}
		if src == "" {
			if dataSrc, exists := s.Attr("data-src"); exists && strings.HasPrefix(dataSrc, "http") {
				src = dataSrc
			}
		}
		if src == "" {
			if origSrc, exists := s.Attr("src"); exists && strings.HasPrefix(origSrc, "http") {
				src = origSrc
			}
		}

		// Если нашли нормальную ссылку и это не смайлик
		if src != "" && !strings.Contains(src, "smilies") {
			src = strings.ReplaceAll(src, "/thumb/", "/")

			// Если обложки еще нет - первая картинка становится обложкой
			if coverURL == "" {
				coverURL = src
			} else {
				// Остальные идут в скриншоты (с проверкой на дубликаты)
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

	// 4. Скачиваем обложку отдельно
	if coverURL != "" {
		coverPaths := DownloadImagesAsync(ctx, p.client, []string{coverURL}, saveDir, pageURL)
		if len(coverPaths) > 0 {
			game.CoverPath = coverPaths[0]
		}
	}

	// 5. Скачиваем скриншоты (берем до 9 штук)
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

// cleanText убирает лишние пробелы и переносы строк
func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	// Схлопываем множественные пробелы в один
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(s, " "))
}

// stripHTMLTags вырезает все HTML теги из строки
func stripHTMLTags(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}
