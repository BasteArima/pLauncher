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
	//
	// Скриншоты берём ТОЛЬКО из спойлера «Скриншоты/Примеры»: в самом теле поста
	// идут декоративные картинки (постер, img-right в описании, баннер Steam, лого),
	// которые скриншотами не являются. В спойлере лежат миниатюры fastpic/imgbox —
	// приводим их к полноразмерному варианту (/thumb/ -> /big/, imgbox _t -> _o),
	// т.к. ссылка <a href=".../view/...html"> ведёт на HTML-страницу, а не на картинку.
	var coverURL string
	var screenshotURLs []string

	addScreenshot := func(src string) {
		if src == "" || src == coverURL {
			return
		}
		for _, u := range screenshotURLs {
			if u == src {
				return
			}
		}
		screenshotURLs = append(screenshotURLs, src)
	}

	// Спойлер со скриншотами ищем по заголовку (.sp-head).
	var screensSpoiler *goquery.Selection
	post.Find(".sp-wrap").EachWithBreak(func(i int, sp *goquery.Selection) bool {
		head := strings.ToLower(cleanText(sp.Find(".sp-head").First().Text()))
		if strings.Contains(head, "скриншот") || strings.Contains(head, "примеры") ||
			strings.Contains(head, "скринлист") || strings.Contains(head, "screenshot") {
			screensSpoiler = sp
			return false
		}
		return true
	})

	if screensSpoiler != nil {
		// Обложка — первый постер в теле поста (вне любых спойлеров).
		post.Find("var.postImg, img.postImg").EachWithBreak(func(i int, s *goquery.Selection) bool {
			if s.Closest(".sp-wrap").Length() > 0 {
				return true
			}
			if src := normalizePostImg(s); src != "" {
				coverURL = src
				return false
			}
			return true
		})
		// Скриншоты — только из найденного спойлера.
		screensSpoiler.Find("var.postImg, img.postImg").Each(func(i int, s *goquery.Selection) {
			addScreenshot(normalizePostImg(s))
		})
		// Если постера вне спойлеров не оказалось — берём первый скриншот под обложку.
		if coverURL == "" && len(screenshotURLs) > 0 {
			coverURL = screenshotURLs[0]
			screenshotURLs = screenshotURLs[1:]
		}
	} else {
		// Фолбэк (нет спойлера со скриншотами): скриншотами считаем только миниатюры,
		// обёрнутые в ссылку на хостинг картинок (fastpic/imgbox/…) — так из тела поста
		// не утекают декорации (постер, img-right, баннеры). Первая «свободная» (без
		// такой ссылки) картинка идёт под обложку.
		post.Find("var.postImg, img.postImg").Each(func(i int, s *goquery.Selection) {
			src := normalizePostImg(s)
			if src == "" {
				return
			}
			href, _ := s.Closest("a").Attr("href")
			if isImageHostLink(href) {
				addScreenshot(src)
			} else if coverURL == "" {
				coverURL = src
			}
		})

		// Совсем ничего «ссылочного» не нашли — откатываемся к старому поведению
		// (первая картинка — обложка, остальные — скриншоты), чтобы не потерять
		// галерею на постах с другой разметкой.
		if len(screenshotURLs) == 0 {
			coverURL = ""
			post.Find("var.postImg, img.postImg").Each(func(i int, s *goquery.Selection) {
				src := normalizePostImg(s)
				if src == "" {
					return
				}
				if coverURL == "" {
					coverURL = src
					return
				}
				addScreenshot(src)
			})
		}
	}

	// Подстраховка: обложки нет, но скриншоты есть — первый под обложку.
	if coverURL == "" && len(screenshotURLs) > 0 {
		coverURL = screenshotURLs[0]
		screenshotURLs = screenshotURLs[1:]
	}

	downloadInto(ctx, p.client, game, coverURL, screenshotURLs, saveDir, pageURL)
	return game, nil
}

// normalizePostImg достаёт реальный URL картинки из var/img.postImg и приводит его
// к полноразмерному варианту. Возвращает "" для баннеров трекера и смайлов.
func normalizePostImg(s *goquery.Selection) string {
	src, _ := s.Attr("title")
	if src == "" {
		src, _ = s.Attr("src")
	}
	if src == "" {
		return ""
	}

	// АНТИ-МУСОР: баннеры трекера и смайлы
	if strings.Contains(src, "static.pornolab.net") || strings.Contains(src, "smilies") {
		return ""
	}

	// ХАК ДЛЯ FASTPIC: миниатюра /thumb/ -> полноразмер /big/.
	// У миниатюры расширение всегда .jpeg, а файл в /big/ хранится с ОРИГИНАЛЬНЫМ
	// расширением (.jpg/.png) — иначе /big/...jpeg отдаёт 404. Берём настоящее
	// расширение из ссылки на страницу просмотра (.../HASH.jpg.html). Если ссылки
	// нет — оставляем рабочую миниатюру, чтобы не качать заведомый 404.
	if strings.Contains(src, "fastpic") && strings.Contains(src, "/thumb/") {
		if ext := fastpicBigExt(s); ext != "" {
			src = replaceURLExt(strings.Replace(src, "/thumb/", "/big/", 1), ext)
		}
	} else {
		src = strings.ReplaceAll(src, "/thumb/", "/big/")
	}

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
	return src
}

// fastpicBigExt достаёт оригинальное расширение картинки fastpic (".jpg"/".png")
// из ссылки на страницу просмотра вида .../HASH.jpg.html у ближайшего <a>.
// У миниатюры расширение всегда .jpeg, поэтому полагаться на неё нельзя.
func fastpicBigExt(s *goquery.Selection) string {
	href, ok := s.Closest("a").Attr("href")
	if !ok {
		return ""
	}
	href = strings.TrimSuffix(strings.TrimSuffix(href, ".html"), ".HTML")
	dot := strings.LastIndexByte(href, '.')
	slash := strings.LastIndexByte(href, '/')
	if dot <= slash {
		return ""
	}
	ext := href[dot:]
	if len(ext) > 5 || strings.ContainsAny(ext, "/?#") {
		return ""
	}
	return ext
}

// replaceURLExt заменяет расширение в последнем сегменте URL (точки в домене не трогает).
func replaceURLExt(rawURL, ext string) string {
	slash := strings.LastIndexByte(rawURL, '/')
	dot := strings.LastIndexByte(rawURL, '.')
	if dot <= slash {
		return rawURL
	}
	return rawURL[:dot] + ext
}

// isImageHostLink сообщает, ведёт ли ссылка на страницу просмотра картинки на
// известном хостинге. Настоящие скриншоты в постах обёрнуты в такую ссылку,
// а декоративные картинки (постер, баннеры) — нет.
func isImageHostLink(href string) bool {
	if href == "" {
		return false
	}
	for _, h := range []string{"fastpic", "imgbox", "postimg", "imageban", "pixhost", "imgur", "ibb.co", "radikal"} {
		if strings.Contains(href, h) {
			return true
		}
	}
	return false
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
