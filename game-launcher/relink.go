package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"game-launcher/internal/apperr"
	"game-launcher/internal/models"
	"game-launcher/internal/scanner"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// =========================================================================
// ПЕРЕПРИВЯЗКА ПЕРЕМЕЩЁННЫХ ИГР
// Игра, чья папка исчезла, не удаляется, а переносится на новое место со всеми
// метаданными (обложка, теги, время, коллекции, ссылки).
// =========================================================================

// MissingGame — игра без папки на диске и найденный кандидат нового расположения.
type MissingGame struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	CoverPath   string `json:"cover_path"`
	OldPath     string `json:"old_path"`
	Candidate   string `json:"candidate"`    // найденная папка ("" — не нашли, можно указать вручную)
	DuplicateID string `json:"duplicate_id"` // запись, уже созданная сканом для Candidate (сольётся при привязке)
	DuplicateOf string `json:"duplicate_of"` // название этой записи (для подсказки в UI)
}

// folderMissing сообщает, что папки игры точно нет на диске.
// Пустой путь не считаем «пропавшим» — его нельзя проверить.
func folderMissing(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return os.IsNotExist(err)
}

// genericExeNames — имена запускалок, общие для многих игр одного движка:
// по ним нельзя опознать конкретную игру (RPG Maker = Game.exe, NW.js = nw.exe и т.п.).
var genericExeNames = map[string]bool{
	"game.exe": true, "nw.exe": true, "start.exe": true, "launcher.exe": true,
	"play.exe": true, "run.exe": true, "main.exe": true, "index.html": true,
	"start.bat": true, "run.bat": true, "game.bat": true,
}

// relExe возвращает путь к файлу запуска относительно папки игры ("" — если exe вне папки).
func relExe(folder, exe string) string {
	if folder == "" || exe == "" {
		return ""
	}
	rel, err := filepath.Rel(folder, exe)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return ""
	}
	return rel
}

// matchMoved подбирает новую папку для пропавшей игры среди dirs.
//  1. Папка с тем же именем (без учёта регистра) — основной случай «перенёс на другой диск».
//  2. Иначе — единственная папка, где лежит тот же файл запуска по тому же относительному
//     пути (случай «переименовал папку»). Для типовых имён (Game.exe…) не применяется.
//
// used — уже занятые кандидаты (чтобы две игры не получили одну папку).
func matchMoved(g *models.Game, dirs []string, used map[string]bool) string {
	base := strings.ToLower(filepath.Base(g.FolderPath))
	for _, d := range dirs {
		if used[scanner.NormalizePath(d)] {
			continue
		}
		if strings.ToLower(filepath.Base(d)) == base {
			return d
		}
	}

	rel := relExe(g.FolderPath, g.ExecPath)
	if rel == "" || genericExeNames[strings.ToLower(filepath.Base(rel))] {
		return ""
	}
	found := ""
	for _, d := range dirs {
		if used[scanner.NormalizePath(d)] {
			continue
		}
		if _, err := os.Stat(filepath.Join(d, rel)); err == nil {
			if found != "" {
				return "" // неоднозначно — пусть пользователь укажет сам
			}
			found = d
		}
	}
	return found
}

// relinkRoots — где искать переехавшие папки: папки сканирования, родители папок
// существующих игр (игры, добавленные перетаскиванием) и уцелевшие родители пропавших.
func (a *App) relinkRoots(games []*models.Game) []string {
	seen := map[string]bool{}
	var roots []string
	add := func(p string) {
		if p == "" {
			return
		}
		n := scanner.NormalizePath(p)
		if seen[n] {
			return
		}
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			seen[n] = true
			roots = append(roots, p)
		}
	}
	paths, _ := a.GetScanPaths()
	for _, p := range paths {
		add(p)
	}
	for _, g := range games {
		if g.FolderPath != "" {
			add(filepath.Dir(g.FolderPath))
		}
	}
	return roots
}

// listSubdirs возвращает подпапки первого уровня всех корней (без скрытых).
func listSubdirs(roots []string) []string {
	var dirs []string
	for _, r := range roots {
		entries, err := os.ReadDir(r)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				dirs = append(dirs, filepath.Join(r, e.Name()))
			}
		}
	}
	return dirs
}

// FindMissingGames возвращает игры, чьей папки нет на диске, с автоматически
// найденным новым расположением (если удалось).
func (a *App) FindMissingGames() ([]MissingGame, error) {
	out := []MissingGame{}
	if a.repo == nil || a.isLocked() {
		return out, nil
	}
	games, err := a.repo.GetAllGames(a.ctx)
	if err != nil {
		return nil, err
	}

	var missing []*models.Game
	owner := map[string]*models.Game{} // нормализованный путь → игра, которая им владеет
	for _, g := range games {
		if folderMissing(g.FolderPath) {
			missing = append(missing, g)
		} else if g.FolderPath != "" {
			owner[scanner.NormalizePath(g.FolderPath)] = g
		}
	}
	if len(missing) == 0 {
		return out, nil
	}

	dirs := listSubdirs(a.relinkRoots(games))
	used := map[string]bool{}
	for _, g := range missing {
		m := MissingGame{
			ID:        g.ID,
			Title:     g.Title,
			CoverPath: a.mediaRel(g.CoverPath),
			OldPath:   g.FolderPath,
		}
		if c := matchMoved(g, dirs, used); c != "" {
			n := scanner.NormalizePath(c)
			used[n] = true
			m.Candidate = c
			if dup := owner[n]; dup != nil {
				m.DuplicateID = dup.ID
				m.DuplicateOf = dup.Title
			}
		}
		out = append(out, m)
	}
	return out, nil
}

