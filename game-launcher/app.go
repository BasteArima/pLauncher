package main

import (
	"context"
	"encoding/json"
	"fmt"
	"game-launcher/internal/parser"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"game-launcher/internal/db"
	"game-launcher/internal/launcher"
	"game-launcher/internal/models"
	"game-launcher/internal/scanner"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App — это ядро нашего приложения, связывающее все модули
type App struct {
	ctx      context.Context
	mu       sync.Mutex // защищает (ре)инициализацию репозитория при смене папки
	repo     db.GameRepository
	scanner  *scanner.Scanner
	launcher *launcher.Controller
	dataDir  string // абсолютный путь к папке с данными (БД + обложки)
}

// NewApp создает новое приложение
func NewApp() *App {
	return &App{}
}

// startup вызывается при старте приложения (до показа окна).
// БД инициализируется только если путь к данным уже выбран (есть в конфиге).
// Иначе фронтенд покажет окно первого запуска и вызовет ConfigureDataDir.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.launcher = launcher.NewController()

	cfg := loadConfig()
	if cfg.DataDir != "" {
		if err := a.initServices(cfg.DataDir); err != nil {
			fmt.Printf("Ошибка инициализации данных (%s): %v\n", cfg.DataDir, err)
		}
	}
}

// initServices (пере)открывает БД и сканер для указанной папки данных.
// Безопасно вызывать повторно — старое соединение закрывается.
func (a *App) initServices(dataDir string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(abs, "covers"), 0755); err != nil {
		return fmt.Errorf("couldn't create data folder: %w", err)
	}
	// Папка для пользовательских языков (туда можно класть свои <код>.json)
	langDir := filepath.Join(abs, "languages")
	os.MkdirAll(langDir, 0755)
	writeLanguageHelp(langDir) // README + _example.json как готовый шаблон

	repo, err := db.NewSQLiteRepo(filepath.Join(abs, "games.db"))
	if err != nil {
		return fmt.Errorf("database init error: %w", err)
	}

	// Закрываем предыдущее соединение, если оно было
	if old, ok := a.repo.(*db.SQLiteRepo); ok && old != nil {
		old.Close()
	}

	a.repo = repo
	a.scanner = scanner.NewScanner(repo)
	a.dataDir = abs
	return nil
}

// shutdown вызывается при закрытии приложения.
// Размер окна тут НЕ читаем: на этапе закрытия окно уже разрушается и
// runtime.WindowGetSize в Wails 2.12 паникует (деление на ноль при DPI=0).
// Размер сохраняется в реальном времени по ресайзу (см. SaveWindowSize).
func (a *App) shutdown(ctx context.Context) {
	if a.repo != nil {
		// Приводим интерфейс к конкретному типу для вызова специфичного метода Close
		if sqliteRepo, ok := a.repo.(*db.SQLiteRepo); ok {
			sqliteRepo.Close()
		}
	}
}

// =========================================================================
// УПРАВЛЕНИЕ ПАПКОЙ ДАННЫХ
// =========================================================================

// SaveWindowSize сохраняет текущий размер/состояние окна (вызывается с фронта по ресайзу,
// т.к. OnShutdown в режиме разработки срабатывает не всегда).
func (a *App) SaveWindowSize() {
	// На некоторых состояниях окна (свёрнуто/закрывается) Wails может паниковать
	// внутри WindowGetSize — не даём этому уронить процесс.
	defer func() { recover() }()
	maximised := runtime.WindowIsMaximised(a.ctx)
	w, h := runtime.WindowGetSize(a.ctx)
	cfg := loadConfig()
	if !maximised && w > 0 && h > 0 {
		cfg.WindowWidth = w
		cfg.WindowHeight = h
	}
	cfg.WindowMaximised = maximised
	saveConfig(cfg)
}

// IsConfigured сообщает фронтенду, выбрана ли уже папка для данных.
func (a *App) IsConfigured() bool {
	return a.repo != nil && a.dataDir != ""
}

// GetDataDir возвращает текущую папку данных.
func (a *App) GetDataDir() string { return a.dataDir }

// GetDefaultDataDir, GetPortableDataDir, GetDocumentsDataDir — варианты для окна выбора.
func (a *App) GetDefaultDataDir() string   { return defaultDataDir() }
func (a *App) GetPortableDataDir() string  { return portableDataDir() }
func (a *App) GetDocumentsDataDir() string { return documentsDataDir() }

// ConfigureDataDir вызывается при первом запуске: создаёт папку, открывает БД,
// сохраняет путь в конфиг.
func (a *App) ConfigureDataDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path cannot be empty")
	}
	if err := a.initServices(path); err != nil {
		return err
	}
	return saveConfig(appConfig{DataDir: a.dataDir})
}

