package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"game-launcher/internal/launchfiles"
	"game-launcher/internal/models"

	_ "modernc.org/sqlite" // Анонимный импорт для регистрации драйвера
)

// GameRepository описывает контракт для работы с БД.
// Использование интерфейсов позволяет нам легко тестировать код в будущем.
type GameRepository interface {
	SaveGame(ctx context.Context, g *models.Game) error
	GetAllGames(ctx context.Context) ([]*models.Game, error)
	DeleteGame(ctx context.Context, id string) error
	DeleteAllGames(ctx context.Context) error
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	SearchGames(ctx context.Context, searchQuery string) ([]*models.Game, error)
	SetGameSize(ctx context.Context, id string, size, checkedAt int64) error
	MarkLaunched(context.Context, string, int64) error
	SaveRelink(context.Context, *models.Game, string) error
	ReadSessionSnapshot(context.Context, []string) (*SessionSnapshot, error)
	SessionRevision(context.Context) (int64, string, error)
}

// SQLiteRepo - реализация GameRepository для SQLite
type SQLiteRepo struct {
	db *sql.DB
}

// NewSQLiteRepo создает новое подключение к БД и накатывает миграции.
func NewSQLiteRepo(dbPath string) (*SQLiteRepo, error) {
	// Убедимся, что папка для БД существует
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("database directory creation error: %w", err)
	}

	// Открываем подключение
	// busy_timeout: фоновые задачи (подсчёт размеров папок) пишут в БД параллельно с UI;
	// без таймаута конкурирующая запись сразу падает с SQLITE_BUSY, а так — ждёт до 5 с.
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("database open error: %w", err)
	}

	// Проверяем пинг, чтобы убедиться, что файл реально доступен
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database ping error: %w", err)
	}

	repo := &SQLiteRepo{db: db}

	// Инициализируем схему (таблицы)
	if err := repo.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return repo, nil
}

// Close закрывает соединение (вызовем при выходе из приложения)
func (r *SQLiteRepo) Close() error {
	return r.db.Close()
}

// BackupTo сохраняет согласованный снимок БД в новый файл dest (VACUUM INTO),
// не закрывая текущее соединение. Файл dest не должен существовать.
func (r *SQLiteRepo) BackupTo(ctx context.Context, dest string) error {
	_, err := r.db.ExecContext(ctx, "VACUUM INTO ?", dest)
	return err
}