// RelinkGame переносит игру на новую папку. Путь к файлу запуска пересчитывается
// относительно новой папки. Если под новой папкой уже есть запись (её добавил скан
// после переезда) — она сливается в эту игру: время суммируется, членство
// в коллекциях переносится, дубликат удаляется.
func (a *App) RelinkGame(id, newPath string) (*models.Game, error) {
	if a.repo == nil {
		return nil, errNoDataDir()
	}
	info, err := os.Stat(newPath)
	if err != nil || !info.IsDir() {
		return nil, apperr.New("app.folder_missing", apperr.P{"path": newPath}, "folder not found: %s", newPath)
	}
	games, err := a.repo.GetAllGames(a.ctx)
	if err != nil {
		return nil, err
	}
	var g, dup *models.Game
	norm := scanner.NormalizePath(newPath)
	for _, x := range games {
		if x.ID == id {
			g = x
		} else if x.FolderPath != "" && scanner.NormalizePath(x.FolderPath) == norm {
			dup = x
		}
	}
	if g == nil {
		return nil, fmt.Errorf("game with ID %s not found in database", id)
	}

	oldFolder := g.FolderPath
	g.FolderPath = newPath
	g.ExecPath = rebaseExe(oldFolder, g.ExecPath, newPath)
	if g.Engine == "" {
		g.Engine = scanner.DetectEngine(newPath)
	}

	if dup != nil {
		g.TimePlayed += dup.TimePlayed
		if dup.LastLaunchedAt > g.LastLaunchedAt {
			g.LastLaunchedAt = dup.LastLaunchedAt
		}
		g.Favorite = g.Favorite || dup.Favorite
		if g.ExecPath == "" {
			g.ExecPath = dup.ExecPath
		}
	}

	if err := a.repo.SaveGame(a.ctx, g); err != nil {
		return nil, err
	}
	if dup != nil {
		if err := a.replaceInCollections(dup.ID, g.ID); err != nil {
			fmt.Printf("RelinkGame: коллекции не обновлены: %v\n", err)
		}
		if err := a.repo.DeleteGame(a.ctx, dup.ID); err != nil {
			fmt.Printf("RelinkGame: дубликат %s не удалён: %v\n", dup.ID, err)
		}
	}
	// Папка другая — размер пересчитаем в фоне
	if a.repo.SetGameSize(a.ctx, g.ID, 0, 0) == nil {
		g.SizeBytes, g.SizeCheckedAt = 0, 0
	}
	a.refreshSizesAsync(false)
	a.normalizeGames([]*models.Game{g})
	return g, nil
}

// RelinkAllFound привязывает все пропавшие игры, для которых найден кандидат.
// Возвращает число перепривязанных.
func (a *App) RelinkAllFound() (int, error) {
	list, err := a.FindMissingGames()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, m := range list {
		if m.Candidate == "" {
			continue
		}
		if _, err := a.RelinkGame(m.ID, m.Candidate); err != nil {
			fmt.Printf("RelinkAllFound: %s: %v\n", m.Title, err)
			continue
		}
		n++
	}
	return n, nil
}

// SelectRelinkFolder открывает диалог выбора нового расположения игры, начиная
// с ближайшей уцелевшей родительской папки старого пути.
func (a *App) SelectRelinkFolder(oldPath string) (string, error) {
	start := filepath.Dir(oldPath)
	for start != "" && folderMissing(start) {
		parent := filepath.Dir(start)
		if parent == start {
			start = ""
			break
		}
		start = parent
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Укажите новое расположение папки игры",
		DefaultDirectory: start,
	})
}

// rebaseExe переносит путь к файлу запуска из старой папки игры в новую.
// Если файла на прежнем относительном пути нет — ищет лучший exe заново.
// Файл запуска вне папки игры, если он ещё существует, оставляется как есть.
func rebaseExe(oldFolder, exe, newFolder string) string {
	if rel := relExe(oldFolder, exe); rel != "" {
		cand := filepath.Join(newFolder, rel)
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	} else if exe != "" {
		if _, err := os.Stat(exe); err == nil {
			return exe
		}
	}
	return scanner.FindBestExecutable(newFolder)
}

// replaceInCollections заменяет ID игры в ручных списках коллекций (без дублей).
func (a *App) replaceInCollections(oldID, newID string) error {
	cols, err := a.GetCollections()
	if err != nil {
		return err
	}
	changed := false
	for i := range cols {
		ids := cols[i].GameIDs
		out := make([]string, 0, len(ids))
		seen := map[string]bool{}
		for _, x := range ids {
			if x == oldID {
				x = newID
				changed = true
			}
			if !seen[x] {
				seen[x] = true
				out = append(out, x)
			}
		}
		cols[i].GameIDs = out
	}
	if !changed {
		return nil
	}
	return a.SaveCollections(cols)
}
