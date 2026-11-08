package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"game-launcher/internal/apperr"
	"game-launcher/internal/models"
	"game-launcher/internal/parser"
)

type ManualGameChoice struct {
	GameID     string                    `json:"game_id"`
	URL        string                    `json:"url"`
	Collection parser.PornolabCollection `json:"collection"`
}
type MetadataApplyResult struct {
	Applied bool   `json:"applied"`
	Manual  bool   `json:"manual"`
	GameID  string `json:"game_id"`
}

func (a *App) collectionPrivate() bool {
	a.priv.mu.Lock()
	defer a.priv.mu.Unlock()
	return a.priv.locked || a.priv.panicHidden
}
func (a *App) clearCollectionState() {
	a.collectionMu.Lock()
	defer a.collectionMu.Unlock()
	a.pornolab.Clear()
	a.manualChoices = nil
}

// Resolve uses the same response in both metadata panels. Refresh starts a new page action.
func (a *App) ResolvePornolabCollection(gameID, url string, refresh bool) (parser.PornolabCollection, error) {
	empty := parser.PornolabCollection{Candidates: []parser.PornolabCandidate{}}
	if a.repo == nil || a.collectionPrivate() {
		return empty, nil
	}
	g, err := a.gameByID(gameID)
	if err != nil {
		return empty, err
	}
	p, err := parser.GetParser(url, "")
	if err != nil {
		return empty, err
	}
	pornolab, ok := p.(*parser.PornlabParser)
	if !ok {
		return empty, apperr.New("parse.unsupported", nil, "unknown source or unsupported site")
	}
	ctx, cancel := a.pornolab.Context(a.ctx)
	defer cancel()
	suggestion := folderSearchName(g.FolderPath)
	result, err := pornolab.InspectCollection(parser.WithPornolab(ctx, parser.PornolabOptions{Cache: &a.pornolab, Fresh: refresh}), url, suggestion.Title, parser.SearchHint{Version: suggestion.Version, Engine: g.Engine})
	if !a.pornolab.Valid(ctx) || a.collectionPrivate() {
		return empty, context.Canceled
	}
	return result, gameInfoParseErr(err)
}

func collectionURL(raw string, index int) (string, error) {
	base, _, err := parser.PornolabURL(raw)
	if err != nil {
		return "", err
	}
	return base + "#game=" + strconv.Itoa(index), nil
}

func (a *App) PreviewPornolabGame(gameID, url, snapshot string, index int) (*models.Game, error) {
	if a.repo == nil || a.collectionPrivate() {
		return nil, nil
	}
	if _, err := a.gameByID(gameID); err != nil {
		return nil, err
	}
	selected, err := collectionURL(url, index)
	if err != nil {
		return nil, err
	}
	p, err := parser.GetParser(selected, "")
	if err != nil {
		return nil, err
	}
	if _, ok := p.(*parser.PornlabParser); !ok {
		return nil, apperr.New("parse.unsupported", nil, "unknown source or unsupported site")
	}
	if snapshot == "" {
		return nil, apperr.New("parse.collection_choice_required", nil, "choose a game from this release")
	}
	ctx, cancel := a.pornolab.Context(a.ctx)
	defer cancel()
	g, err := p.Parse(parser.WithPornolab(ctx, parser.PornolabOptions{Cache: &a.pornolab, Snapshot: snapshot}), selected, "")
	if !a.pornolab.Valid(ctx) || a.collectionPrivate() {
		return nil, context.Canceled
	}
	return g, gameInfoParseErr(err)
}

