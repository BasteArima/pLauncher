package models

// Collection — пользовательская коллекция игр (как в Steam).
// type:
//   "manual"  — ручной список игр (GameIDs)
//   "dynamic" — динамическая, членство вычисляется по правилам (пока — по тегам Tags)
type Collection struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	GameIDs []string `json:"game_ids"` // для ручных коллекций
	Tags    []string `json:"tags"`     // для динамических: игра входит, если есть любой из тегов
}
