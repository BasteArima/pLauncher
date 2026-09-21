package parser

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// Защищённые картинки fastpic (имя файла начинается с "_"): прямой запрос
// /big/... отвечает 302 на HTML-страницу просмотра, поэтому downloadFile видит
// text/html. Оригинал доступен только по подписанной ссылке ?md5=…&expires=…,
// которая есть на странице fullview. Её и достаём.

var fastpicSignedRe = regexp.MustCompile(`https?://i\d+\.fastpic\.org/big/[^'"\s<>]+\?md5=[^'"\s<>&]+&(?:amp;)?expires=\d+`)

// fastpicFullviewURL строит адрес страницы fullview по прямой ссылке на картинку:
// https://i128.fastpic.org/big/2026/0913/3d/_hash.jpg →
// https://fastpic.org/fullview/128/2026/0913/_hash.jpg.html
func fastpicFullviewURL(bigURL string) (string, error) {
	u, err := url.Parse(bigURL)
	if err != nil {
		return "", err
	}
	host := strings.TrimSuffix(u.Hostname(), ".fastpic.org")
	server := strings.TrimPrefix(host, "i")
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// big / год / ммдд / подпапка / файл
	if host == u.Hostname() || server == host || len(parts) != 5 || parts[0] != "big" {
		return "", fmt.Errorf("not a fastpic image url: %s", bigURL)
	}
	return fmt.Sprintf("https://fastpic.org/fullview/%s/%s/%s/%s.html", server, parts[1], parts[2], parts[4]), nil
}

// fastpicSignedFromHTML ищет на странице подписанную ссылку именно на этот файл.
func fastpicSignedFromHTML(page, fileName string) string {
	for _, m := range fastpicSignedRe.FindAllString(page, -1) {
		if strings.Contains(m, "/"+fileName+"?") {
			return html.UnescapeString(m)
		}
	}
	return ""
}

// resolveFastpicSigned получает подписанную ссылку на оригинал защищённой картинки.
func resolveFastpicSigned(client *http.Client, bigURL string) (string, error) {
	fullview, err := fastpicFullviewURL(bigURL)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("GET", fullview, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fastpic fullview: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	u, _ := url.Parse(bigURL)
	if signed := fastpicSignedFromHTML(string(body), path.Base(u.Path)); signed != "" {
		return signed, nil
	}
	return "", fmt.Errorf("fastpic: signed link not found on %s", fullview)
}
