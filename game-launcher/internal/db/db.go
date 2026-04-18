package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"game-launcher/internal/models"

	_ "modernc.org/sqlite" // Анонимный импорт для регистрации драйвера
)

// GameRepository описывает контракт для работы с БД.
// Использование интерфейсов позволяет нам легко тестировать код в будущем.
type GameRepository interface {
	SaveGame(ctx context.Context, g *models.Game) error
	GetAllGames(ctx context.Context) ([]*models.Game, error)
	DeleteGame(ctx context.Context, id string) error
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	SearchGames(ctx context.Context, searchQuery string) ([]*models.Game, error)
}

// SQLiteRepo - реализация GameRepository для SQLite
type SQLiteRepo struct {
	db *sql.DB
}

// NewSQLiteRepo создает новое подключение к БД и накатывает миграции.
func NewSQLiteRepo(dbPath string) (*SQLiteRepo, error) {
	// Убедимся, что папка для БД существует
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("ошибка создания директории БД: %w", err)
	}

	// Открываем подключение
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия БД: %w", err)
	}

	// Проверяем пинг, чтобы убедиться, что файл реально доступен
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ошибка пинга БД: %w", err)
	}

	repo := &SQLiteRepo{db: db}

	// Инициализируем схему (таблицы)
	if err := repo.initSchema(); err != nil {
		return nil, err
	}

	return repo, nil
}

// Close закрывает соединение (вызовем при выходе из приложения)
func (r *SQLiteRepo) Close() error {
	return r.db.Close()
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
		return fmt.Errorf("ошибка создания таблицы games: %w", err)
	}

	// НОВАЯ таблица для настроек (ключ-значение)
	querySettings := `
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT
	);`

	if _, err := r.db.Exec(querySettings); err != nil {
		return fmt.Errorf("ошибка создания таблицы settings: %w", err)
	}

	// Накатываем новые колонки (если их еще нет)
	r.db.Exec(`ALTER TABLE games ADD COLUMN added_at INTEGER DEFAULT 0;`)
	r.db.Exec(`ALTER TABLE games ADD COLUMN last_launched_at INTEGER DEFAULT 0;`)

	return nil
}

// SaveGame добавляет новую игру или обновляет существующую (UPSERT)
func (r *SQLiteRepo) SaveGame(ctx context.Context, g *models.Game) error {
	// SQLite не умеет хранить массивы (слайсы), поэтому сериализуем их в JSON.
	// Это стандартный подход, который не нагружает сборщик мусора (GC).
	langsJSON, _ := json.Marshal(g.Languages)
	imagesJSON, _ := json.Marshal(g.Images)

	query := `
	INSERT INTO games (id, title, description, version, languages, cover_path, images, exec_path, folder_path, time_played, added_at, last_launched_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		title=excluded.title,
		description=excluded.description,
		version=excluded.version,
		languages=excluded.languages,
		cover_path=excluded.cover_path,
		images=excluded.images,
		exec_path=excluded.exec_path,
		folder_path=excluded.folder_path,
		time_played=excluded.time_played,
		last_launched_at=excluded.last_launched_at;
	`
	// Обрати внимание: added_at не обновляется при конфликте, чтобы сохранить дату первого добавления!

	_, err := r.db.ExecContext(ctx, query,
		g.ID, g.Title, g.Description, g.Version, string(langsJSON),
		g.CoverPath, string(imagesJSON), g.ExecPath, g.FolderPath, g.TimePlayed, g.AddedAt, g.LastLaunchedAt,
	)

	if err != nil {
		return fmt.Errorf("ошибка сохранения игры %s: %w", g.Title, err)
	}
	return nil
}

// GetAllGames извлекает все игры из базы
func (r *SQLiteRepo) GetAllGames(ctx context.Context) ([]*models.Game, error) {
	query := `SELECT id, title, description, version, languages, cover_path, images, exec_path, folder_path, time_played, added_at, last_launched_at FROM games`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса игр: %w", err)
	}
	defer rows.Close() // Важно закрывать rows для предотвращения утечек памяти

	var games []*models.Game

	for rows.Next() {
		var g models.Game
		var langsJSON, imagesJSON string

		err := rows.Scan(
			&g.ID, &g.Title, &g.Description, &g.Version, &langsJSON,
			&g.CoverPath, &imagesJSON, &g.ExecPath, &g.FolderPath, &g.TimePlayed, &g.AddedAt, &g.LastLaunchedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения строки: %w", err)
		}

		// Десериализуем JSON обратно в слайсы строк
		json.Unmarshal([]byte(langsJSON), &g.Languages)
		json.Unmarshal([]byte(imagesJSON), &g.Images)

		games = append(games, &g)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return games, nil
}

// DeleteGame удаляет игру из базы данных по её ID
func (r *SQLiteRepo) DeleteGame(ctx context.Context, id string) error {
	query := `DELETE FROM games WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("ошибка при удалении игры: %w", err)
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
	query := `
	SELECT id, title, description, version, languages, cover_path, images, exec_path, folder_path, time_played, added_at, last_launched_at 
	FROM games 
	WHERE title LIKE ? 
	ORDER BY title ASC`

	searchTerm := "%" + searchQuery + "%"
	rows, err := r.db.QueryContext(ctx, query, searchTerm)
	if err != nil {
		return nil, fmt.Errorf("ошибка поиска игр: %w", err)
	}
	defer rows.Close()

	var games []*models.Game
	for rows.Next() {
		var g models.Game
		var langsJSON, imagesJSON string

		err := rows.Scan(
			&g.ID, &g.Title, &g.Description, &g.Version, &langsJSON,
			&g.CoverPath, &imagesJSON, &g.ExecPath, &g.FolderPath, &g.TimePlayed, &g.AddedAt, &g.LastLaunchedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения строки: %w", err)
		}

		json.Unmarshal([]byte(langsJSON), &g.Languages)
		json.Unmarshal([]byte(imagesJSON), &g.Images)

		games = append(games, &g)
	}

	return games, nil
}
