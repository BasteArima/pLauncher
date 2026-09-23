package main

import (
	"fmt"
	"strings"

	"game-launcher/internal/scanner"
)

// =========================================================================
// ИГНОР ПАПОК ПРИ СКАНИРОВАНИИ
// Папка из списка не добавляется сканом (например, папка с видео рядом с играми).
// «Игнорировать» у игры = убрать её из лаунчера + внести папку в список.
// Файлы на диске не трогаются. Список ведётся в Настройки → Библиотека.
// =========================================================================

// GetIgnoredPaths возвращает список игнорируемых папок.
func (a *App) GetIgnoredPaths() []string {
	if a.repo == nil {
		return []string{}
	}
	return scanner.LoadIgnoredPaths(a.ctx, a.repo)
}

// addIgnored добавляет пути в список (без дубликатов).
func (a *App) addIgnored(paths []string) error {
	list := scanner.LoadIgnoredPaths(a.ctx, a.repo)
	seen := map[string]bool{}
	for _, p := range list {
		seen[scanner.NormalizePath(p)] = true
	}
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if n := scanner.NormalizePath(p); !seen[n] {
			seen[n] = true
			list = append(list, p)
		}
	}
	return scanner.SaveIgnoredPaths(a.ctx, a.repo, list)
}

// IgnoreGames убирает игры из лаунчера и вносит их папки в игнор скана.
// Возвращает число убранных игр.
func (a *App) IgnoreGames(ids []string) (int, error) {
	if a.repo == nil {
		return 0, fmt.Errorf("data folder is not selected yet")
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	games, err := a.repo.GetAllGames(a.ctx)
	if err != nil {
		return 0, err
	}
	var paths []string
	for _, g := range games {
		if want[g.ID] && g.FolderPath != "" {
			paths = append(paths, g.FolderPath)
		}
	}
	if err := a.addIgnored(paths); err != nil {
		return 0, err
	}
	return a.RemoveGames(ids)
}

// AddIgnoredPath вносит папку в игнор (из настроек). Если она уже есть в библиотеке —
// игра убирается, чтобы список и библиотека не противоречили друг другу.
// Возвращает число убранных игр.
func (a *App) AddIgnoredPath(path string) (int, error) {
	if a.repo == nil {
		return 0, fmt.Errorf("data folder is not selected yet")
	}
	if strings.TrimSpace(path) == "" {
		return 0, nil
	}
	if err := a.addIgnored([]string{path}); err != nil {
		return 0, err
	}
	norm := scanner.NormalizePath(path)
	games, err := a.repo.GetAllGames(a.ctx)
	if err != nil {
		return 0, err
	}
	var ids []string
	for _, g := range games {
		if g.FolderPath != "" && scanner.NormalizePath(g.FolderPath) == norm {
			ids = append(ids, g.ID)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return a.RemoveGames(ids)
}

// RemoveIgnoredPath убирает папку из игнора (при следующем скане она снова добавится).
func (a *App) RemoveIgnoredPath(path string) error {
	if a.repo == nil {
		return nil
	}
	norm := scanner.NormalizePath(path)
	list := scanner.LoadIgnoredPaths(a.ctx, a.repo)
	out := make([]string, 0, len(list))
	for _, p := range list {
		if scanner.NormalizePath(p) != norm {
			out = append(out, p)
		}
	}
	return scanner.SaveIgnoredPaths(a.ctx, a.repo, out)
}

// unignore — явное добавление папки (перетаскивание/«отдельная игра») снимает её с игнора.
func (a *App) unignore(path string) {
	norm := scanner.NormalizePath(path)
	for _, p := range scanner.LoadIgnoredPaths(a.ctx, a.repo) {
		if scanner.NormalizePath(p) == norm {
			a.RemoveIgnoredPath(p)
			return
		}
	}
}
