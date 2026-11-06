package main

import (
	"game-launcher/internal/apperr"
	"game-launcher/internal/launcher"
	"game-launcher/internal/launchfiles"
)

// Narrow interface also lets tests replace OS associations with a harmless fake opener.
type fileLauncher interface {
	OpenFolder(string) error
	FindExecutables(string) ([]string, error)
	LaunchAssociated(string, string) error
	LaunchUntracked(string, string) error
}

func (a *App) GetLaunchFileLabel(path string) string {
	if a.repo == nil || a.isLocked() {
		return ""
	}
	return launchfiles.LabelKey(path)
}

func (a *App) FindLaunchFileCandidates(gameID string) (*launchfiles.Candidates, error) {
	if a.repo == nil || a.isLocked() {
		return nil, nil
	}
	g, err := a.gameByID(gameID)
	if a.isLocked() {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if folderMissing(g.FolderPath) {
		return nil, apperr.New("app.folder_missing", apperr.P{"path": g.FolderPath}, "game folder not found: %s", g.FolderPath)
	}
	result, err := launchfiles.Find(a.ctx, g.FolderPath)
	if a.isLocked() {
		return nil, nil
	}
	if err != nil {
		return nil, apperr.New("launch.search", apperr.P{"path": g.FolderPath}, "couldn't search launch files in %s: %w", g.FolderPath, err)
	}
	return &result, nil
}

func (a *App) GetLaunchFileAvailability(gameID string) ([]launchfiles.FileInfo, error) {
	if a.repo == nil || a.isLocked() {
		return nil, nil
	}
	g, err := a.gameByID(gameID)
	if a.isLocked() {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := []launchfiles.FileInfo{}
	for _, file := range launchfiles.Files(g) {
		f := launchfiles.Describe(file.Path, file.Label, g.FolderPath)
		f.Primary = launchfiles.Key(file.Path) == launchfiles.Key(g.ExecPath)
		f.Availability = launchfiles.Availability(file.Path)
		out = append(out, f)
	}
	if a.isLocked() {
		return nil, nil
	}
	return out, nil
}

func (a *App) SelectLaunchFile(gameID string) (string, error) {
	if a.repo == nil || a.isLocked() {
		return "", nil
	}
	g, err := a.gameByID(gameID)
	if a.isLocked() {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	path, err := a.SelectExecutable(g.FolderPath)
	if a.isLocked() {
		return "", nil
	}
	if err != nil {
		return "", apperr.New("launch.select", nil, "couldn't select launch file: %w", err)
	}
	return path, nil
}

// The saved role at dispatch time determines accounting, even for callers of legacy Launch.
func (a *App) LaunchGameFile(gameID, path string) error {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	if a.repo == nil || a.isLocked() {
		return nil
	}
	g, err := a.gameByID(gameID)
	if a.isLocked() {
		return nil
	}
	if err != nil {
		return err
	}
	found := false
	primary := false
	for _, f := range launchfiles.Files(g) {
		if launchfiles.Key(f.Path) == launchfiles.Key(path) {
			path = f.Path
			primary = launchfiles.Key(path) == launchfiles.Key(g.ExecPath)
			found = true
			break
		}
	}
	if !found {
		return apperr.New("launch.unlisted", nil, "this file is no longer in the saved launch list; save or reload the game")
	}
	if folderMissing(g.FolderPath) {
		return apperr.New("app.folder_missing", apperr.P{"path": g.FolderPath}, "game folder not found: %s", g.FolderPath)
	}
	switch launchfiles.Availability(path) {
	case "missing":
		return apperr.New("launch.missing", apperr.P{"path": path}, "launch file not found: %s", path)
	case "unavailable":
		return apperr.New("launch.access", apperr.P{"path": path}, "cannot access launch file: %s", path)
	case "unsupported":
		return apperr.New("launch.unsupported", apperr.P{"path": path}, "not a supported launch file: %s", path)
	}
	if a.isLocked() {
		return nil
	}
	if a.launcher == nil {
		a.launcher = launcher.NewController()
	}
	if primary {
		return a.launchTrackedLocked(g.ID, path, g.FolderPath)
	}
	if err := a.launcher.LaunchUntracked(path, g.FolderPath); err != nil {
		return apperr.New("launch.failed", apperr.P{"path": path}, "couldn't open launch file %s: %w", path, err)
	}
	return nil
}
