package main

import (
	"path"
	"strings"

	"game-launcher/internal/apperr"
	"game-launcher/internal/models"
	"game-launcher/internal/parser"
	"game-launcher/internal/scanner"
)

type NameSuggestion struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

// Работает и для Windows-путей в библиотеке, перенесённой на другую ОС.
func folderSearchName(folder string) NameSuggestion {
	name := path.Base(strings.TrimRight(strings.ReplaceAll(folder, "\\", "/"), "/"))
	if name == "." || name == "/" {
		name = ""
	}
	title, version := scanner.SuggestSearchName(name)
	return NameSuggestion{title, version}
}

func (a *App) SuggestSearchName(gameID string) (NameSuggestion, error) {
	if a.repo == nil || a.isLocked() {
		return NameSuggestion{}, nil
	}
	g, err := a.gameByID(gameID)
	if err != nil {
		return NameSuggestion{}, err
	}
	return folderSearchName(g.FolderPath), nil
}

func (a *App) SearchGameInfo(query string, sources []string, gameID string) (parser.SearchResult, error) {
	empty := parser.SearchResult{Hits: []parser.SearchHit{}, Failed: []parser.SearchFailure{}}
	if a.repo == nil || a.isLocked() {
		return empty, nil
	}
	hint := parser.SearchHint{}
	if gameID != "" {
		g, err := a.gameByID(gameID)
		if err != nil {
			return empty, err
		}
		hint.Version = folderSearchName(g.FolderPath).Version
		hint.Engine = g.Engine
	}
	result := parser.Search(a.ctx, nil, query, sources, hint)
	if a.isLocked() {
		return empty, nil
	}
	return result, nil
}

func (a *App) PreviewGameInfo(url string) (*models.Game, error) {
	if a.repo == nil || a.isLocked() {
		return nil, nil
	}
	p, err := parser.GetParser(url, "")
	if err != nil {
		return nil, err
	}
	g, err := p.Parse(parser.WithPreviewImages(a.ctx), url, "")
	if a.isLocked() {
		return nil, nil
	}
	return g, gameInfoParseErr(err)
}

func (a *App) ApplyGameInfo(gameID, url string) error {
	if a.repo == nil || a.isLocked() {
		return nil
	}
	return a.updateGameMetadata(gameID, url, true)
}

func gameInfoParseErr(err error) error {
	if err != nil && apperr.Code(err) == "" {
		return apperr.New("parse.read", nil, "couldn't read game info: %w", err)
	}
	return err
}
