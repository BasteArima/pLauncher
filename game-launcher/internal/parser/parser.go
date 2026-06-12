package parser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"game-launcher/internal/models"
)

// languageAliases сопоставляет канонический язык со списком его написаний,
// которые встречаются в описаниях раздач (RU/EN формы).
var languageAliases = map[string][]string{
	"Русский":   {"русский", "russian", " rus ", "(rus)", "[rus]"},
	"English":   {"английский", "english", " eng ", "(eng)", "[eng]"},
	"日本語":       {"японский", "japanese", " jpn "},
	"中文":        {"китайский", "chinese"},
	"Español":   {"испанский", "spanish"},
	"Deutsch":   {"немецкий", "german"},
	"Français":  {"французский", "french"},
	"Português": {"португальский", "portuguese"},
	"한국어":       {"корейский", "korean"},
	"Italiano":  {"итальянский", "italian"},
}

// detectLanguages пытается определить языки игры по тексту страницы.
// Чисто эвристический разбор: ищет известные названия языков. Возвращает
// канонические подписи без дубликатов (порядок стабилен).
func detectLanguages(text string) []string {
	lower := " " + strings.ToLower(text) + " "

	// Стабильный порядок вывода
	order := []string{"Русский", "English", "日本語", "中文", "Español",
		"Deutsch", "Français", "Português", "한국어", "Italiano"}

	var found []string
	for _, canon := range order {
		for _, alias := range languageAliases[canon] {
			if strings.Contains(lower, alias) {
				found = append(found, canon)
				break
			}
		}
	}
	return found
}

// Source описывает поддерживаемый сайт-источник (для показа списка в UI).
type Source struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

// SupportedSources — единый список парсеров. Расширяется здесь же.
func SupportedSources() []Source {
	return []Source{
		{Name: "F95zone", Domain: "f95zone.to"},
		{Name: "Pornolab", Domain: "pornolab.net"},
		{Name: "Erotorrent", Domain: "erotorrent.ru"},
		{Name: "Island of Pleasure", Domain: "island-of-pleasure.site"},
		{Name: "Steam", Domain: "store.steampowered.com"},
	}
}

// SiteParser — единый интерфейс для всех сайтов-источников
type SiteParser interface {
	Parse(ctx context.Context, pageURL string, saveDir string) (*models.Game, error)
}

// GetParser фабрика, которая выдает нужный парсер в зависимости от URL
func GetParser(targetURL string, proxyURL string) (SiteParser, error) {
	client := newHTTPClient(proxyURL)

	if strings.Contains(targetURL, "f95zone") {
		return &F95Parser{client: client}, nil
	} else if strings.Contains(targetURL, "erotorrent.ru") {
		return &ErotorrentParser{client: client}, nil // ДОБАВИЛИ ЭТУ СТРОКУ
	} else if strings.Contains(targetURL, "pornlab.net") || strings.Contains(targetURL, "pornolab.net") {
		return &PornlabParser{client: client}, nil
	} else if strings.Contains(targetURL, "island-of-pleasure.site") {
		return &IslandParser{client: client}, nil
	} else if strings.Contains(targetURL, "store.steampowered.com") {
		return &SteamParser{client: client}, nil
	}

	return nil, errors.New("unknown source or unsupported site")
}

// newHTTPClient создает клиента с таймаутами и (опционально) прокси
func newHTTPClient(proxyStr string) *http.Client {
	transport := &http.Transport{}
	if proxyStr != "" {
		if pURL, err := url.Parse(proxyStr); err == nil {
			transport.Proxy = http.ProxyURL(pURL)
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second, // Защита от зависших соединений
	}
}

// DownloadImagesAsync параллельно скачивает картинки, используя пулл горутин.
// Возвращает слайс путей к локально сохраненным файлам.
// DownloadImagesAsync параллельно скачивает картинки. Добавлен параметр referer!
func DownloadImagesAsync(ctx context.Context, client *http.Client, imageURLs []string, saveDir string, referer string, namePrefix string) []string {
	var wg sync.WaitGroup

	os.MkdirAll(saveDir, 0755)

	// Пишем результат каждой горутины в свою ячейку по индексу — порядок картинок
	// сохраняется независимо от того, какая закачка завершилась первой.
	results := make([]string, len(imageURLs))

	for i, imgURL := range imageURLs {
		wg.Add(1)

		go func(index int, urlToDownload string) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			ext := filepath.Ext(urlToDownload)
			if ext == "" {
				ext = ".jpg"
			}
			ext = strings.Split(ext, "?")[0]

			fileName := fmt.Sprintf("%s%d%s", namePrefix, index, ext)
			localPath := filepath.Join(saveDir, fileName)

			// Передаем referer в функцию скачивания
			err := downloadFile(client, urlToDownload, localPath, referer)
			if err != nil {
				fmt.Printf("Ошибка скачивания %s: %v\n", urlToDownload, err)
				return
			}

			results[index] = localPath
		}(i, imgURL)
	}

	wg.Wait()

	// Убираем пустые ячейки (неудавшиеся закачки), сохраняя исходный порядок
	localPaths := make([]string, 0, len(results))
	for _, p := range results {
		if p != "" {
			localPaths = append(localPaths, p)
		}
	}
	return localPaths
}

// downloadInto скачивает обложку и до 9 скриншотов в saveDir и проставляет пути
// в game. Обложка и скриншоты получают разные префиксы имён, чтобы не затирать
// друг друга (раньше cover и первый скрин оба сохранялись как 0.jpg).
func downloadInto(ctx context.Context, client *http.Client, game *models.Game, coverURL string, screenshotURLs []string, saveDir, referer string) {
	if coverURL != "" {
		if cp := DownloadImagesAsync(ctx, client, []string{coverURL}, saveDir, referer, "cover_"); len(cp) > 0 {
			game.CoverPath = cp[0]
		}
	}
	if len(screenshotURLs) > 0 {
		limit := 9
		if len(screenshotURLs) < limit {
			limit = len(screenshotURLs)
		}
		game.Images = DownloadImagesAsync(ctx, client, screenshotURLs[:limit], saveDir, referer, "shot_")
	}
}

// absoluteURL дополняет относительные ссылки доменом сайта.
func absoluteURL(src, domain string) string {
	if strings.HasPrefix(src, "//") {
		return "https:" + src
	}
	if strings.HasPrefix(src, "/") {
		return domain + src
	}
	return src
}

// caseIndex — регистронезависимый поиск подстроки, возвращает байтовый индекс
// в исходной строке (корректен для ASCII/кириллицы, чего достаточно для меток).
func caseIndex(s, sub string) int {
	return strings.Index(strings.ToLower(s), strings.ToLower(sub))
}

// truncateText обрезает текст до max рун по границе слова, добавляя многоточие.
func truncateText(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	cut := string(r[:max])
	if i := strings.LastIndexByte(cut, ' '); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut) + "…"
}

// downloadFile скачивает один файл на диск (теперь с Referer)
func downloadFile(client *http.Client, urlToDownload, filepath, referer string) error {
	req, err := http.NewRequest("GET", urlToDownload, nil)
	if err != nil {
		return err
	}

	// Максимально подробные заголовки реального браузера
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "image")
	req.Header.Set("Sec-Fetch-Mode", "no-cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")

	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("server error: %s", resp.Status)
	}

	// ПРОВЕРКА: Если сервер прислал нам HTML вместо картинки - это ошибка защиты
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("server blocked the download (returned %s instead of an image)", contentType)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
