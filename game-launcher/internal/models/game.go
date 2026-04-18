package models

// Game описывает метаданные игры для нашего лаунчера
type Game struct {
	ID          string   `json:"id"`          // Уникальный идентификатор (например, хеш названия)
	Title       string   `json:"title"`       // Название
	Description string   `json:"description"` // Описание
	Version     string   `json:"version"`     // Версия игры
	Languages   []string `json:"languages"`   // Поддерживаемые языки
	CoverPath   string   `json:"cover_path"`  // Локальный путь к обложке
	Images      []string `json:"images"`      // Локальные пути к скриншотам
	ExecPath    string   `json:"exec_path"`   // Путь к исполняемому файлу (.exe)
	FolderPath  string   `json:"folder_path"` // Путь к папке с игрой
	TimePlayed  int      `json:"time_played"` // Время в игре (в минутах)
}
