package parser

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"strings"

	"game-launcher/internal/models"

	"github.com/PuerkitoBio/goquery"
)

// IslandParser реализация парсера для island-of-pleasure.site
type IslandParser struct {
	client *http.Client
}

// islandDescription вырезает само описание из текста блока ss-fstory-content:
// берёт текст после метки «Описание:» и отрезает технический хвост, который
// начинается с «Год выпуска» (Жанр/Платформа/Версия/changelog не нужны в описании).
func islandDescription(s string) string {
	if i := caseIndex(s, "Описание"); i >= 0 {
		rest := s[i:]
		if c := strings.IndexByte(rest, ':'); c >= 0 {
			s = strings.TrimSpace(rest[c+1:])
		}
	}
	if i := caseIndex(s, "Год выпуска"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
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
		return nil, fmt.Errorf("island-of-pleasure request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("island-of-pleasure returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("HTML read error: %w", err)
	}

	game := &models.Game{
		Languages: []string{},
	}

	// Поле «Название» содержит полный релизный заголовок:
	//   "Название [версия] (год) (Rus/Eng) [движок] ..."
	rawTitle := cleanText(doc.Find("div.finfo-title:contains('Название') + .finfo-text").First().Text())
	if rawTitle == "" {
		rawTitle = cleanText(doc.Find("h1.fstory-h1 b").Text())
	}
	game.Title = cleanReleaseTitle(rawTitle)
	game.Version = versionFromTitle(rawTitle)
	game.Languages = parseReleaseLanguages(rawTitle)
	game.Tags = parseReleaseTags(rawTitle)
	game.Engine = engineFromTokens(game.Tags) // [Ren'Py] и т.п. из заголовка

	// Тело поста («Описание / Год выпуска / … / Версия») — источник автора и версии.
	contentHTML, _ := doc.Find("div.ss-fstory-content").First().Html()

	// Версия из явного поля «Версия:» надёжнее заголовка: тут заголовок вида
	// "Name (AltName) [версия] ..." сбивает versionFromTitle (скобка раньше []).
	if v := versionFromHTML(contentHTML); v != "" {
		game.Version = v
	}

	// Автор — поле «Разработчик/Издатель» в теле; фолбэк — последняя скобка заголовка,
	// если это не движок/платформа/язык/год.
	game.Author = developerFromHTML(contentHTML)
	if game.Author == "" {
		if last := authorFromTitle(rawTitle); last != "" {
			low := strings.ToLower(last)
			if canonicalEngine(last) == "" && !singleTokenTags[low] && langCodes[low] == "" && !isYear(last) {
				game.Author = last
			}
		}
	}

	// Описание — это и есть содержимое блока ss-fstory-content (начинается с «Описание:»)
	descText := islandDescription(cleanText(html.UnescapeString(doc.Find("div.ss-fstory-content").First().Text())))
	if descText != "" {
		game.Description = truncateText(descText, 2000)
	} else {
		game.Description = ""
	}

	if len(game.Languages) == 0 {
		game.Languages = detectLanguages(doc.Find("div.ss-fstory-content").Text())
	}
	if game.Languages == nil {
		game.Languages = []string{}
	}

	// Обложка (афиша)
	var coverURL string
	if coverSrc, exists := doc.Find("div.fstory-poster img").Attr("src"); exists {
		coverURL = absoluteURL(coverSrc, "https://island-of-pleasure.site")
	}

	// Фулл-сайз скриншоты
	var screenshotURLs []string
	doc.Find("ul.xfieldimagegallery.screens li a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		href = absoluteURL(href, "https://island-of-pleasure.site")
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
