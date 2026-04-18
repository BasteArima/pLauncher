package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"game-launcher/internal/parser"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"game-launcher/internal/db"
	"game-launcher/internal/launcher"
	"game-launcher/internal/models"
	"game-launcher/internal/scanner"

	"github.com/wailsapp/wails/v2/pkg/runtime" // <--- ДОБАВИТЬ ЭТУ СТРОКУ
)

// App — это ядро нашего приложения, связывающее все модули
type App struct {
	ctx      context.Context
	repo     db.GameRepository
	scanner  *scanner.Scanner
	launcher *launcher.Controller
}

// NewApp создает новое приложение
func NewApp() *App {
	return &App{}
}

// startup вызывается при старте приложения (до показа окна).
// Здесь мы инициализируем все наши сервисы.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx // Сохраняем контекст, он нужен для вызова системных диалогов Wails

	// Создаем папку для базы данных в AppData пользователя (стандарт для десктопа)
	// PORTABLE MODE: Создаем папку data рядом с лаунчером
	dataDir := filepath.Join(".", "data")
	os.MkdirAll(filepath.Join(dataDir, "covers"), 0755) // Папка для обложек

	dbPath := filepath.Join(dataDir, "games.db")

	// 1. Инициализируем БД
	repo, err := db.NewSQLiteRepo(dbPath)
	if err != nil {
		// В реальном приложении тут лучше вызвать логгер или Wails-диалог с ошибкой
		fmt.Printf("Критическая ошибка инициализации БД: %v\n", err)
	}
	a.repo = repo

	// 2. Инициализируем остальные модули, передавая им зависимости (Dependency Injection)
	a.scanner = scanner.NewScanner(a.repo)
	a.launcher = launcher.NewController()
}

// shutdown вызывается при закрытии приложения.
// Идеальное место для корректного освобождения ресурсов (чтобы не было утечек памяти).
func (a *App) shutdown(ctx context.Context) {
	if a.repo != nil {
		// Приводим интерфейс к конкретному типу для вызова специфичного метода Close
		if sqliteRepo, ok := a.repo.(*db.SQLiteRepo); ok {
			sqliteRepo.Close()
		}
	}
}

// =========================================================================
// ЭКСПОРТИРУЕМЫЕ МЕТОДЫ (Они автоматически станут доступны в JavaScript)
// =========================================================================

// GetGames возвращает список всех игр для отрисовки в UI
func (a *App) GetGames() ([]*models.Game, error) {
	return a.repo.GetAllGames(a.ctx)
}

// ScanLocalFolder запускает миграцию старых data.json из указанной папки
func (a *App) ScanLocalFolder(rootPath string) (int, error) {
	return a.scanner.MigrateOldGames(a.ctx, rootPath)
}

// Launch запускает игру
func (a *App) Launch(gameID string, exePath string, folderPath string) error {
	// Сохраняем время запуска
	if games, err := a.repo.GetAllGames(a.ctx); err == nil {
		for _, g := range games {
			if g.ID == gameID {
				g.LastLaunchedAt = time.Now().Unix()
				a.repo.SaveGame(a.ctx, g)
				break
			}
		}
	}
	return a.launcher.LaunchGame(exePath, folderPath)
}

// OpenFolder просто открывает папку с игрой в проводнике
func (a *App) OpenFolder(folderPath string) error {
	return a.launcher.OpenFolder(folderPath)
}

// FindExecutables ищет все .exe файлы в папке (чтобы пользователь мог выбрать нужный)
func (a *App) FindExecutables(folderPath string) ([]string, error) {
	return a.launcher.FindExecutables(folderPath)
}