// ChangeDataDir переносит данные в новую папку и переключается на неё.
func (a *App) ChangeDataDir(newPath string) error {
	if strings.TrimSpace(newPath) == "" {
		return fmt.Errorf("path cannot be empty")
	}
	newAbs, err := filepath.Abs(newPath)
	if err != nil {
		return err
	}
	oldAbs := a.dataDir
	if newAbs == oldAbs {
		return nil
	}

	// Закрываем БД, чтобы снять блокировку файла перед переносом
	a.mu.Lock()
	if old, ok := a.repo.(*db.SQLiteRepo); ok && old != nil {
		old.Close()
	}
	a.repo = nil
	a.mu.Unlock()

	if oldAbs != "" {
		if err := moveDir(oldAbs, newAbs); err != nil {
			// Пытаемся вернуть рабочее состояние на старой папке
			_ = a.initServices(oldAbs)
			return fmt.Errorf("couldn't move data: %w", err)
		}
	}

	if err := a.initServices(newAbs); err != nil {
		return err
	}
	return saveConfig(appConfig{DataDir: a.dataDir})
}

// SelectDataFolder открывает системный диалог выбора папки для данных.
func (a *App) SelectDataFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Выберите папку для файлов лаунчера",
	})
}

// OpenDataDir открывает папку данных в проводнике.
func (a *App) OpenDataDir() error {
	if a.dataDir == "" {
		return fmt.Errorf("data folder is not selected")
	}
	return a.launcher.OpenFolder(a.dataDir)
}

// ClearData удаляет все игры из БД и содержимое папки covers (файлы обложек).
// Сама папка данных и путь к ней сохраняются.
func (a *App) ClearData() error {
	if a.repo == nil {
		return fmt.Errorf("data is not initialized")
	}
	if err := a.repo.DeleteAllGames(a.ctx); err != nil {
		return err
	}
	coversDir := filepath.Join(a.dataDir, "covers")
	if err := os.RemoveAll(coversDir); err != nil {
		return err
	}
	return os.MkdirAll(coversDir, 0755)
}

// mediaRel приводит сохранённый путь обложки/скриншота к виду относительно
// папки данных (со слэшами) — это и есть путь, который отдаётся фронтенду
// и обслуживается по URL /media/<путь>.
func (a *App) mediaRel(p string) string {
	if p == "" {
		return ""
	}
	p = filepath.ToSlash(p)
	d := filepath.ToSlash(a.dataDir)
	// Абсолютный путь внутри папки данных -> срезаем префикс
	if d != "" && strings.HasPrefix(strings.ToLower(p), strings.ToLower(d)+"/") {
		return p[len(d)+1:]
	}
	// Легаси: старые записи вида "data/covers/..."
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "data/")
	return p
}

// normalizeGames переводит пути медиа всех игр в относительный вид для фронтенда.
func (a *App) normalizeGames(games []*models.Game) []*models.Game {
	for _, g := range games {
		g.CoverPath = a.mediaRel(g.CoverPath)
		for i, img := range g.Images {
			g.Images[i] = a.mediaRel(img)
		}
	}
	return games
}

// =========================================================================
// ЭКСПОРТИРУЕМЫЕ МЕТОДЫ (Они автоматически станут доступны в JavaScript)
// =========================================================================

// GetGames возвращает список всех игр для отрисовки в UI
func (a *App) GetGames() ([]*models.Game, error) {
	if a.repo == nil {
		return []*models.Game{}, nil
	}
	games, err := a.repo.GetAllGames(a.ctx)
	if err != nil {
		return nil, err
	}
	return a.normalizeGames(games), nil
}

// ScanLocalFolder сканирует верхний слой подпапок выбранной папки и добавляет
// каждую как игру (с авто-поиском .exe и подхватом старого data.json, если он есть).
func (a *App) ScanLocalFolder(rootPath string) (int, error) {
	if a.scanner == nil {
		return 0, fmt.Errorf("data folder is not selected yet")
	}
	return a.scanner.ScanFolder(a.ctx, rootPath)
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
	// onExit вызовется при завершении процесса игры — допишем время в БД
	return a.launcher.LaunchGame(exePath, folderPath, func(minutes int) {
		a.addPlaytime(gameID, minutes)
	})
}

