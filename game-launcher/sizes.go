package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sync/atomic"
	"time"

	"game-launcher/internal/launchfiles"
	"game-launcher/internal/models"
	"game-launcher/internal/scanner"
)

// =========================================================================
// РАЗМЕР ПАПОК ИГР + МАССОВЫЕ ДЕЙСТВИЯ
// Размер считается в фоне (обход папки может занять секунды на больших играх),
// хранится в БД и пересчитывается, если устарел (sizeMaxAge) или по запросу.
// =========================================================================

const sizeMaxAge = 7 * 24 * time.Hour

// sizesRunning — идёт ли фоновый пересчёт (второй параллельно не запускаем).
var sizesRunning atomic.Bool

// folderSize суммирует размеры файлов в папке (символические ссылки не проходит).
func folderSize(root string) (int64, error) { return folderSizeUntil(root, nil) }

// folderSizeUntil — то же, но обход прерывается, как только stop() вернёт true.
func folderSizeUntil(root string, stop func() bool) (int64, error) {
	var total int64
	stopped := false
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if stop != nil && stop() {
			stopped = true
			return filepath.SkipAll
		}
		if err != nil {
			if path == root {
				return err
			}
			return nil // недоступные подпапки пропускаем
		}
		if d.Type()&fs.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	if err == nil && stopped {
		err = errSizeStopped
	}
	return total, err
}

var errSizeStopped = fmt.Errorf("size calculation stopped")

func sizeStale(g *models.Game, now time.Time) bool {
	return g.SizeCheckedAt == 0 || now.Sub(time.Unix(g.SizeCheckedAt, 0)) > sizeMaxAge
}

// refreshSizesAsync пересчитывает размеры в фоне; force — пересчитать все.
// По ходу шлёт "games-updated", чтобы UI подхватил цифры.
func (a *App) refreshSizesAsync(force bool) {
	if a.repo == nil || !sizesRunning.CompareAndSwap(false, true) {
		return
	}
	repo := a.repo
	stop := a.sizesStop.Load
	a.sizesWG.Add(1)
	go func() {
		defer a.sizesWG.Done()
		defer sizesRunning.Store(false)
		games, err := repo.GetAllGames(a.ctx)
		if err != nil {
			return
		}
		now := time.Now()
		done := 0
		for _, g := range games {
			if a.repo != repo || stop() { // сменили папку данных или закрываемся — прекращаем
				return
			}
			if g.FolderPath == "" || folderMissing(g.FolderPath) || (!force && !sizeStale(g, now)) {
				continue
			}
			size, err := folderSizeUntil(g.FolderPath, stop)
			if err != nil {
				continue
			}
			if stop() {
				return
			}
			if repo.SetGameSize(a.ctx, g.ID, size, time.Now().Unix()) == nil {
				done++
				if done%25 == 0 {
					a.emit("games-updated")
				}
			}
		}
		if done > 0 {
			a.emit("games-updated")
		}
	}()
}

// RefreshSizes запускает фоновый пересчёт размеров всех игр (кнопка в настройках).
func (a *App) RefreshSizes() {
	a.refreshSizesAsync(true)
}

// RecalcGameSize синхронно пересчитывает размер одной игры и возвращает его.
func (a *App) RecalcGameSize(id string) (int64, error) {
	if a.repo == nil {
		return 0, errNoDataDir()
	}
	g, err := a.gameByID(id)
	if err != nil {
		return 0, err
	}
	size, err := folderSize(g.FolderPath)
	if err != nil {
		return 0, err
	}
	return size, a.repo.SetGameSize(a.ctx, id, size, time.Now().Unix())
}

// --- Массовые действия ---

// RemoveGames удаляет несколько игр из библиотеки (файлы на диске остаются)
// и вычищает их из ручных списков коллекций. Возвращает число удалённых.
func (a *App) RemoveGames(ids []string) (int, error) {
	if a.repo == nil {
		return 0, errNoDataDir()
	}
	removed := map[string]bool{}
	for _, id := range ids {
		if err := a.repo.DeleteGame(a.ctx, id); err != nil {
			fmt.Printf("RemoveGames: %s: %v\n", id, err)
			continue
		}
		removed[id] = true
	}
	if len(removed) > 0 {
		if cols, err := a.GetCollections(); err == nil {
			changed := false
			for i := range cols {
				kept := cols[i].GameIDs[:0:0]
				for _, gid := range cols[i].GameIDs {
					if removed[gid] {
						changed = true
						continue
					}
					kept = append(kept, gid)
				}
				cols[i].GameIDs = kept
			}
			if changed {
				a.SaveCollections(cols)
			}
		}
	}
	return len(removed), nil
}

// SetFavorites ставит/снимает «избранное» у нескольких игр.
func (a *App) SetFavorites(ids []string, favorite bool) (int, error) {
	return a.updateMany(ids, func(g *models.Game) bool {
		if g.Favorite == favorite {
			return false
		}
		g.Favorite = favorite
		return true
	})
}

// DetectLaunchFiles ищет файл запуска для игр, у которых он не указан.
// Пустой ids — для всей библиотеки. Возвращает число найденных.
func (a *App) DetectLaunchFiles(ids []string) (int, error) {
	return a.updateMany(ids, func(g *models.Game) bool {
		if g.ExecPath != "" || g.FolderPath == "" || folderMissing(g.FolderPath) {
			return false
		}
		exe := scanner.FindBestExecutable(g.FolderPath)
		if exe == "" {
			return false
		}
		launchfiles.SetDetectedPrimary(g, exe)
		return true
	})
}

// updateMany применяет fn к играм из ids (пустой ids — ко всем) и сохраняет
// изменившиеся. Возвращает число сохранённых.
func (a *App) updateMany(ids []string, fn func(g *models.Game) bool) (int, error) {
	if a.repo == nil {
		return 0, errNoDataDir()
	}
	games, err := a.repo.GetAllGames(a.ctx)
	if err != nil {
		return 0, err
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	n := 0
	for _, g := range games {
		if len(ids) > 0 && !want[g.ID] {
			continue
		}
		if fn(g) {
			if err := a.repo.SaveGame(a.ctx, g); err != nil {
				fmt.Printf("updateMany: %s: %v\n", g.Title, err)
				continue
			}
			n++
		}
	}
	return n, nil
}