func (r *SQLiteRepo) initSchema() error {
	// Существующая таблица игр
	queryGames := `
	CREATE TABLE IF NOT EXISTS games (
		id TEXT PRIMARY KEY,
		title TEXT,
		description TEXT,
		version TEXT,
		languages TEXT,
		cover_path TEXT,
		images TEXT,
		exec_path TEXT,
		folder_path TEXT,
		time_played INTEGER
	);`

	if _, err := r.db.Exec(queryGames); err != nil {
		return fmt.Errorf("games table creation error: %w", err)
	}

	// НОВАЯ таблица для настроек (ключ-значение)
	querySettings := `
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT
	);`

	if _, err := r.db.Exec(querySettings); err != nil {
		return fmt.Errorf("settings table creation error: %w", err)
	}

	// Накатываем новые колонки (если их еще нет)
	r.db.Exec(`ALTER TABLE games ADD COLUMN added_at INTEGER DEFAULT 0;`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN last_launched_at INTEGER DEFAULT 0;`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN tags TEXT DEFAULT '';`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN favorite INTEGER DEFAULT 0;`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN cover_fit TEXT DEFAULT '';`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN cover_pos TEXT DEFAULT '';`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN author TEXT DEFAULT '';`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN engine TEXT DEFAULT '';`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN sources TEXT DEFAULT '';`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN primary_source TEXT DEFAULT '';`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN update_available INTEGER DEFAULT 0;`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN update_version TEXT DEFAULT '';`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN update_source TEXT DEFAULT '';`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN last_checked_at INTEGER DEFAULT 0;`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN size_bytes INTEGER DEFAULT 0;`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN size_checked_at INTEGER DEFAULT 0;`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN status TEXT DEFAULT '';`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN rating INTEGER DEFAULT 0;`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN notes TEXT DEFAULT '';`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN cover_zoom REAL DEFAULT 0;`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN cover_pos_tall TEXT DEFAULT '';`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN cover_zoom_tall REAL DEFAULT 0;`)

	r.db.Exec(`ALTER TABLE games ADD COLUMN notes_updated_at INTEGER DEFAULT 0;`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN update_found_at INTEGER DEFAULT 0;`)

	r.db.Exec(`ALTER TABLE games ADD COLUMN exec_label TEXT DEFAULT '';`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN launch_files TEXT DEFAULT '[]';`)
	return r.initSessions()
}

// scanGames читает строки результата в слайс игр (общая логика для Get/Search).
func scanGames(rows *sql.Rows) ([]*models.Game, error) {
	var games []*models.Game
	for rows.Next() {
		var g models.Game
		var langsJSON, imagesJSON, tagsJSON, sourcesJSON, launchFilesJSON string
		var favorite, updateAvailable int

		err := rows.Scan(
			&g.ID, &g.Title, &g.Description, &g.Version, &langsJSON,
			&g.CoverPath, &imagesJSON, &tagsJSON, &g.ExecPath, &g.FolderPath,
			&favorite, &g.TimePlayed, &g.AddedAt, &g.LastLaunchedAt, &g.CoverFit, &g.CoverPos,
			&g.Author, &g.Engine,
			&sourcesJSON, &g.PrimarySource, &updateAvailable, &g.UpdateVersion, &g.UpdateSource, &g.LastCheckedAt,
			&g.SizeBytes, &g.SizeCheckedAt,
			&g.Status, &g.Rating, &g.Notes, &g.CoverZoom, &g.CoverPosTall, &g.CoverZoomTall, &g.NotesUpdatedAt, &g.UpdateFoundAt, &g.ExecLabel, &launchFilesJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("row read error: %w", err)
		}

		json.Unmarshal([]byte(langsJSON), &g.Languages)
		json.Unmarshal([]byte(imagesJSON), &g.Images)
		json.Unmarshal([]byte(tagsJSON), &g.Tags)
		json.Unmarshal([]byte(sourcesJSON), &g.Sources)
		g.LaunchFiles = []models.LaunchFile{}
		if launchFilesJSON != "" && launchFilesJSON != "null" {
			if err := json.Unmarshal([]byte(launchFilesJSON), &g.LaunchFiles); err != nil {
				return nil, fmt.Errorf("invalid launch files for %s: %w", g.ID, err)
			}
		}
		g.Favorite = favorite != 0
		g.UpdateAvailable = updateAvailable != 0

		games = append(games, &g)
	}
	return games, rows.Err()
}

// gameColumns — единый список колонок для SELECT (порядок важен для scanGames).
const gameColumns = `id, title, description, version, languages, cover_path, images, tags, exec_path, folder_path, favorite, time_played, added_at, last_launched_at, cover_fit, cover_pos, author, engine, sources, primary_source, update_available, update_version, update_source, last_checked_at, size_bytes, size_checked_at, COALESCE(status, ''), COALESCE(rating, 0), COALESCE(notes, ''), COALESCE(cover_zoom, 0), COALESCE(cover_pos_tall, ''), COALESCE(cover_zoom_tall, 0), COALESCE(notes_updated_at, 0), COALESCE(update_found_at, 0), COALESCE(exec_label, ''), COALESCE(launch_files, '[]')`

// SaveGame добавляет новую игру или обновляет существующую (UPSERT)
func (r *SQLiteRepo) SaveGame(ctx context.Context, g *models.Game) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Acquire the writer lock before reading capacity/legacy fields.
	if _, err := tx.ExecContext(ctx, `UPDATE games SET id=id WHERE id=?`, g.ID); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+gameColumns+` FROM games WHERE id=?`, g.ID)
	if err != nil {
		return err
	}
	oldGames, err := scanGames(rows)
	rows.Close()
	if err != nil {
		return err
	}
	var old *models.Game
	if len(oldGames) != 0 {
		old = oldGames[0]
	}
	// Older clients omit the new array. An explicit [] clears it.
	if old != nil && g.LaunchFiles == nil {
		g.LaunchFiles = old.LaunchFiles
		if g.ExecPath == old.ExecPath && g.ExecLabel == "" {
			g.ExecLabel = old.ExecLabel
		}
	}
	if err := launchfiles.Validate(g, old, true); err != nil {
		return err
	}
	if err := saveGame(ctx, tx, g); err != nil {
		return err
	}
	return tx.Commit()
}

type gameWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func saveGame(ctx context.Context, writer gameWriter, g *models.Game) error {
	if err := launchfiles.Validate(g, nil, false); err != nil {
		return err
	}
	if g.ExecPath == "" {
		g.ExecLabel = ""
	}
	if g.LaunchFiles == nil {
		g.LaunchFiles = []models.LaunchFile{}
	}
	launchJSON, _ := json.Marshal(g.LaunchFiles)
	// SQLite не умеет хранить массивы (слайсы), поэтому сериализуем их в JSON.
	// Это стандартный подход, который не нагружает сборщик мусора (GC).
	langsJSON, _ := json.Marshal(g.Languages)
	imagesJSON, _ := json.Marshal(g.Images)
	tagsJSON, _ := json.Marshal(g.Tags)
	sourcesJSON, _ := json.Marshal(g.Sources)
	favorite := 0
	if g.Favorite {
		favorite = 1
	}
	updateAvailable := 0
	if g.UpdateAvailable {
		updateAvailable = 1
	}

	// Дату заметки определяем атомарно по сохранённому тексту, игнорируя старую дату из UI.
	query := `
	INSERT INTO games (id, title, description, version, languages, cover_path, images, tags, exec_path, folder_path, favorite, time_played, added_at, last_launched_at, cover_fit, cover_pos, author, engine, sources, primary_source, update_available, update_version, update_source, last_checked_at, size_bytes, size_checked_at, status, rating, notes, cover_zoom, cover_pos_tall, cover_zoom_tall, notes_updated_at, update_found_at, exec_label, launch_files)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CASE WHEN ? = '' THEN 0 ELSE COALESCE(NULLIF(?, 0), unixepoch()) END, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		title=excluded.title,
		description=excluded.description,
		version=excluded.version,
		languages=excluded.languages,
		cover_path=excluded.cover_path,
		images=excluded.images,
		tags=excluded.tags,
		exec_path=excluded.exec_path,
		exec_label=excluded.exec_label,
		launch_files=excluded.launch_files,
		folder_path=excluded.folder_path,
		favorite=excluded.favorite,
		cover_fit=excluded.cover_fit,
		cover_pos=excluded.cover_pos,
		author=excluded.author,
		engine=excluded.engine,
		sources=excluded.sources,
		primary_source=excluded.primary_source,
		update_available=excluded.update_available,
		update_version=excluded.update_version,
		update_source=excluded.update_source,
		last_checked_at=excluded.last_checked_at,
		status=excluded.status,
		rating=excluded.rating,
		notes_updated_at=CASE WHEN COALESCE(games.notes, '') != excluded.notes THEN unixepoch() ELSE COALESCE(games.notes_updated_at, 0) END,
		notes=excluded.notes,
		update_found_at=CASE WHEN excluded.update_available = 0 THEN 0 ELSE excluded.update_found_at END,
		cover_zoom=excluded.cover_zoom,
		cover_pos_tall=excluded.cover_pos_tall,
		cover_zoom_tall=excluded.cover_zoom_tall;
	`
	// Обрати внимание: added_at не обновляется при конфликте, чтобы сохранить дату первого добавления!
	// size_bytes/size_checked_at тоже: их пишет только SetGameSize (фоновый подсчёт), чтобы
	// сохранение игры из UI со старым размером не затирало свежий.

	// Playtime and last launch are owned by session tracking, never by stale UI saves.
	_, err := writer.ExecContext(ctx, query,
		g.ID, g.Title, g.Description, g.Version, string(langsJSON),
		g.CoverPath, string(imagesJSON), string(tagsJSON), g.ExecPath, g.FolderPath, favorite, g.TimePlayed, g.AddedAt, g.LastLaunchedAt, g.CoverFit, g.CoverPos,
		g.Author, g.Engine,
		string(sourcesJSON), g.PrimarySource, updateAvailable, g.UpdateVersion, g.UpdateSource, g.LastCheckedAt,
		g.SizeBytes, g.SizeCheckedAt,
		g.Status, g.Rating, g.Notes, g.CoverZoom, g.CoverPosTall, g.CoverZoomTall, g.Notes, g.NotesUpdatedAt, g.UpdateFoundAt, g.ExecLabel, string(launchJSON),
	)

	if err != nil {
		return fmt.Errorf("error saving game %s: %w", g.Title, err)
	}
	return nil
}

// SetGameSize обновляет только размер папки игры и время его подсчёта.
func (r *SQLiteRepo) SetGameSize(ctx context.Context, id string, size, checkedAt int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE games SET size_bytes = ?, size_checked_at = ? WHERE id = ?`, size, checkedAt, id)
	return err
}

