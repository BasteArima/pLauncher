package scanner

import (
	"context"
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

// ScanFolder сканирует верхний слой подпапок rootPath и добавляет каждую как игру.
// Если в подпапке есть старый parsed_data/data.json — подхватывает метаданные из него.
// Игры, уже существующие в БД (по ID), пропускаются — ручные правки не затираются.
// Возвращает количество добавленных игр.
func (s *Scanner) ScanFolder(ctx context.Context, rootPath string) (int, error) {
	info, err := os.Stat(rootPath)
	if err != nil || !info.IsDir() {
		return 0, fmt.Errorf("root directory not found: %s", rootPath)
	}

	existingGames, err := s.repo.GetAllGames(ctx)
	if err != nil {
		return 0, fmt.Errorf("couldn't get games from database: %w", err)
	}
	// Дедуп по нормализованному пути к папке (а не по ID), чтобы
	// переезд/переименование папки не плодил дубликаты.
	existingByPath := make(map[string]*models.Game)
	for _, g := range existingGames {
		existingByPath[NormalizePath(g.FolderPath)] = g
	}

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return 0, fmt.Errorf("error reading directory %s: %w", rootPath, err)
	}

	count := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Пропускаем служебные/скрытые папки
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		gameFolderPath := filepath.Join(rootPath, entry.Name())
		norm := NormalizePath(gameFolderPath)

		if existing := existingByPath[norm]; existing != nil {
			// Игра уже в библиотеке — ручные правки не трогаем, но дозаполняем
			// пустой движок авто-определением по содержимому папки.
			if existing.Engine == "" {
				if eng := DetectEngine(gameFolderPath); eng != "" {
					existing.Engine = eng
					if err := s.repo.SaveGame(ctx, existing); err != nil {
						log.Printf("Внимание: не удалось дозаполнить движок для %s: %v", existing.Title, err)
					}
				}
			}
			continue
		}

		var game *models.Game

		// Если есть старый data.json — берём метаданные из него
		jsonPath := filepath.Join(gameFolderPath, "parsed_data", "data.json")
		if _, statErr := os.Stat(jsonPath); statErr == nil {
			if g, perr := s.parseOldJSON(jsonPath, gameFolderPath); perr == nil {
				game = g
			} else {
				log.Printf("Внимание: ошибка парсинга %s: %v", jsonPath, perr)
			}
		}

		// Иначе (или если json битый) создаём запись из имени папки
		if game == nil {
			game = &models.Game{
				ID:         NewID(),
				Title:      cleanFolderTitle(entry.Name()),
				Languages:  []string{},
				Images:     []string{},
				FolderPath: gameFolderPath,
				AddedAt:    time.Now().Unix(),
			}
		}

		if game.ExecPath == "" {
			game.ExecPath = FindBestExecutable(gameFolderPath)
		}
		if game.Engine == "" {
			game.Engine = DetectEngine(gameFolderPath)
		}

		if err := s.repo.SaveGame(ctx, game); err != nil {
			log.Printf("Внимание: ошибка сохранения игры %s: %v", game.Title, err)
			continue
		}

		existingByPath[norm] = game
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

	var coverPath string
	if len(oldData.Images) > 0 {
		coverPath = oldData.Images[0]
	}

	game := &models.Game{
		ID:          NewID(),
		Title:       oldData.Title,
		Description: oldData.Description,
		Version:     oldData.Version,
		Languages:   []string{},
		CoverPath:   coverPath,
		Images:      oldData.Images,
		FolderPath:  gameFolderPath,
		AddedAt:     time.Now().Unix(),
	}

	if game.Title == "" {
		game.Title = cleanFolderTitle(filepath.Base(gameFolderPath))
	}

	return game, nil
}

// cleanFolderTitle превращает имя папки в более читаемое название:
// убирает теги версий/раздач в скобках и нормализует разделители.
func cleanFolderTitle(name string) string {
	title := name
	// Срезаем хвост в квадратных скобках/фигурных: "Game [v1.2] [RUS]" -> "Game"
	if i := strings.IndexAny(title, "[{"); i > 0 {
		title = title[:i]
	}
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.ReplaceAll(title, ".", " ")
	title = strings.TrimSpace(title)
	if title == "" {
		title = name
	}
	return title
}
