package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"game-launcher/internal/apperr"
)

// =========================================================================
// САМООБНОВЛЕНИЕ ЛАУНЧЕРА
// Источник — GitHub Releases репозитория updateRepo (должен быть публичным:
// API без токена приватные репозитории не отдаёт). Релиз собирает CI
// (.github/workflows/release.yml): для каждой ОС свой файл + <имя>.sha256
// (см. assetNameFor). На macOS бандл .app сам себя не заменит — там только ссылка на релиз.
// =========================================================================

// updateRepo вшивается при сборке: -ldflags "-X main.updateRepo=owner/repo".
var updateRepo = "BasteArima/pLauncher"

// githubAPI — база API (подменяется в тестах).
var githubAPI = "https://api.github.com"

// executablePath и restartApp подменяются в тестах.
var executablePath = os.Executable
var restartApp = func(a *App, exe string) error {
	if err := exec.Command(exe, afterUpdateFlag).Start(); err != nil {
		return err
	}
	runtime.Quit(a.ctx)
	return nil
}

// afterUpdateFlag — новый процесс ждёт, пока старый освободит БД и файл.
const afterUpdateFlag = "--after-update"

// LauncherUpdate — результат проверки обновления лаунчера.
type LauncherUpdate struct {
	Available  bool   `json:"available"`
	CanInstall bool   `json:"can_install"` // есть сборка для этой ОС, которую можно поставить автоматически
	Current    string `json:"current"`
	Latest     string `json:"latest"`
	Notes      string `json:"notes"`    // описание релиза (что нового)
	PageURL    string `json:"page_url"` // страница релиза
	AssetURL   string `json:"asset_url"`
	AssetName  string `json:"asset_name"`
	Size       int64  `json:"size"`
	shaURL     string
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

var httpClient = &http.Client{Timeout: 5 * time.Minute}

// CheckLauncherUpdate запрашивает последний релиз и сравнивает с текущей версией.
// Сборки "dev" не обновляются (Available=false), но последняя версия показывается.
func (a *App) CheckLauncherUpdate() (*LauncherUpdate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/releases/latest", githubAPI, updateRepo), nil)
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
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, apperr.New("update.no_releases", apperr.P{"repo": updateRepo}, "no published releases in %s (or the repository is private)", updateRepo)
	default:
		return nil, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	u := &LauncherUpdate{
		Current: appVersion,
		Latest:  strings.TrimPrefix(rel.TagName, "v"),
		Notes:   rel.Body,
		PageURL: rel.HTMLURL,
	}
	want := assetNameFor(goruntime.GOOS, goruntime.GOARCH)
	for _, as := range rel.Assets {
		if want != "" && u.AssetURL == "" && strings.EqualFold(as.Name, want) {
			u.AssetURL, u.AssetName, u.Size = as.URL, as.Name, as.Size
		}
	}
	for _, as := range rel.Assets {
		if u.AssetName != "" && strings.EqualFold(as.Name, u.AssetName+".sha256") {
			u.shaURL = as.URL
		}
	}
	u.Available = appVersion != "dev" && newerVersion(u.Latest, appVersion)
	u.CanInstall = u.Available && u.AssetURL != ""
	return u, nil
}

// assetNameFor — имя файла сборки в релизе для ОС/архитектуры ("" — автоустановки нет).
// Должно совпадать с именами в .github/workflows/release.yml.
func assetNameFor(goos, goarch string) string {
	switch goos {
	case "windows":
		return "pLauncher.exe"
	case "linux":
		return "pLauncher-linux-" + goarch
	}
	return ""
}

// newerVersion сообщает, что версия a новее b ("1.10.0" > "1.9.3"; суффиксы вида
// "-beta" отбрасываются). Недостающие части считаются нулями.
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
	return false
}

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

// InstallLauncherUpdate скачивает новую версию рядом с текущим exe, проверяет
// контрольную сумму (если релиз её публикует), подменяет файл и перезапускает лаунчер.
// Запущенный exe в Windows нельзя перезаписать, но можно переименовать — поэтому
// старый уходит в pLauncher.exe.old (удаляется при следующем запуске).
func (a *App) InstallLauncherUpdate() error {
	u, err := a.CheckLauncherUpdate()
	if err != nil {
		return err
	}
	if !u.Available {
		return fmt.Errorf("already up to date (%s)", u.Current)
	}
	if !u.CanInstall {
		return apperr.New("update.manual", nil, "automatic update isn’t available on this system — download it from the release page")
	}
	exe, err := executablePath()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	newPath := exe + ".new"
	if err := downloadTo(u.AssetURL, newPath, u.Size); err != nil {
		os.Remove(newPath)
		return apperr.New("update.download", nil, "download failed: %w (you can download it manually from the release page)", err)
	}
	if u.shaURL != "" {
		if err := verifySHA256(newPath, u.shaURL); err != nil {
			os.Remove(newPath)
			return err
		}
	}
	if goruntime.GOOS != "windows" {
		os.Chmod(newPath, 0o755) // скачанный файл без бита исполнения
	}

	oldPath := exe + ".old"
	os.Remove(oldPath)
	if err := os.Rename(exe, oldPath); err != nil {
		os.Remove(newPath)
		return apperr.New("update.no_write", apperr.P{"path": filepath.Dir(exe)}, "couldn't replace the launcher file (no write access to %s?): %w", filepath.Dir(exe), err)
	}
	if err := os.Rename(newPath, exe); err != nil {
		os.Rename(oldPath, exe) // откат
		os.Remove(newPath)
		return err
	}
	return restartApp(a, exe)
}

// OpenLauncherReleasePage открывает страницу релизов в браузере.
func (a *App) OpenLauncherReleasePage() {
	runtime.BrowserOpenURL(a.ctx, fmt.Sprintf("https://github.com/%s/releases/latest", updateRepo))
}

func downloadTo(url, dest string, wantSize int64) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server answered %s", resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if wantSize > 0 && n != wantSize {
		return fmt.Errorf("incomplete file: %d of %d bytes", n, wantSize)
	}
	return nil
}

// verifySHA256 сверяет файл с суммой из файла вида "<hex>  pLauncher.exe".
func verifySHA256(path, shaURL string) error {
	resp, err := httpClient.Get(shaURL)
	if err != nil {
		return fmt.Errorf("couldn't get checksum: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return err
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return fmt.Errorf("empty checksum file")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, fields[0]) {
		return apperr.New("update.checksum", nil, "checksum mismatch: the downloaded file is damaged")
	}
	return nil
}

// cleanupAfterUpdate удаляет старый exe, оставшийся после самообновления.
func cleanupAfterUpdate() {
	if exe, err := executablePath(); err == nil {
		os.Remove(exe + ".old")
		os.Remove(exe + ".new")
	}
}
