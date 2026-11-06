package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"game-launcher/internal/apperr"
	"game-launcher/internal/models"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

var vndbAPIURL = "https://api.vndb.org/kana/vn"
var vndbIDPattern = regexp.MustCompile(`^v[1-9][0-9]*$`)
var vndbBBCode = regexp.MustCompile(`(?i)\[(/?)(url|spoiler|quote|code|raw|b|i|u|s|br)(?:[= ][^\]]*)?\]`)

type VNDBParser struct{ client *http.Client }
type vndbEntry struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Released    string   `json:"released"`
	Languages   []string `json:"languages"`
	Developers  []struct {
		Name string `json:"name"`
	} `json:"developers"`
	Image *struct {
		URL       string `json:"url"`
		Thumbnail string `json:"thumbnail"`
	} `json:"image"`
	Screenshots []struct {
		URL string `json:"url"`
	} `json:"screenshots"`
	Tags []struct {
		Name     string  `json:"name"`
		Rating   float64 `json:"rating"`
		Spoiler  float64 `json:"spoiler"`
		Category string  `json:"category"`
	} `json:"tags"`
}

func vndbID(pageURL string) string {
	u, err := url.Parse(pageURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Hostname(), "vndb.org") {
		return ""
	}
	id := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/")
	if !vndbIDPattern.MatchString(id) {
		return ""
	}
	return id
}

func queryVNDB(ctx context.Context, client *http.Client, filter, query, fields, order string, limit int) ([]vndbEntry, error) {
	body, _ := json.Marshal(struct {
		Filters []string `json:"filters"`
		Fields  string   `json:"fields"`
		Sort    string   `json:"sort"`
		Results int      `json:"results"`
	}{[]string{filter, "=", query}, fields, order, limit})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, vndbAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, requestErr("VNDB", err)
	}
	req.Header.Set("Content-Type", "application/json")
	var result struct {
		Results []vndbEntry `json:"results"`
	}
	if err = requestJSON(client, req, "VNDB", &result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

func (p *VNDBParser) Parse(ctx context.Context, pageURL, saveDir string) (*models.Game, error) {
	id := vndbID(pageURL)
	if id == "" {
		return nil, apperr.New("vndb.id", nil, "couldn't determine the VNDB visual novel ID from the link")
	}
	rows, err := queryVNDB(ctx, p.client, "id", id, "title,description,released,languages,developers.name,image.url,screenshots.url,tags.name,tags.rating,tags.spoiler,tags.category", "id", 1)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 || rows[0].ID != id {
		return nil, apperr.New("vndb.nodata", apperr.P{"id": id}, "VNDB returned no data for %s", id)
	}
	r := rows[0]
	g := &models.Game{Title: cleanText(r.Title), Description: stripVNDBBBCode(r.Description), Languages: []string{}, Tags: []string{}}
	var authors []string
	for _, d := range r.Developers {
		if name := cleanText(d.Name); name != "" {
			authors = append(authors, name)
		}
	}
	g.Author = strings.Join(authors, ", ")
	seen := map[string]bool{}
	for _, code := range r.Languages {
		name := vndbLanguage(code)
		if name != "" && !seen[name] {
			seen[name] = true
			g.Languages = append(g.Languages, name)
		}
	}
	sort.SliceStable(r.Tags, func(i, j int) bool { return r.Tags[i].Rating > r.Tags[j].Rating })
	seen = map[string]bool{}
	for _, tag := range r.Tags {
		name := cleanText(tag.Name)
		if tag.Spoiler != 0 || name == "" || seen[name] {
			continue
		}
		seen[name] = true
		g.Tags = append(g.Tags, name)
		if len(g.Tags) == 15 {
			break
		}
	}
	cover := ""
	if r.Image != nil {
		cover = r.Image.URL
	}
	var shots []string
	for _, s := range r.Screenshots {
		if s.URL != "" {
			shots = append(shots, s.URL)
		}
	}
	downloadInto(ctx, p.client, g, cover, shots, saveDir, pageURL)
	return g, nil
}

func vndbLanguage(code string) string {
	known := map[string]string{"ru": "Русский", "en": "English", "ja": "日本語", "zh": "中文", "zh-Hans": "中文", "zh-Hant": "中文", "es": "Español", "de": "Deutsch", "fr": "Français", "pt": "Português", "pt-br": "Português", "ko": "한국어", "it": "Italiano", "tr": "Türkçe"}
	if name := known[code]; name != "" {
		return name
	}
	return display.Self.Name(language.Make(code))
}

// Спойлеры удаляем вместе с содержимым, включая вложенные/незакрытые блоки.
func stripVNDBBBCode(s string) string {
	var out strings.Builder
	depth, last := 0, 0
	for _, m := range vndbBBCode.FindAllStringSubmatchIndex(s, -1) {
		if depth == 0 {
			out.WriteString(s[last:m[0]])
		}
		tag := strings.ToLower(s[m[4]:m[5]])
		closing := s[m[2]:m[3]] == "/"
		if tag == "spoiler" {
			if !closing {
				depth++
			} else if depth > 0 {
				depth--
			}
		} else if tag == "br" && depth == 0 {
			out.WriteByte('\n')
		}
		last = m[1]
	}
	if depth == 0 {
		out.WriteString(s[last:])
	}
	return strings.TrimSpace(html.UnescapeString(out.String()))
}