// GetAllGames извлекает все игры из базы
func (r *SQLiteRepo) GetAllGames(ctx context.Context) ([]*models.Game, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+gameColumns+` FROM games`)
	if err != nil {
		return nil, fmt.Errorf("games query error: %w", err)
	}
	defer rows.Close() // Важно закрывать rows для предотвращения утечек памяти

	return scanGames(rows)
}

// DeleteGame удаляет игру из базы данных по её ID
func (r *SQLiteRepo) DeleteGame(ctx context.Context, id string) error {
	query := `DELETE FROM games WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting game: %w", err)
	}
	return nil
}

// DeleteAllGames очищает таблицу игр (используется при «Очистить данные»)
func (r *SQLiteRepo) DeleteAllGames(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM games`); err != nil {
		return fmt.Errorf("error clearing games table: %w", err)
	}
	return nil
}

// GetSetting получает настройку по ключу
func (r *SQLiteRepo) GetSetting(ctx context.Context, key string) (string, error) {
	var val string
	query := `SELECT value FROM settings WHERE key = ?`
	err := r.db.QueryRowContext(ctx, query, key).Scan(&val)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // Если настройки еще нет, возвращаем пустую строку без ошибки
		}
		return "", err
	}
	return val, nil
}

// SetSetting сохраняет или обновляет настройку
func (r *SQLiteRepo) SetSetting(ctx context.Context, key, value string) error {
	query := `
	INSERT INTO settings (key, value) VALUES (?, ?)
	ON CONFLICT(key) DO UPDATE SET value=excluded.value;
	`
	_, err := r.db.ExecContext(ctx, query, key, value)
	return err
}

// SearchGames ищет игры, в названии которых есть совпадения с запросом
func (r *SQLiteRepo) SearchGames(ctx context.Context, searchQuery string) ([]*models.Game, error) {
	// Используем оператор LIKE и оборачиваем запрос в знаки процента (поиск подстроки)
	query := `SELECT ` + gameColumns + ` FROM games WHERE title LIKE ? ORDER BY title ASC`

	searchTerm := "%" + searchQuery + "%"
	rows, err := r.db.QueryContext(ctx, query, searchTerm)
	if err != nil {
		return nil, fmt.Errorf("games search error: %w", err)
	}
	defer rows.Close()

	return scanGames(rows)
}
