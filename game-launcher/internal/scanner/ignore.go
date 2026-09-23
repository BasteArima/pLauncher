package scanner

import (
	"context"
	"encoding/json"

	"game-launcher/internal/db"
)

// IgnoredPathsKey — ключ в таблице settings: JSON-список папок, которые скан не добавляет
// (например, папка с видео внутри папки с играми). Хранятся исходные пути,
// сравниваются через NormalizePath.
const IgnoredPathsKey = "ignored_paths"

// LoadIgnoredPaths читает список игнорируемых папок.
func LoadIgnoredPaths(ctx context.Context, repo db.GameRepository) []string {
	raw, _ := repo.GetSetting(ctx, IgnoredPathsKey)
	var paths []string
	if raw != "" {
		json.Unmarshal([]byte(raw), &paths)
	}
	if paths == nil {
		paths = []string{}
	}
	return paths
}

// SaveIgnoredPaths сохраняет список игнорируемых папок.
func SaveIgnoredPaths(ctx context.Context, repo db.GameRepository, paths []string) error {
	data, _ := json.Marshal(paths)
	return repo.SetSetting(ctx, IgnoredPathsKey, string(data))
}

// ignoredSet — множество нормализованных игнорируемых путей.
func ignoredSet(ctx context.Context, repo db.GameRepository) map[string]bool {
	set := map[string]bool{}
	for _, p := range LoadIgnoredPaths(ctx, repo) {
		set[NormalizePath(p)] = true
	}
	return set
}
