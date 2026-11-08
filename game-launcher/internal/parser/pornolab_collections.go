package parser

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"game-launcher/internal/apperr"
	"game-launcher/internal/models"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

type pornolabContextKey struct{}

// Options keep legacy update checks distinct from metadata application.
type PornolabOptions struct {
	Cache          *PornolabCache
	Fresh          bool
	LegacyUpdate   bool
	SelectedTitle  string
	SavedSelection bool
	Snapshot       string
}

func WithPornolab(ctx context.Context, opts PornolabOptions) context.Context {
	return context.WithValue(ctx, pornolabContextKey{}, opts)
}
func PornolabContextOptions(ctx context.Context) PornolabOptions {
	opts, _ := ctx.Value(pornolabContextKey{}).(PornolabOptions)
	return opts
}

func collectionChoiceErr() error {
	return apperr.New("parse.collection_choice_required", nil, "choose a game from this release")
}
func collectionMissingErr() error {
	return apperr.New("parse.collection_game_missing", nil, "game not found in this release; choose it again")
}
func collectionChangedErr() error {
	return apperr.New("parse.collection_changed", nil, "the games in this release have changed; choose again")
}

// PornolabURL returns the request URL and the one-based selection. Other anchors are not selections.
func PornolabURL(raw string) (string, int, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", 0, collectionMissingErr()
	}
	index := 0
	for _, part := range strings.Split(u.Fragment, "&") {
		key, value, _ := strings.Cut(part, "=")
		if key != "game" {
			continue
		}
		n, e := strconv.Atoi(value)
		if e != nil || n < 1 || index != 0 || strconv.Itoa(n) != value {
			return "", 0, collectionMissingErr()
		}
		index = n
	}
	u.Fragment = ""
	u.RawFragment = ""
	return u.String(), index, nil
}
func pornolabSelectedURL(base string, index int) string { return base + "#game=" + strconv.Itoa(index) }

type PornolabCandidate struct {
	SearchHit
	Index int `json:"index"`
	// Empty/duplicate headings are rendered with collection.game_number and Index by the UI.
	FallbackTitle bool   `json:"fallback_title"`
	Description   string `json:"description"`
}
type PornolabCollection struct {
	Candidates []PornolabCandidate `json:"candidates"`
	Snapshot   string              `json:"snapshot"`
	Collection bool                `json:"collection"`
}
type pornolabSection struct {
	game models.Game
	year string
}

func (p *PornlabParser) InspectCollection(ctx context.Context, raw, query string, hint SearchHint) (PornolabCollection, error) {
	result := PornolabCollection{Candidates: []PornolabCandidate{}}
	opts, _ := ctx.Value(pornolabContextKey{}).(PornolabOptions)
	doc, err := p.document(ctx, raw, opts.Cache, opts.Fresh)
	if err != nil {
		return result, err
	}
	base, _, err := PornolabURL(raw)
	if err != nil {
		return result, err
	}
	sections := pornolabSections(doc)
	result.Collection = len(sections) > 1
	result.Snapshot = sectionSnapshot(sections)
	if opts.Cache != nil {
		result.Snapshot = opts.Cache.stamp(result.Snapshot)
	}
	counts := map[string]int{}
	for _, s := range sections {
		counts[selectionTitle(s.game.Title)]++
	}
	hits := make([]SearchHit, 0, len(sections))
	byURL := map[string]PornolabCandidate{}
	for i, s := range sections {
		hit := SearchHit{Source: "Pornolab", URL: pornolabSelectedURL(base, i+1), Title: s.game.Title, Author: s.game.Author, Version: s.game.Version, Year: s.year, Thumb: s.game.CoverPath, Engine: s.game.Engine}
		fallback := selectionTitle(hit.Title) == "" || counts[selectionTitle(hit.Title)] > 1
		byURL[hit.URL] = PornolabCandidate{SearchHit: hit, Index: i + 1, FallbackTitle: fallback, Description: truncateText(s.game.Description, 160)}
		if fallback {
			hit.Title = ""
		} // Ambiguous labels are never folder-match recommendations.
		hits = append(hits, hit)
	}
	rankSearchHits(hits, query, hint)
	for _, hit := range hits {
		c := byURL[hit.URL]
		c.Score = hit.Score
		c.Best = hit.Best
		result.Candidates = append(result.Candidates, c)
	}
	return result, nil
}

