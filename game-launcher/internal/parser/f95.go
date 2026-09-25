package parser

import (
	"context"
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
		return nil, requestErr("F95zone", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, httpStatusErr("F95zone", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, readErr(err)
	}

	game := &models.Game{
		Languages: []string{},
	}

	// 1. Заголовок и версия.
	// H1 содержит метки-префиксы (VN, Ren'Py, Completed и т.п.) отдельными
	// ссылками <a>, а уже текстом идёт "Название [версия] [разработчик]".
	// Убираем метки и разбираем оставшийся текст готовыми хелперами.
	titleSel := doc.Find("h1.p-title-value").First()
	clone := titleSel.Clone()
	clone.Find("a, .label-append, .labelLink").Remove()
	rawTitle := cleanText(clone.Text()) // "College of Mysteria [v0.13] [HappySteveGames]"

	game.Title = cleanReleaseTitle(rawTitle)
	if game.Title == "" {
		game.Title = rawTitle
	}
	game.Version = versionFromTitle(rawTitle)  // "[v0.13]" -> "v0.13"
	game.Author = authorFromTitle(rawTitle)    // последняя скобка "[HappySteveGames]"

	// Движок — из меток-префиксов h1 (VN, Ren'Py, Completed): берём первую,
	// что распознаётся как движок.
	var labelTokens []string
	titleSel.Find(".labelLink").Each(func(i int, s *goquery.Selection) {
		labelTokens = append(labelTokens, cleanText(s.Text()))
	})
	game.Engine = engineFromTokens(labelTokens)

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
			game.Description = ""
		}
	}

	// 2.1 Фолбэк версии: поле "Version:" в теле поста ("<b>Version</b>: 0.13").
	if game.Version == "" && htmlContent != "" {
		if m := regexp.MustCompile(`(?is)<b>\s*Version\s*:?\s*</b>\s*:?\s*([^<\n]+)`).FindStringSubmatch(htmlContent); len(m) > 1 {
			game.Version = cleanText(html.UnescapeString(m[1]))
		}
	}

	// 2.2 Фолбэк автора: поле "Developer:" в теле поста
	// ("<b>Developer</b>: <a>HappySteve</a> - <a>Patreon</a>"). Берём часть до " - ".
	if game.Author == "" && htmlContent != "" {
		if m := regexp.MustCompile(`(?is)<b>\s*Developer\s*/?\s*Publisher?\s*:?\s*</b>\s*:?\s*(.*?)<br`).FindStringSubmatch(htmlContent); len(m) > 1 {
			dev := cleanText(html.UnescapeString(stripHTMLTags(m[1])))
			if i := strings.Index(dev, " - "); i >= 0 {
				dev = dev[:i]
			}
			game.Author = cleanText(dev)
		}
	}

	// 2.5 Определяем языки по тексту главного поста
	game.Languages = detectLanguages(doc.Find("article.message-body div.bbWrapper").First().Text())

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

	downloadInto(ctx, p.client, game, coverURL, screenshotURLs, saveDir, pageURL)
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
