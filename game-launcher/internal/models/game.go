package models

// Game описывает метаданные игры для нашего лаунчера
type Game struct {
	ID             string   `json:"id"`               // Уникальный идентификатор (например, хеш названия)
	Title          string   `json:"title"`            // Название
	Description    string   `json:"description"`      // Описание
	Version        string   `json:"version"`          // Версия игры
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
}
