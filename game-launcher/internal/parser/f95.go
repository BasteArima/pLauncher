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
	var imageURLs []string
	doc.Find("article.message-body").First().Find("img.bbImage").Each(func(i int, s *goquery.Selection) {
		src := ""

		// Логика поиска фулл-сайза:
		// 1. Сначала проверяем, обернута ли картинка в ссылку <a> (там всегда лежит оригинал)
		parent := s.Parent()
		if parent.Is("a") {
			if href, exists := parent.Attr("href"); exists && strings.HasPrefix(href, "http") {
				src = href
			}
		}

		// 2. Если ссылки нет, ищем в data-src (оригинал для ленивой загрузки)
		if src == "" {
			if dataSrc, exists := s.Attr("data-src"); exists && strings.HasPrefix(dataSrc, "http") {
				src = dataSrc
			}
		}

		// 3. Фоллбэк: берем обычный src (если картинка вставлена напрямую без миниатюр)
		if src == "" {
			if origSrc, exists := s.Attr("src"); exists && strings.HasPrefix(origSrc, "http") {
				src = origSrc
			}
		}

		// Если нашли нормальную ссылку и это не смайлик
		if src != "" && !strings.Contains(src, "smilies") {
			// Убираем возможный мусор в ссылке, который иногда оставляет xenforo
			src = strings.ReplaceAll(src, "/thumb/", "/")

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

	// 4. Скачиваем картинки (1 обложка + до 5 скриншотов для галереи)
	if len(imageURLs) > 0 {
		limit := 10
		if len(imageURLs) < limit {
			limit = len(imageURLs)
		}

		urlsToDownload := imageURLs[:limit]
		localPaths := DownloadImagesAsync(ctx, p.client, urlsToDownload, saveDir, pageURL)

		if len(localPaths) > 0 {
			// Гарантированно первая картинка из поста становится обложкой
			game.CoverPath = localPaths[0]

			// Остальные уходят в скриншоты
			if len(localPaths) > 1 {
				game.Images = localPaths[1:]
			} else {
				game.Images = []string{}
			}
		}
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
