package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"game-launcher/internal/apperr"
)

// =========================================================================
// САМООБНОВЛЕНИЕ ЛАУНЧЕРА: проверка новых версий
// Источник — GitHub Releases репозитория updateRepo (должен быть публичным:
// API без токена приватные репозитории не отдаёт). Релиз собирает CI
// (.github/workflows/release.yml): для каждой ОС свой файл + <имя>.sha256
// (см. assetNameFor). На macOS бандл .app сам себя не заменит — там только ссылка на релиз.
// Загрузка и установка — в updater_install.go.
// =========================================================================

// updateRepo вшивается при сборке: -ldflags "-X main.updateRepo=owner/repo".
var updateRepo = "BasteArima/pLauncher"

// githubAPI — база API (подменяется в тестах).
var githubAPI = "https://api.github.com"

// prereleaseEnv — переменная окружения: показывать и пре-релизы (проверка обновления на себе).
const prereleaseEnv = "PLAUNCHER_UPDATE_PRERELEASE"

// maxReleaseNotes — сколько пропущенных версий показываем в окне обновления.
const maxReleaseNotes = 10

// LauncherRelease — одна версия в списке «что нового».
type LauncherRelease struct {
	Version string `json:"version"`
	Notes   string `json:"notes"` // раздел «What's new» описания релиза (markdown)
	URL     string `json:"url"`
}

// LauncherUpdate — результат проверки обновления лаунчера.
type LauncherUpdate struct {
	Available    bool              `json:"available"`
	CanInstall   bool              `json:"can_install"`   // можно скачать и поставить прямо из лаунчера
	ManualReason string            `json:"manual_reason"` // почему только вручную: "os" (нет сборки) | "readonly" (нет прав на папку)
	Current      string            `json:"current"`
	Latest       string            `json:"latest"`
	Releases     []LauncherRelease `json:"releases"` // все версии новее текущей, новые первыми
	PageURL      string            `json:"page_url"` // страница релиза
	AssetURL     string            `json:"asset_url"`
	AssetName    string            `json:"asset_name"`
	Size         int64             `json:"size"`
	shaURL       string
}

type ghRelease struct {
	TagName    string `json:"tag_name"`
	Body       string `json:"body"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

func (r ghRelease) version() string { return strings.TrimPrefix(r.TagName, "v") }

var httpClient = &http.Client{Timeout: 30 * time.Second}

// fetchReleases возвращает опубликованные релизы (черновики отброшены, пре-релизы —
// только с prereleaseEnv) от новых к старым.
func fetchReleases() ([]ghRelease, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/releases?per_page=30", githubAPI, updateRepo), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "pLauncher/"+appVersion)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, apperr.New("update.offline", nil, "couldn't reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	noReleases := apperr.New("update.no_releases", apperr.P{"repo": updateRepo}, "no published releases in %s (or the repository is private)", updateRepo)
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, noReleases
	default:
		return nil, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var all []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&all); err != nil {
		return nil, err
	}
	withPre := os.Getenv(prereleaseEnv) != ""
	var out []ghRelease
	for _, r := range all {
		if !r.Draft && (withPre || !r.Prerelease) {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return nil, noReleases
	}
	sort.SliceStable(out, func(i, j int) bool { return newerVersion(out[i].version(), out[j].version()) })
	return out, nil
}

// CheckLauncherUpdate запрашивает релизы и сравнивает с текущей версией.
// Сборки "dev" не обновляются (Available=false), но последняя версия показывается.
func (a *App) CheckLauncherUpdate() (*LauncherUpdate, error) {
	rels, err := fetchReleases()
	if err != nil {
		return nil, err
	}
	latest := rels[0]
	u := &LauncherUpdate{Current: appVersion, Latest: latest.version(), PageURL: latest.HTMLURL}

	want := assetNameFor(goruntime.GOOS, goruntime.GOARCH)
	for _, as := range latest.Assets {
		if want != "" && u.AssetURL == "" && strings.EqualFold(as.Name, want) {
			u.AssetURL, u.AssetName, u.Size = as.URL, as.Name, as.Size
		}
	}
	for _, as := range latest.Assets {
		if u.AssetName != "" && strings.EqualFold(as.Name, u.AssetName+".sha256") {
			u.shaURL = as.URL
		}
	}

	u.Available = appVersion != "dev" && newerVersion(u.Latest, appVersion)
	if !u.Available {
		return u, nil
	}
	for _, r := range rels {
		if newerVersion(r.version(), appVersion) && len(u.Releases) < maxReleaseNotes {
			u.Releases = append(u.Releases, LauncherRelease{Version: r.version(), Notes: whatsNew(r.Body), URL: r.HTMLURL})
		}
	}
	switch {
	case u.AssetURL == "":
		u.ManualReason = "os"
	case !exeDirWritable():
		u.ManualReason = "readonly"
	default:
		u.CanInstall = true
	}
	return u, nil
}

// exeDirWritable — можно ли заменить файл лаунчера (Program Files и т.п. — нельзя).
func exeDirWritable() bool {
	exe, err := currentExe()
	return err == nil && dirWritable(filepath.Dir(exe))
}

// assetNameFor — имя файла сборки в релизе для ОС/архитектуры ("" — автоустановки нет).
// Должно совпадать с именами в .github/workflows/release.yml: по ним обновление
// находят уже установленные версии, поэтому менять их нельзя.
func assetNameFor(goos, goarch string) string {
	switch goos {
	case "windows":
		return "pLauncher.exe"
	case "linux":
		return "pLauncher-linux-" + goarch
	}
	return ""
}

var releaseHeadingRe = regexp.MustCompile(`(?m)^##[ \t]+(.+?)[ \t]*$`)

// whatsNew вырезает из описания релиза раздел со списком изменений: инструкции по
// установке в окне обновления не нужны. Нет такого раздела — текст до первого
// заголовка, а если его нет — описание целиком.
func whatsNew(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	heads := releaseHeadingRe.FindAllStringSubmatchIndex(body, -1)
	for i, h := range heads {
		switch strings.ToLower(strings.ReplaceAll(body[h[2]:h[3]], "’", "'")) {
		case "what's new", "highlights", "changes":
			end := len(body)
			if i+1 < len(heads) {
				end = heads[i+1][0]
			}
			return strings.TrimSpace(body[h[1]:end])
		}
	}
	if len(heads) > 0 {
		if intro := strings.TrimSpace(body[:heads[0][0]]); intro != "" {
			return intro
		}
	}
	return strings.TrimSpace(body)
}

// newerVersion сообщает, что версия a новее b ("1.10.0" > "1.9.3"). Недостающие части
// считаются нулями. При равных номерах релиз новее своего пре-релиза ("1.1.0" > "1.1.0-rc.1").
func newerVersion(a, b string) bool {
	pa, pb := versionParts(a), versionParts(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x > y
		}
	}
	return !isPrerelease(a) && isPrerelease(b)
}

func isPrerelease(v string) bool { return strings.Contains(v, "-") }

func versionParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	var out []int
	for _, s := range strings.Split(v, ".") {
		n, _ := strconv.Atoi(s)
		out = append(out, n)
	}
	return out
}

// OpenLauncherReleasePage открывает страницу релизов в браузере.
func (a *App) OpenLauncherReleasePage() {
	runtime.BrowserOpenURL(a.ctx, fmt.Sprintf("https://github.com/%s/releases/latest", updateRepo))
}
