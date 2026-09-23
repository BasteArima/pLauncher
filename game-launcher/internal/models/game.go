package models

// GameSource — ссылка на игру на конкретной площадке и последняя увиденная там версия.
// LastVersion — база для сравнения при проверке обновлений (версия, которую мы
// «приняли»/увидели последней именно с этой площадки; форматы версий у площадок разные,
// поэтому сравниваем строки в рамках одной площадки, а не между ними).
type GameSource struct {
	Source      string `json:"source"`       // имя площадки (как в parser.SupportedSources)
	URL         string `json:"url"`          // ссылка на страницу игры
	LastVersion string `json:"last_version"` // последняя принятая/увиденная версия с этой площадки
}

// Game описывает метаданные игры для нашего лаунчера
type Game struct {
	ID             string   `json:"id"`               // Уникальный идентификатор (например, хеш названия)
	Title          string   `json:"title"`            // Название
	Description    string   `json:"description"`      // Описание
	Version        string   `json:"version"`          // Версия игры
	Author         string   `json:"author"`           // Разработчик/издатель
	Engine         string   `json:"engine"`           // Движок (Ren'Py, Unity, RPG Maker и т.п.)
	Languages      []string `json:"languages"`        // Поддерживаемые языки
	CoverPath      string   `json:"cover_path"`       // Локальный путь к обложке
	CoverFit       string   `json:"cover_fit"`        // object-fit обложки: cover|contain|fill (пусто = cover)
	CoverPos       string   `json:"cover_pos"`        // object-position обложки, напр. "50% 20%" (пусто = центр)
	Images         []string `json:"images"`           // Локальные пути к скриншотам
	Tags           []string `json:"tags"`             // Жанры/теги (RPG, Ren'Py, и т.п.)
	ExecPath       string   `json:"exec_path"`        // Путь к исполняемому файлу (.exe)
	FolderPath     string   `json:"folder_path"`      // Путь к папке с игрой
	Favorite       bool     `json:"favorite"`         // В избранном
	TimePlayed     int      `json:"time_played"`      // Время в игре (в минутах)
	AddedAt        int64    `json:"added_at"`         // UNIX-время добавления
	LastLaunchedAt int64    `json:"last_launched_at"` // UNIX-время последнего запуска

	// Источники и проверка обновлений
	Sources         []GameSource `json:"sources"`          // ссылки по площадкам {площадка→url}
	PrimarySource   string       `json:"primary_source"`   // площадка для авто-проверки обновлений
	UpdateAvailable bool         `json:"update_available"` // найдено обновление хотя бы на одной площадке
	UpdateVersion   string       `json:"update_version"`   // найденная новая версия (для бейджа)
	UpdateSource    string       `json:"update_source"`    // площадка, где найдено обновление
	LastCheckedAt   int64        `json:"last_checked_at"`  // UNIX-время последней проверки обновлений

	// Размер папки игры на диске (считается в фоне, пишется только через SetGameSize)
	SizeBytes     int64 `json:"size_bytes"`
	SizeCheckedAt int64 `json:"size_checked_at"` // UNIX-время подсчёта (0 — ещё не считали)

	// Вычисляемое поле (в БД не хранится): папки игры нет на диске (перемещена/удалена).
	FolderMissing bool `json:"folder_missing"`
}