// addPlaytime прибавляет минуты к времени в игре и уведомляет UI событием.
func (a *App) addPlaytime(gameID string, minutes int) {
	if a.repo == nil || minutes <= 0 {
		return
	}
	games, err := a.repo.GetAllGames(a.ctx)
	if err != nil {
		return
	}
	for _, g := range games {
		if g.ID == gameID {
			g.TimePlayed += minutes
			if err := a.repo.SaveGame(a.ctx, g); err == nil {
				runtime.EventsEmit(a.ctx, "games-updated")
			}
			return
		}
	}
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
		return fmt.Errorf("game with ID %s not found in database", gameID)
	}

	// 2. Инициализируем парсер
	p, err := parser.GetParser(url, "")
	if err != nil {
		return err
	}

	// 3. Скачиваем данные в подпапку обложек внутри папки данных
	saveDir := filepath.Join(a.dataDir, "covers", targetGame.ID)
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
	if len(parsedData.Languages) > 0 {
		targetGame.Languages = parsedData.Languages
	}
	if len(parsedData.Tags) > 0 {
		targetGame.Tags = parsedData.Tags
	}
	if parsedData.CoverPath != "" {
		targetGame.CoverPath = a.mediaRel(parsedData.CoverPath)
	}
	if len(parsedData.Images) > 0 {
		rel := make([]string, 0, len(parsedData.Images))
		for _, img := range parsedData.Images {
			rel = append(rel, a.mediaRel(img))
		}
		targetGame.Images = rel
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

// GetScanPaths возвращает список папок для сканирования (с миграцией старого scan_path).
func (a *App) GetScanPaths() ([]string, error) {
	if a.repo == nil {
		return []string{}, nil
	}
	raw, _ := a.repo.GetSetting(a.ctx, "scan_paths")
	var paths []string
	if raw != "" {
		json.Unmarshal([]byte(raw), &paths)
	}
	// Миграция: одиночный scan_path -> список
	if len(paths) == 0 {
		if old, _ := a.repo.GetSetting(a.ctx, "scan_path"); old != "" {
			paths = []string{old}
			a.saveScanPaths(paths)
		}
	}
	if paths == nil {
		paths = []string{}
	}
	return paths, nil
}

func (a *App) saveScanPaths(paths []string) error {
	data, _ := json.Marshal(paths)
	return a.repo.SetSetting(a.ctx, "scan_paths", string(data))
}

// AddScanPath добавляет папку в список (без дубликатов).
func (a *App) AddScanPath(path string) error {
	if a.repo == nil {
		return fmt.Errorf("data folder is not selected yet")
	}
	if strings.TrimSpace(path) == "" {
		return nil
	}
	paths, _ := a.GetScanPaths()
	norm := scanner.NormalizePath(path)
	for _, p := range paths {
		if scanner.NormalizePath(p) == norm {
			return nil // уже есть
		}
	}
	return a.saveScanPaths(append(paths, path))
}

// RemoveScanPath убирает папку из списка (игры в библиотеке остаются).
func (a *App) RemoveScanPath(path string) error {
	if a.repo == nil {
		return nil
	}
	paths, _ := a.GetScanPaths()
	out := make([]string, 0, len(paths))
	norm := scanner.NormalizePath(path)
	for _, p := range paths {
		if scanner.NormalizePath(p) != norm {
			out = append(out, p)
		}
	}
	return a.saveScanPaths(out)
}

// ScanAllFolders сканирует все сохранённые папки, возвращает число добавленных игр.
func (a *App) ScanAllFolders() (int, error) {
	if a.scanner == nil {
		return 0, fmt.Errorf("data folder is not selected yet")
	}
	paths, _ := a.GetScanPaths()
	total := 0
	for _, p := range paths {
		n, err := a.scanner.ScanFolder(a.ctx, p)
		if err != nil {
			fmt.Printf("Ошибка сканирования %s: %v\n", p, err)
			continue
		}
		total += n
	}
	return total, nil
}

// GetCustomLocales читает пользовательские языки из <dataDir>/languages/*.json.
// Имя файла = код языка (например de.json -> "de"). Внутри — плоский словарь
// ключ->перевод, плюс ключ "__name" с названием языка для меню.
func (a *App) GetCustomLocales() (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	if a.dataDir == "" {
		return out, nil
	}
	dir := filepath.Join(a.dataDir, "languages")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out, nil // папки ещё нет — ничего страшного
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), "_") || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var dict map[string]string
		if json.Unmarshal(data, &dict) != nil {
			continue
		}
		code := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		out[code] = dict
	}
	return out, nil
}

// OpenLanguagesFolder открывает папку с пользовательскими языками.
func (a *App) OpenLanguagesFolder() error {
	if a.dataDir == "" {
		return fmt.Errorf("data folder is not selected")
	}
	dir := filepath.Join(a.dataDir, "languages")
	os.MkdirAll(dir, 0755)
	return a.launcher.OpenFolder(dir)
}

// GetSupportedSources возвращает список поддерживаемых парсеров для показа в UI.
func (a *App) GetSupportedSources() []parser.Source {
	return parser.SupportedSources()
}