// UpdateGameMetadata скачивает данные по ссылке и обновляет существующую игру
func (a *App) UpdateGameMetadata(gameID string, url string) error {
	// 1. Ищем существующую игру (для простоты достаем все и фильтруем)
	games, err := a.repo.GetAllGames(a.ctx)
	if err != nil {
		return err
	}

	var targetGame *models.Game
	for _, g := range games {
		if g.ID == gameID {
			targetGame = g
			break
		}
	}

	if targetGame == nil {
		return fmt.Errorf("игра с ID %s не найдена в базе", gameID)
	}

	// 2. Инициализируем парсер
	p, err := parser.GetParser(url, "")
	if err != nil {
		return err
	}

	// 3. Скачиваем данные прямо в папку с игрой (как в твоем старом питон-скрипте)
	saveDir := filepath.Join(".", "data", "covers", targetGame.ID)
	parsedData, err := p.Parse(a.ctx, url, saveDir)
	if err != nil {
		return err
	}

	// 4. Обновляем поля игры новыми данными
	if parsedData.Title != "" {
		targetGame.Title = parsedData.Title
	}
	if parsedData.Version != "" {
		targetGame.Version = parsedData.Version
	}
	if parsedData.Description != "" {
		targetGame.Description = parsedData.Description
	}
	if len(parsedData.Images) > 0 {
		targetGame.Images = parsedData.Images
		targetGame.CoverPath = parsedData.CoverPath
	}

	// 5. Сохраняем обновленную игру обратно в БД
	return a.repo.SaveGame(a.ctx, targetGame)
}

// RemoveGame удаляет игру из базы данных лаунчера (файлы на диске остаются)
func (a *App) RemoveGame(id string) error {
	return a.repo.DeleteGame(a.ctx, id)
}

// SelectFolder открывает системное окно выбора папки
func (a *App) SelectFolder() (string, error) {
	options := runtime.OpenDialogOptions{
		Title: "Выберите корневую папку с играми",
	}
	// Открывает диалог выбора папки (Directory)
	return runtime.OpenDirectoryDialog(a.ctx, options)
}

// GetScanPath возвращает сохраненный путь из БД
func (a *App) GetScanPath() (string, error) {
	return a.repo.GetSetting(a.ctx, "scan_path")
}

// SetScanPath сохраняет выбранный путь в БД
func (a *App) SetScanPath(path string) error {
	return a.repo.SetSetting(a.ctx, "scan_path", path)
}

// SearchGames вызывает поиск по базе данных. Если запрос пустой, отдает все игры.
func (a *App) SearchGames(query string) ([]*models.Game, error) {
	if query == "" {
		return a.repo.GetAllGames(a.ctx)
	}
	return a.repo.SearchGames(a.ctx, query)
}

// UpdateGame сохраняет вручную отредактированную игру в БД
func (a *App) UpdateGame(game models.Game) error {
	// Wails автоматически конвертирует JSON-объект из JS в структуру models.Game
	return a.repo.SaveGame(a.ctx, &game)
}

// SelectCoverImage открывает системное окно для выбора локальной картинки
func (a *App) SelectCoverImage() (string, error) {
	options := runtime.OpenDialogOptions{
		Title: "Выберите обложку для игры",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Изображения (*.jpg, *.png, *.webp)",
				Pattern:     "*.jpg;*.jpeg;*.png;*.webp;*.gif",
			},
		},
	}
	return runtime.OpenFileDialog(a.ctx, options)
}

// CopyCoverToData копирует выбранную картинку в локальную папку data лаунчера
// CopyCoverToData копирует обложку в папку data/covers с гарантированно уникальным именем
func (a *App) CopyCoverToData(gameID string, sourcePath string) (string, error) {
	ext := filepath.Ext(sourcePath)
	if ext == "" {
		ext = ".jpg" // На всякий случай, если у файла нет расширения
	}

	// Генерируем 100% уникальное имя файла с помощью time.Now().UnixNano()
	fileName := fmt.Sprintf("%s_cover_%d%s", gameID, time.Now().UnixNano(), ext)

	// Путь сохранения
	coversDir := filepath.Join(".", "data", "covers")
	destPath := filepath.Join(coversDir, fileName)

	// Убеждаемся, что папка существует
	if err := os.MkdirAll(coversDir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать папку для обложек: %w", err)
	}

	// Открываем исходный файл
	src, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	// Создаем новый файл
	dst, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	// Копируем байты
	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}

	return destPath, nil
}