// Saved identity uses conservative equality; ranking's article removal is too loose here.
func selectionTitle(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
func resolvePornolabSelection(sections []pornolabSection, index int, title string, saved bool) (int, error) {
	if saved {
		wanted := selectionTitle(title)
		found := 0
		if wanted == "" {
			return 0, collectionMissingErr()
		}
		for i, s := range sections {
			if selectionTitle(s.game.Title) == wanted {
				if found != 0 {
					return 0, collectionMissingErr()
				}
				found = i + 1
			}
		}
		if found == 0 {
			return 0, collectionMissingErr()
		}
		return found, nil
	}
	if index < 1 || index > len(sections) {
		return 0, collectionMissingErr()
	}
	return index, nil
}

// A small, opaque-to-the-UI list of identities survives cache eviction without retaining pages.
// Order changes resolve by unique title; versions and media changes do not reopen the chooser.
type pornolabSnapshot struct {
	Titles []string `json:"titles"`
	Digest string   `json:"digest"`
}

func snapshotState(sections []pornolabSection) pornolabSnapshot {
	state := pornolabSnapshot{Titles: make([]string, len(sections))}
	games := make([]models.Game, len(sections))
	for i, s := range sections {
		state.Titles[i] = selectionTitle(s.game.Title)
		games[i] = s.game
	}
	b, _ := json.Marshal(games)
	sum := sha256.Sum256(b)
	state.Digest = hex.EncodeToString(sum[:])
	return state
}
func sectionSnapshot(sections []pornolabSection) string {
	b, _ := json.Marshal(snapshotState(sections))
	return base64.RawURLEncoding.EncodeToString(b)
}
func resolveSnapshot(snapshot string, sections []pornolabSection, index int) (int, error) {
	if len(snapshot) > 1<<20 {
		return 0, collectionChangedErr()
	}
	b, err := base64.RawURLEncoding.DecodeString(snapshot)
	var old pornolabSnapshot
	if err != nil || json.Unmarshal(b, &old) != nil || index < 1 || index > len(old.Titles) {
		return 0, collectionChangedErr()
	}
	now := snapshotState(sections)
	if len(old.Titles) != len(now.Titles) {
		return 0, collectionChangedErr()
	}
	a, c := append([]string{}, old.Titles...), append([]string{}, now.Titles...)
	sort.Strings(a)
	sort.Strings(c)
	for i := range a {
		if a[i] != c[i] {
			return 0, collectionChangedErr()
		}
	}
	if old.Digest == now.Digest {
		return index, nil
	}
	// On changed data only a unique nonempty title can recover the reviewed choice.
	wanted := old.Titles[index-1]
	found := 0
	if wanted == "" {
		return 0, collectionMissingErr()
	}
	for i, key := range now.Titles {
		if key == wanted {
			if found != 0 {
				return 0, collectionMissingErr()
			}
			found = i + 1
		}
	}
	if found == 0 {
		return 0, collectionMissingErr()
	}
	return found, nil
}

func class(n *html.Node, c string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, v := range strings.Fields(a.Val) {
				if v == c {
					return true
				}
			}
		}
	}
	return false
}
func nodeSelection(n *html.Node) *goquery.Selection { return goquery.NewDocumentFromNode(n).Selection }
func spoilerName(s *goquery.Selection) string {
	// Direct children only: a game wrapper must not inherit its nested screenshot label.
	name := s.ChildrenFiltered(".sp-head, .sp-title").First().Text()
	body := s.ChildrenFiltered(".sp-body").First()
	if name == "" {
		name, _ = body.Attr("title")
	}
	if name == "" {
		name = body.ChildrenFiltered(".sp-title").First().Text()
	}
	return strings.ToLower(cleanText(name))
}
func screenshotSpoiler(name string) bool {
	for _, v := range []string{"скриншот", "примеры", "скринлист", "screenshot"} {
		if strings.Contains(name, v) {
			return true
		}
	}
	return false
}
func ignoredSpoiler(name string) bool {
	for _, v := range []string{"цитата", "quote", "установ", "installation", "патч", "changelog"} {
		if strings.Contains(name, v) {
			return true
		}
	}
	return false
}

