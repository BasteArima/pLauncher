package scanner

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"game-launcher/internal/db"
	"game-launcher/internal/models"
)

// oldGameData описывает структуру JSON из старого скрипта на Python
type oldGameData struct {
	Title       string   `json:"title"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Images      []string `json:"images"`
}

// Scanner отвечает за поиск локальных игр на диске
type Scanner struct {
	repo db.GameRepository
}

// NewScanner создает новый экземпляр сканера
func NewScanner(repo db.GameRepository) *Scanner {
	return &Scanner{
		repo: repo,
	}
}

// MigrateOldGames сканирует указанную корневую директорию, ищет старые data.json
// и сохраняет их в новую базу данных. Возвращает количество добавленных игр.
// MigrateOldGames сканирует указанную директорию и добавляет игры ТОЛЬКО если их еще нет в базе
func (s *Scanner) MigrateOldGames(ctx context.Context, rootPath string) (int, error) {
	info, err := os.Stat(rootPath)
	if err != nil || !info.IsDir() {
		return 0, fmt.Errorf("корневая директория не найдена: %s", rootPath)
	}

	// 1. Получаем все существующие игры, чтобы не затереть ручные правки
	existingGames, err := s.repo.GetAllGames(ctx)
	if err != nil {
		return 0, fmt.Errorf("не удалось получить список игр из БД: %w", err)
	}
	existingIDs := make(map[string]bool)
	for _, g := range existingGames {
		existingIDs[g.ID] = true
	}

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return 0, fmt.Errorf("ошибка чтения директории %s: %w", rootPath, err)
	}

	count := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		gameFolderPath := filepath.Join(rootPath, entry.Name())

		// Генерируем ID (хэш от пути)
		hash := md5.Sum([]byte(gameFolderPath))
		gameID := hex.EncodeToString(hash[:])

		// 2. Если игра уже есть в базе — ПРОПУСКАЕМ!
		if existingIDs[gameID] {
			continue
		}

		// Если игры нет, проверяем, есть ли в ней старый data.json
		parsedFolderPath := filepath.Join(gameFolderPath, "parsed_data")
		jsonPath := filepath.Join(parsedFolderPath, "data.json")

		if _, err := os.Stat(jsonPath); err != nil {
			continue // Если старого json нет, пропускаем
		}

		game, err := s.parseOldJSON(jsonPath, gameFolderPath)
		if err != nil {
			log.Printf("Внимание: ошибка парсинга %s: %v", jsonPath, err)
			continue
		}

		// ЕСЛИ В СТАРОЙ БАЗЕ НЕ БЫЛО .EXE, ИЩЕМ ЕГО СЕЙЧАС
		if game.ExecPath == "" {
			game.ExecPath = autoFindExecutable(gameFolderPath)
		}

		if err := s.repo.SaveGame(ctx, game); err != nil {
			log.Printf("Внимание: ошибка сохранения игры %s: %v", game.Title, err)
			continue
		}

		count++
	}

	return count, nil
}

// parseOldJSON читает файл и конвертирует его в новую модель Game
func (s *Scanner) parseOldJSON(jsonPath, gameFolderPath string) (*models.Game, error) {
	fileBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}

	var oldData oldGameData
	if err := json.Unmarshal(fileBytes, &oldData); err != nil {
		return nil, err
	}

	// Генерируем ID.
	// Чтобы ID был уникальным, но постоянным (если мы запустим сканер дважды,
	// он должен обновить старую запись, а не создать дубликат),
	// мы берем MD5-хэш от абсолютного пути к папке с игрой.
	hash := md5.Sum([]byte(gameFolderPath))
	gameID := hex.EncodeToString(hash[:])

	// В старом коде на Python обложка отдельно не хранилась, бралась первая картинка
	var coverPath string
	if len(oldData.Images) > 0 {
		coverPath = oldData.Images[0]
	}

	game := &models.Game{
		ID:          gameID,
		Title:       oldData.Title,
		Description: oldData.Description,
		Version:     oldData.Version,
		Languages:   []string{}, // В старом JSON языков не было
		CoverPath:   coverPath,
		Images:      oldData.Images,
		ExecPath:    "", // Пока пусто, заполним это на этапе 4 (Системный контроллер)
		FolderPath:  gameFolderPath,
		TimePlayed:  0,
		AddedAt:     time.Now().Unix(),
	}

	// Защита от пустых названий (как в старом скрипте "Untitled")
	if game.Title == "" {
		game.Title = "Untitled"
	}

	return game, nil
}

// autoFindExecutable ищет .exe файл: сначала в корне, затем в подпапках.
// Игнорирует деинсталляторы и краш-репортеры.
func autoFindExecutable(folderPath string) string {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return ""
	}

	// 1. ПРИОРИТЕТ: Ищем в корне папки
	for _, e := range entries {
		if !e.IsDir() {
			name := strings.ToLower(e.Name())
			if strings.HasSuffix(name, ".exe") && !strings.Contains(name, "unins") && !strings.Contains(name, "crash") {
				return filepath.Join(folderPath, e.Name())
			}
		}
	}

	// 2. ВТОРИЧНЫЙ ПОИСК: Ищем в подпапках
	var foundExe string
	_ = filepath.WalkDir(folderPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || path == folderPath {
			return nil // Пропускаем ошибки и саму корневую папку (мы ее уже проверили)
		}

		// Если уже нашли файл, говорим функции WalkDir пропустить сканирование оставшихся папок
		if foundExe != "" {
			return filepath.SkipDir
		}

		if !d.IsDir() {
			name := strings.ToLower(d.Name())
			if strings.HasSuffix(name, ".exe") && !strings.Contains(name, "unins") && !strings.Contains(name, "crash") {
				foundExe = path
			}
		}
		return nil
	})

	return foundExe
}
