package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"

	"game-launcher/internal/models"
)

// SteamParser получает данные через официальный appdetails API Steam (без скрейпинга).
type SteamParser struct {
	client *http.Client
}

var steamAppID = regexp.MustCompile(`/app/(\d+)`)

// steamResponse описывает нужные поля ответа store API.
type steamResponse map[string]struct {
	Success bool `json:"success"`
	Data    struct {
		Name             string `json:"name"`
		ShortDescription string `json:"short_description"`
		HeaderImage      string `json:"header_image"`
		Genres           []struct {
			Description string `json:"description"`
		} `json:"genres"`
		Screenshots []struct {
			PathFull string `json:"path_full"`
		} `json:"screenshots"`
	} `json:"data"`
}

func (p *SteamParser) Parse(ctx context.Context, pageURL string, saveDir string) (*models.Game, error) {
	m := steamAppID.FindStringSubmatch(pageURL)
	if len(m) < 2 {
		return nil, fmt.Errorf("couldn't determine the Steam app ID from the link")
	}
	appID := m[1]

	apiURL := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%s&l=russian", appID)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Steam request error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Steam returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed steamResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("error parsing Steam response: %w", err)
	}
	entry, ok := parsed[appID]
	if !ok || !entry.Success {
		return nil, fmt.Errorf("Steam returned no data for app %s", appID)
	}
	d := entry.Data

	game := &models.Game{Languages: []string{}}
	game.Title = cleanText(d.Name)
	game.Description = cleanText(html.UnescapeString(stripHTMLTags(d.ShortDescription)))
	if game.Description == "" {
		game.Description = ""
	}
	for _, g := range d.Genres {
		if g.Description != "" {
			game.Tags = append(game.Tags, g.Description)
		}
	}

	// Обложка: вертикальный «капсюль» библиотеки (600x900); если его нет — header_image.
	coverURL := fmt.Sprintf("https://steamcdn-a.akamaihd.net/steam/apps/%s/library_600x900.jpg", appID)
	if !imageExists(p.client, coverURL) {
		coverURL = d.HeaderImage
	}

	var screenshotURLs []string
	for _, s := range d.Screenshots {
		if s.PathFull != "" {
			screenshotURLs = append(screenshotURLs, s.PathFull)
		}
	}

	downloadInto(ctx, p.client, game, coverURL, screenshotURLs, saveDir, pageURL)
	return game, nil
}

// imageExists проверяет, что по ссылке реально лежит картинка (HEAD-запрос).
func imageExists(client *http.Client, url string) bool {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}
