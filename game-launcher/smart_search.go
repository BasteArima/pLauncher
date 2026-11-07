package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"game-launcher/internal/apperr"
	"game-launcher/internal/models"
	"game-launcher/internal/parser"
)

type smartSearchState struct {
	mu     sync.Mutex
	id     uint64
	ctx    context.Context
	cancel context.CancelFunc
}

// Tokens survive navigation, but not stop, lock, panic or a new run.
func (a *App) BeginSmartSearch() uint64 {
	a.smartSearch.mu.Lock()
	defer a.smartSearch.mu.Unlock()
	if a.smartSearch.cancel != nil {
		a.smartSearch.cancel()
	}
	a.smartSearch.id++
	a.smartSearch.ctx = nil
	a.priv.mu.Lock()
	private := a.priv.locked || a.priv.panicHidden
	a.priv.mu.Unlock()
	if a.repo == nil || private {
		return 0
	}
	a.smartSearch.ctx, a.smartSearch.cancel = context.WithCancel(a.ctx)
	return a.smartSearch.id
}

func (a *App) StopSmartSearch(token uint64) {
	a.smartSearch.mu.Lock()
	defer a.smartSearch.mu.Unlock()
	if token != 0 && token != a.smartSearch.id {
		return
	}
	a.cancelSmartSearchLocked()
}

func (a *App) cancelSmartSearchLocked() {
	if a.smartSearch.cancel != nil {
		a.smartSearch.cancel()
	}
	a.smartSearch.ctx = nil
}

// Uses the same metadata application as ApplyGameInfo, with a cancellable parse
// and a commit serialized with StopSmartSearch. Never publish a late response.
func (a *App) ApplySmartGameInfo(token uint64, gameID, query string, result parser.SearchResult) (bool, error) {
	url := parser.ConfidentSearchURL(query, result)
	if url == "" {
		return false, nil
	}
	a.smartSearch.mu.Lock()
	ctx := a.smartSearch.ctx
	valid := token != 0 && token == a.smartSearch.id && ctx != nil && !a.isLocked()
	a.smartSearch.mu.Unlock()
	if !valid {
		return false, nil
	}
	applied := false
	// Isolate downloaded media too: cancellation must not overwrite a cover that
	// the game already uses, even if a parser finishes an image after cancellation.
	mediaDir, err := os.MkdirTemp(filepath.Join(a.dataDir, "covers"), "smart-")
	if err != nil {
		return false, apperr.New("app.library_save", nil, "couldn't save game info: %w", err)
	}
	defer func() {
		if !applied {
			_ = os.RemoveAll(mediaDir)
		}
	}()
	err = a.updateGameMetadataContext(ctx, gameID, url, true, mediaDir, func(g *models.Game) error {
		a.smartSearch.mu.Lock()
		defer a.smartSearch.mu.Unlock()
		if ctx.Err() != nil || a.smartSearch.ctx != ctx || a.isLocked() {
			return nil
		}
		// A manual edit or another lookup may have supplied a source in the meantime.
		current, err := a.gameByID(gameID)
		if err != nil {
			return err
		}
		if len(current.Sources) != 0 {
			return nil
		}
		err = a.repo.SaveGame(ctx, g)
		applied = err == nil
		return err
	})
	if ctx.Err() != nil {
		return false, nil
	}
	return applied, err
}