// GetCollections возвращает сохранённые коллекции (хранятся JSON-ом в settings).
func (a *App) GetCollections() ([]models.Collection, error) {
	if a.repo == nil {
		return []models.Collection{}, nil
	}
	raw, err := a.repo.GetSetting(a.ctx, "collections")
	if err != nil || raw == "" {
		return []models.Collection{}, nil
	}
	var cols []models.Collection
	if err := json.Unmarshal([]byte(raw), &cols); err != nil {
		return []models.Collection{}, nil
	}
	return cols, nil
}

// SaveCollections сохраняет весь набор коллекций.
func (a *App) SaveCollections(cols []models.Collection) error {
	if a.repo == nil {
		return fmt.Errorf("data folder is not selected yet")
	}
	data, _ := json.Marshal(cols)
	return a.repo.SetSetting(a.ctx, "collections", string(data))
}

// SearchGames вызывает поиск по базе данных. Если запрос пустой, отдает все игры.
func (a *App) SearchGames(query string) ([]*models.Game, error) {
	if a.repo == nil {
		return []*models.Game{}, nil
	}
	var (
		games []*models.Game
		err   error
	)
	if query == "" {
		games, err = a.repo.GetAllGames(a.ctx)
	} else {
		games, err = a.repo.SearchGames(a.ctx, query)
	}
	if err != nil {
		return nil, err
	}
	return a.normalizeGames(games), nil
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

// CopyCoverToData копирует выбранную обложку в <dataDir>/covers и возвращает
// путь ОТНОСИТЕЛЬНО папки данных (вида "covers/<имя>") для хранения в БД.
func (a *App) CopyCoverToData(gameID string, sourcePath string) (string, error) {
	ext := filepath.Ext(sourcePath)
	if ext == "" {
		ext = ".jpg"
	}
	fileName := fmt.Sprintf("%s_cover_%d%s", gameID, time.Now().UnixNano(), ext)
	return a.copyIntoCovers(sourcePath, fileName)
}

// copyIntoCovers копирует файл в <dataDir>/covers/<fileName>, возвращает
// относительный путь "covers/<fileName>".
func (a *App) copyIntoCovers(sourcePath, fileName string) (string, error) {
	if a.dataDir == "" {
		return "", fmt.Errorf("data folder is not selected")
	}
	coversDir := filepath.Join(a.dataDir, "covers")
	if err := os.MkdirAll(coversDir, 0755); err != nil {
		return "", fmt.Errorf("couldn't create covers folder: %w", err)
	}
	destPath := filepath.Join(coversDir, fileName)

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

	return "covers/" + fileName, nil
}

// AddGamesFromDrop принимает пути перетаскиваемых папок и добавляет их в БД, если их там нет
func (a *App) AddGamesFromDrop(paths []string) (int, error) {
	if a.repo == nil {
		return 0, fmt.Errorf("data folder is not selected yet")
	}
	games, err := a.repo.GetAllGames(a.ctx)
	if err != nil {
		return 0, err
	}

	// Дедуп по нормализованному пути (а не по ID), чтобы переезд папки не плодил дубликаты.
	existingPaths := make(map[string]bool)
	for _, g := range games {
		existingPaths[scanner.NormalizePath(g.FolderPath)] = true
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

		if existingPaths[scanner.NormalizePath(p)] {
			fmt.Printf("⚠️ Пропуск: Игра уже существует в БД: %s\n", p)
			continue
		}

		newGame := &models.Game{
			ID:         scanner.NewID(),
			Title:      filepath.Base(p),
			FolderPath: p,
			Languages:  []string{}, // Инициализируем пустые массивы
			Images:     []string{},
			ExecPath:   scanner.FindBestExecutable(p),
			Engine:     scanner.DetectEngine(p),
			AddedAt:    time.Now().Unix(),
		}

		if err := a.repo.SaveGame(a.ctx, newGame); err == nil {
			fmt.Printf("✅ Успешно добавлено: %s\n", p)
			existingPaths[scanner.NormalizePath(p)] = true
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
		return fmt.Errorf("Game is already in the library")
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

// (поиск .exe вынесен в пакет scanner — scanner.FindBestExecutable)

// CopyScreenshotToData копирует скриншот в <dataDir>/covers и возвращает
// относительный путь "covers/<имя>".
func (a *App) CopyScreenshotToData(gameID string, sourcePath string) (string, error) {
	ext := filepath.Ext(sourcePath)
	// Уникальное имя на основе наносекунд, чтобы пакетное добавление не затирало файлы.
	fileName := fmt.Sprintf("%s_scr_%d%s", gameID, time.Now().UnixNano(), ext)
	return a.copyIntoCovers(sourcePath, fileName)
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