func (a *App) ApplyPornolabGame(gameID, url, snapshot string, index int) error {
	if a.repo == nil || a.collectionPrivate() {
		return nil
	}
	if parser.SourceName(url) != "Pornolab" {
		return apperr.New("parse.unsupported", nil, "unknown source or unsupported site")
	}
	if snapshot == "" {
		return apperr.New("parse.collection_choice_required", nil, "choose a game from this release")
	}
	selected, err := collectionURL(url, index)
	if err != nil {
		return err
	}
	ctx := parser.WithPornolab(a.ctx, parser.PornolabOptions{Cache: &a.pornolab, Snapshot: snapshot})
	// Separate files until the operation commits, including when privacy interrupts a download.
	dir, err := os.MkdirTemp(filepath.Join(a.dataDir, "covers"), "collection-")
	if err != nil {
		return apperr.New("app.library_save", nil, "couldn't save game info: %w", err)
	}
	applied := false
	defer func() {
		if !applied {
			_ = os.RemoveAll(dir)
		}
	}()
	err = a.updateGameMetadataContext(ctx, gameID, selected, true, dir, func(g *models.Game) error { err := a.repo.SaveGame(a.ctx, g); applied = err == nil; return err })
	if applied {
		a.collectionMu.Lock()
		delete(a.manualChoices, gameID)
		a.collectionMu.Unlock()
	}
	return err
}

func manualCollectionError(err error) bool {
	switch apperr.Code(err) {
	case "parse.collection_choice_required", "parse.collection_game_missing", "parse.collection_changed":
		return true
	}
	return false
}

// The frontend's existing smart-list handoff can use this payload without rerunning a search.
func (a *App) rememberManualChoice(ctx context.Context, gameID, url string) {
	result, err := a.ResolvePornolabCollection(gameID, url, false)
	if err != nil {
		return
	}
	a.collectionMu.Lock()
	defer a.collectionMu.Unlock()
	if ctx.Err() != nil || a.collectionPrivate() {
		return
	}
	if a.manualChoices == nil {
		a.manualChoices = map[string]ManualGameChoice{}
	}
	a.manualChoices[gameID] = ManualGameChoice{gameID, url, result}
}
func (a *App) GetManualGameChoices() []ManualGameChoice {
	a.collectionMu.Lock()
	defer a.collectionMu.Unlock()
	result := []ManualGameChoice{}
	if a.repo == nil || a.collectionPrivate() {
		return result
	}
	for id, c := range a.manualChoices {
		if _, err := a.gameByID(id); err == nil {
			result = append(result, c)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].GameID < result[j].GameID })
	return result
}

// One item of a visible-library batch. Begin/StopSmartSearch supplies the run/cancellation token.
func (a *App) ReparseGameInfo(token uint64, gameID string) (MetadataApplyResult, error) {
	result := MetadataApplyResult{GameID: gameID}
	a.smartSearch.mu.Lock()
	ctx := a.smartSearch.ctx
	valid := a.repo != nil && token != 0 && token == a.smartSearch.id && ctx != nil && !a.collectionPrivate()
	a.smartSearch.mu.Unlock()
	if !valid {
		return result, nil
	}
	g, err := a.gameByID(gameID)
	if err != nil {
		return result, err
	}
	i := primarySourceIndex(g)
	if i < 0 {
		return result, nil
	}
	url := g.Sources[i].URL
	dir, err := os.MkdirTemp(filepath.Join(a.dataDir, "covers"), "reparse-")
	if err != nil {
		return result, apperr.New("app.library_save", nil, "couldn't save game info: %w", err)
	}
	defer func() {
		if !result.Applied {
			_ = os.RemoveAll(dir)
		}
	}()
	err = a.updateGameMetadataContext(ctx, gameID, url, false, dir, func(updated *models.Game) error {
		a.smartSearch.mu.Lock()
		defer a.smartSearch.mu.Unlock()
		if ctx.Err() != nil || a.smartSearch.ctx != ctx || a.collectionPrivate() {
			return nil
		}
		current, e := a.gameByID(gameID)
		if e != nil {
			return e
		}
		j := primarySourceIndex(current)
		if j < 0 || current.Sources[j] != g.Sources[i] {
			return nil
		}
		e = a.repo.SaveGame(ctx, updated)
		result.Applied = e == nil
		return e
	})
	if ctx.Err() != nil {
		return result, nil
	}
	if manualCollectionError(err) {
		result.Manual = true
		a.rememberManualChoice(ctx, gameID, url)
		return result, nil
	}
	return result, err
}