// Flatten only layout wrappers. Spoilers remain atomic unless they own game descriptions.
// This handles separators nested in alignment/font wrappers without breaking nested galleries.
func splitPornolab(root *goquery.Selection) []*goquery.Selection {
	var result []*goquery.Selection
	current := &html.Node{Type: html.ElementNode, Data: "div"}
	flush := func() {
		s := nodeSelection(current)
		if _, ok := sectionFields(s)["описание"]; ok {
			result = append(result, s)
		}
		current = &html.Node{Type: html.ElementNode, Data: "div"}
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if class(n, "q-wrap") || n.Data == "blockquote" {
			return
		}
		if class(n, "post-hr") {
			flush()
			return
		}
		if class(n, "sp-wrap") {
			sel := nodeSelection(n)
			name := spoilerName(sel)
			if ignoredSpoiler(name) {
				return
			}
			if !screenshotSpoiler(name) {
				body := sel.ChildrenFiltered(".sp-body")
				if body.Length() == 0 {
					body = sel
				}
				nested := splitPornolab(body)
				if len(nested) > 0 {
					flush()
					result = append(result, nested...)
					return
				}
			}
			current.AppendChild(cloneNode(n))
			return
		}
		if n.Type == html.TextNode || n.Data == "br" || n.Data == "wbr" || class(n, "post-b") || class(n, "postImg") || isSectionHeading(n) {
			current.AppendChild(cloneNode(n))
			return
		}
		// Preserve image-host anchors for normalizePostImg.
		if n.Data == "a" {
			current.AppendChild(cloneNode(n))
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range root.Nodes {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	flush()
	return result
}
func cloneNode(n *html.Node) *html.Node {
	c := &html.Node{Type: n.Type, DataAtom: n.DataAtom, Data: n.Data, Namespace: n.Namespace, Attr: append([]html.Attribute{}, n.Attr...)}
	for x := n.FirstChild; x != nil; x = x.NextSibling {
		c.AppendChild(cloneNode(x))
	}
	return c
}

var headingSize = regexp.MustCompile(`(?i)font-size\s*:\s*24px(?:\s|;|$)`)

func isSectionHeading(n *html.Node) bool {
	if n.Data != "span" {
		return false
	}
	for _, a := range n.Attr {
		if a.Key == "style" && headingSize.MatchString(a.Val) {
			return true
		}
	}
	return false
}

func sectionFields(s *goquery.Selection) map[string]string {
	out := map[string]string{}
	key := ""
	var value strings.Builder
	flush := func() {
		if key != "" {
			out[key] = strings.TrimSpace(strings.TrimLeft(cleanText(value.String()), ":： "))
		}
		value.Reset()
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if class(n, "sp-wrap") || class(n, "postImg") || n.Data == "img" {
			return
		}
		if class(n, "post-b") {
			flush()
			key = strings.ToLower(strings.TrimRight(cleanText(nodeSelection(n).Text()), ":： "))
			return
		}
		if n.Type == html.TextNode {
			value.WriteString(n.Data)
			return
		}
		if n.Data == "br" || class(n, "post-br") {
			value.WriteByte('\n')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range s.Nodes {
		walk(n)
	}
	flush()
	return out
}

func pornolabSections(doc *goquery.Document) []pornolabSection {
	raw := cleanText(doc.Find("h1.maintitle, #topic-title").First().Text())
	var result []pornolabSection
	for _, s := range splitPornolab(doc.Find("div.post-user-message").First()) {
		fields := sectionFields(s)
		g := models.Game{Version: versionFromTitle(raw), Tags: parseReleaseTags(raw), Languages: parseReleaseLanguages(raw), Description: truncateText(fields["описание"], 2000)}
		s.Find("span[style]").EachWithBreak(func(_ int, h *goquery.Selection) bool {
			if isSectionHeading(h.Nodes[0]) && h.Closest(".sp-wrap").Length() == 0 {
				g.Title = cleanText(h.Text())
				return false
			}
			return true
		})
		if v, ok := fields["версия"]; ok {
			g.Version = v
		}
		g.Author = releaseDeveloper(raw)
		for _, label := range []string{"разработчик/издатель", "разработчик / издатель", "разработчик", "издатель", "developer", "developer/publisher"} {
			if v, ok := fields[label]; ok {
				g.Author = v
				break
			}
		}
		if v, ok := fields["жанр"]; ok {
			g.Tags = nil
			seen := map[string]bool{}
			for _, tag := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' || r == '\n' }) {
				tag = cleanText(tag)
				if tag != "" && !seen[strings.ToLower(tag)] {
					seen[strings.ToLower(tag)] = true
					g.Tags = append(g.Tags, tag)
				}
			}
		}
		var langs []string
		hasLanguage := false
		for label, v := range fields {
			if strings.HasPrefix(label, "язык") {
				hasLanguage = true
				langs = append(langs, v)
			}
		}
		if hasLanguage {
			sort.Strings(langs)
			g.Languages = detectLanguages(strings.Join(langs, " "))
			for _, l := range parseReleaseLanguages(strings.Join(langs, " ")) {
				exists := false
				for _, x := range g.Languages {
					if l == x {
						exists = true
					}
				}
				if !exists {
					g.Languages = append(g.Languages, l)
				}
			}
		}
		if g.Languages == nil {
			g.Languages = []string{}
		}
		g.Engine = engineFromTokens(g.Tags)
		s.Find("var.postImg, img.postImg").Each(func(_ int, img *goquery.Selection) {
			if img.Closest(".sp-wrap").Length() == 0 && g.CoverPath == "" {
				g.CoverPath = normalizePostImg(img)
			}
		})
		seen := map[string]bool{}
		s.Find(".sp-wrap").Each(func(_ int, sp *goquery.Selection) {
			if !screenshotSpoiler(spoilerName(sp)) {
				return
			}
			sp.Find("var.postImg, img.postImg").Each(func(_ int, img *goquery.Selection) {
				u := normalizePostImg(img)
				if u != "" && u != g.CoverPath && !seen[u] {
					seen[u] = true
					g.Images = append(g.Images, u)
				}
			})
		})
		if g.CoverPath == "" && len(g.Images) > 0 {
			g.CoverPath = g.Images[0]
			g.Images = g.Images[1:]
		}
		if len(g.Images) > maxScreenshots {
			g.Images = g.Images[:maxScreenshots]
		}
		year := fields["год выпуска"]
		if _, ok := fields["год выпуска"]; !ok {
			for _, group := range bracketGroups(raw) {
				for _, part := range strings.Split(group, ",") {
					part = strings.TrimSpace(part)
					if len(part) == 4 && isYear(part) {
						year = part
					}
				}
			}
		}
		result = append(result, pornolabSection{g, year})
	}
	return result
}

var developerParentheses = regexp.MustCompile(`\(([^()]*)\)`)

func releaseDeveloper(raw string) string {
	for _, m := range developerParentheses.FindAllStringSubmatch(raw, -1) {
		if len(parseReleaseLanguages(m[1])) == 0 {
			return cleanText(m[1])
		}
	}
	return ""
}
