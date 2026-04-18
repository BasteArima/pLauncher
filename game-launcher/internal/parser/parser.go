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
	}

	return nil, errors.New("неизвестный источник или сайт не поддерживается")
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
func DownloadImagesAsync(ctx context.Context, client *http.Client, imageURLs []string, saveDir string, referer string) []string {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var localPaths []string

	os.MkdirAll(saveDir, 0755)

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

			fileName := fmt.Sprintf("%d%s", index, ext)
			localPath := filepath.Join(saveDir, fileName)

			// Передаем referer в функцию скачивания
			err := downloadFile(client, urlToDownload, localPath, referer)
			if err != nil {
				fmt.Printf("Ошибка скачивания %s: %v\n", urlToDownload, err)
				return
			}

			mu.Lock()
			localPaths = append(localPaths, localPath)
			mu.Unlock()

		}(i, imgURL)
	}

	wg.Wait()
	return localPaths
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
		return fmt.Errorf("ошибка сервера: %s", resp.Status)
	}

	// ПРОВЕРКА: Если сервер прислал нам HTML вместо картинки - это ошибка защиты
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("сервер заблокировал скачивание (прислал %s вместо картинки)", contentType)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
