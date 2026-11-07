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

// LaunchFile is an additional local file. An empty Label uses filename-based localization.
type LaunchFile struct {
	Path  string `json:"path"`
	Label string `json:"label"`
}

// Game описывает метаданные игры для нашего лаунчера
type Game struct {
	ID             string       `json:"id"`              // Уникальный идентификатор (например, хеш названия)
	Title          string       `json:"title"`           // Название
	Description    string       `json:"description"`     // Описание
	Version        string       `json:"version"`         // Версия игры
	Author         string       `json:"author"`          // Разработчик/издатель
	Engine         string       `json:"engine"`          // Движок (Ren'Py, Unity, RPG Maker и т.п.)
	Languages      []string     `json:"languages"`       // Поддерживаемые языки
	CoverPath      string       `json:"cover_path"`      // Локальный путь к обложке
	CoverFit       string       `json:"cover_fit"`       // object-fit обложки: cover|contain|fill (пусто = cover)
	CoverPos       string       `json:"cover_pos"`       // object-position обложки, напр. "50% 20%" (пусто = центр)
	CoverZoom      float64      `json:"cover_zoom"`      // приближение обложки в карточках (0 или 1 — без зума, до 3)
	CoverPosTall   string       `json:"cover_pos_tall"`  // своя точка фокуса для вертикальной карточки (пусто = как у широкой)
	CoverZoomTall  float64      `json:"cover_zoom_tall"` // свой зум для вертикальной карточки (действует, только если задана CoverPosTall)
	Images         []string     `json:"images"`          // Локальные пути к скриншотам
	Tags           []string     `json:"tags"`            // Жанры/теги (RPG, Ren'Py, и т.п.)
	ExecPath       string       `json:"exec_path"`       // Путь к исполняемому файлу (.exe)
	ExecLabel      string       `json:"exec_label"`
	LaunchFiles    []LaunchFile `json:"launch_files"`
	FolderPath     string       `json:"folder_path"`      // Путь к папке с игрой
	Favorite       bool         `json:"favorite"`         // В избранном
	TimePlayed     int          `json:"time_played"`      // Время в игре (в минутах)
	AddedAt        int64        `json:"added_at"`         // UNIX-время добавления
	LastLaunchedAt int64        `json:"last_launched_at"` // UNIX-время последнего запуска

	// Личное: статус прохождения, оценка и заметки игрока
	Status string `json:"status"` // "" (не начата) | playing | want | finished | dropped
	Rating int    `json:"rating"` // 0 — без оценки, 1..5 звёзд
	Notes  string `json:"notes"`  // заметки игрока (спойлеры прячутся в интерфейсе)

	NotesUpdatedAt int64 `json:"notes_updated_at"` // UNIX-время изменения текста заметки

	// Источники и проверка обновлений
	Sources         []GameSource `json:"sources"`          // ссылки по площадкам {площадка→url}
	PrimarySource   string       `json:"primary_source"`   // площадка для авто-проверки обновлений
	UpdateAvailable bool         `json:"update_available"` // найдено обновление хотя бы на одной площадке
	UpdateVersion   string       `json:"update_version"`   // найденная новая версия (для бейджа)
	UpdateSource    string       `json:"update_source"`    // площадка, где найдено обновление
	LastCheckedAt   int64        `json:"last_checked_at"`  // UNIX-время последней проверки обновлений

	UpdateFoundAt int64 `json:"update_found_at"` // UNIX-время обнаружения текущего обновления

	// Размер папки игры на диске (считается в фоне, пишется только через SetGameSize)
	SizeBytes     int64 `json:"size_bytes"`
	SizeCheckedAt int64 `json:"size_checked_at"` // UNIX-время подсчёта (0 — ещё не считали)

	// Вычисляемое поле (в БД не хранится): папки игры нет на диске (перемещена/удалена).
	FolderMissing bool `json:"folder_missing"`
	LaunchMissing bool `json:"launch_missing"` // Computed on read, never stored.
}
