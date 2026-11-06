package models

type PlaySession struct {
	ID              string `json:"id"`
	GameID          string `json:"game_id"`
	StartedAt       int64  `json:"started_at"`
	DurationSeconds int64  `json:"duration_seconds"`
	Interrupted     bool   `json:"interrupted"`
}

type WeekActivity struct {
	Start   int64 `json:"start"`
	End     int64 `json:"end"`
	Seconds int64 `json:"seconds"`
}

type GameSessions struct {
	TrackingSupported bool           `json:"tracking_supported"`
	Count             int            `json:"count"`
	LongestSeconds    int64          `json:"longest_seconds"`
	HistorySince      int64          `json:"history_since"`
	Weeks             []WeekActivity `json:"weeks"`
	Latest            []PlaySession  `json:"latest"`
}

type GameTime struct {
	GameID  string `json:"game_id"`
	Seconds int64  `json:"seconds"`
}

type NamedCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type DiskStats struct {
	Bytes    int64 `json:"bytes"`
	Measured int   `json:"measured"`
	Unknown  int   `json:"unknown"`
	Missing  int   `json:"missing"`
}

type LibraryStats struct {
	AllTimeSeconds int64          `json:"all_time_seconds"`
	MonthSeconds   int64          `json:"month_seconds"`
	WeekSeconds    int64          `json:"week_seconds"`
	HistorySince   int64          `json:"history_since"`
	Weeks          []WeekActivity `json:"weeks"`
	TopGames       []GameTime     `json:"top_games"`
	Statuses       []NamedCount   `json:"statuses"`
	Engines        []NamedCount   `json:"engines"`
	Disk           DiskStats      `json:"disk"`
	Updates        int            `json:"updates"`
	NoCover        int            `json:"no_cover"`
	NoSource       int            `json:"no_source"`
}
