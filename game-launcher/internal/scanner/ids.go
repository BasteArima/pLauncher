package scanner

import (
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"strings"
)

// NewID генерирует стабильный случайный идентификатор игры.
// В отличие от хэша пути, он НЕ меняется при перемещении/переименовании папки,
// поэтому метаданные (обложка, описание, .exe) не теряются. Совпадение игры
// при сканировании определяется по пути к папке (NormalizePath), а не по ID.
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand практически не падает; на всякий случай — детерминированный фолбэк
		return hex.EncodeToString(b)
	}
	return hex.EncodeToString(b)
}

// NormalizePath приводит путь к канонической форме для сравнения папок:
// абсолютный, очищенный, в нижнем регистре (ФС Windows регистронезависима).
// Используется ТОЛЬКО как ключ дедупликации — в БД хранится исходный путь.
func NormalizePath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	return strings.ToLower(filepath.Clean(abs))
}