// AddGamesFromDrop принимает пути перетаскиваемых папок и добавляет их в БД, если их там нет
func (a *App) AddGamesFromDrop(paths []string) (int, error) {
	games, err := a.repo.GetAllGames(a.ctx)
	if err != nil {
		return 0, err
	}

	existingIDs := make(map[string]bool)
	for _, g := range games {
		existingIDs[g.ID] = true
	}

	addedCount := 0
	for _, p := range paths {
		fmt.Printf("Попытка добавить путь: '%s'\n", p)

		if p == "" || p == "undefined" {
			fmt.Println("❌ Ошибка: путь пустой. Браузер заблокировал передачу абсолютного пути.")
			continue
		}

		info, err := os.Stat(p)
		if err != nil {
			fmt.Printf("❌ Ошибка os.Stat (папка не найдена): %v\n", err)
			continue
		}
		if !info.IsDir() {
			fmt.Printf("❌ Ошибка: Это не папка, а файл: %s\n", p)
			continue
		}

		hash := md5.Sum([]byte(p))
		gameID := hex.EncodeToString(hash[:])

		if existingIDs[gameID] {
			fmt.Printf("⚠️ Пропуск: Игра уже существует в БД: %s\n", p)
			continue
		}

		newGame := &models.Game{
			ID:         gameID,
			Title:      filepath.Base(p),
			FolderPath: p,
			Languages:  []string{}, // Инициализируем пустые массивы
			Images:     []string{},
			ExecPath:   autoFindExecutable(p),
			AddedAt:    time.Now().Unix(),
		}

		if err := a.repo.SaveGame(a.ctx, newGame); err == nil {
			fmt.Printf("✅ Успешно добавлено: %s\n", p)
			addedCount++
		} else {
			fmt.Printf("❌ Ошибка сохранения в БД: %v\n", err)
		}
	}

	return addedCount, nil
}

// AddSingleGameManual вызывает диалог выбора папки и добавляет её как игру
func (a *App) AddSingleGameManual() error {
	options := runtime.OpenDialogOptions{
		Title: "Выберите папку с игрой",
	}
	// Открываем диалог
	path, err := runtime.OpenDirectoryDialog(a.ctx, options)
	if err != nil {
		return err
	}
	if path == "" {
		return nil // Пользователь закрыл окно
	}

	// Вызываем наш же метод обработки массива путей
	added, err := a.AddGamesFromDrop([]string{path})
	if err != nil {
		return err
	}
	if added == 0 {
		return fmt.Errorf("Игра уже есть в библиотеке")
	}
	return nil
}

// SelectScreenshots открывает диалог для выбора нескольких картинок
func (a *App) SelectScreenshots() ([]string, error) {
	options := runtime.OpenDialogOptions{
		Title: "Выберите скриншоты",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Изображения",
				Pattern:     "*.jpg;*.jpeg;*.png;*.webp;*.gif",
			},
		},
	}
	// OpenMultipleFilesDialog позволяет выбрать сразу несколько файлов!
	return runtime.OpenMultipleFilesDialog(a.ctx, options)
}

// CopyScreenshotToData копирует скриншот и возвращает новый локальный путь
func (a *App) CopyScreenshotToData(gameID string, sourcePath string) (string, error) {
	ext := filepath.Ext(sourcePath)

	// Генерируем уникальное имя файла на основе времени (микросекунды),
	// чтобы при добавлении 5 скриншотов одновременно они не перезаписали друг друга.
	fileName := fmt.Sprintf("%s_scr_%d%s", gameID, time.Now().UnixNano(), ext)
	destPath := filepath.Join(".", "data", "covers", fileName)

	src, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}

	return destPath, nil
}

// SelectExecutable открывает диалог выбора .exe файла, начиная с указанной папки
func (a *App) SelectExecutable(startDir string) (string, error) {
	options := runtime.OpenDialogOptions{
		Title:            "Выберите .exe файл игры",
		DefaultDirectory: startDir, // Открываем сразу папку с игрой!
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Исполняемые файлы (*.exe, *.bat, *.lnk)",
				Pattern:     "*.exe;*.bat;*.lnk",
			},
			{
				DisplayName: "Все файлы (*.*)",
				Pattern:     "*.*",
			},
		},
	}
	// OpenFileDialog возвращает путь к одному выбранному файлу
	return runtime.OpenFileDialog(a.ctx, options)
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
