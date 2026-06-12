package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

// maxScanDepth ограничивает глубину поиска .exe, чтобы не нырять в ассеты.
const maxScanDepth = 4

// hardSkip — подстроки в имени файла, который точно НЕ является игрой.
var hardSkip = []string{
	"unins", "uninst", "crash", "unitycrashhandler", "ue4prereqsetup",
	"ueprereqsetup", "vcredist", "vc_redist", "dxsetup", "dxwebsetup",
	"directx", "dotnetfx", "ndp", "oalinst", "python", "pythonw",
	"ffmpeg", "node", "nw_elf", "notification_helper", "werfault",
}

// softPenalty — подстроки, которые делают файл маловероятным запускающим.
var softPenalty = []string{
	"setup", "install", "config", "settings", "editor", "server",
	"tool", "patch", "update", "redist", "helper", "diagnostic", "report",
}

// preferredNames — типовые имена запускающего файла.
var preferredNames = map[string]bool{
	"game": true, "start": true, "run": true, "play": true,
	"launch": true, "launcher": true,
}

// engineSubdirs — папки движков; .exe внутри них почти никогда не главный.
var engineSubdirs = map[string]bool{
	"lib": true, "engine": true, "binaries": true, "redist": true,
	"_commonredist": true, "tools": true, "support": true, "renpy": true,
}

// FindBestExecutable ищет наиболее вероятный запускающий .exe в папке игры.
// Кандидаты оцениваются по эвристикам (глубина, имя, совпадение с именем папки),
// возвращается путь с наибольшим баллом. Если ничего не найдено — пустая строка.
func FindBestExecutable(folderPath string) string {
	folderKey := normalizeName(filepath.Base(folderPath))

	bestPath := ""
	bestScore := -1 << 30

	_ = filepath.WalkDir(folderPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // игнорируем недоступные пути
		}

		rel, _ := filepath.Rel(folderPath, path)
		depth := 0
		if rel != "." {
			depth = len(strings.Split(rel, string(os.PathSeparator)))
		}

		if d.IsDir() {
			if depth >= maxScanDepth {
				return filepath.SkipDir
			}
			return nil
		}

		name := strings.ToLower(d.Name())
		if !strings.HasSuffix(name, ".exe") {
			return nil
		}

		base := strings.TrimSuffix(name, ".exe")

		// Полностью исключаем явный системный/служебный мусор
		for _, bad := range hardSkip {
			if strings.Contains(base, bad) {
				return nil
			}
		}

		score := score(base, folderKey, depth, path, folderPath)
		if score > bestScore {
			bestScore = score
			bestPath = path
		}
		return nil
	})

	return bestPath
}

// score оценивает кандидата: больше — вероятнее, что это и есть игра.
func score(base, folderKey string, depth int, path, folderPath string) int {
	s := 100

	// Чем глубже файл — тем хуже (корень папки игры предпочтителен)
	s -= depth * 25

	// Совпадение имени .exe с именем папки — сильный сигнал
	nb := normalizeName(base)
	if nb != "" && nb == folderKey {
		s += 80
	} else if nb != "" && (strings.Contains(folderKey, nb) || strings.Contains(nb, folderKey)) {
		s += 40
	}

	// Типовые имена запускающего файла
	if preferredNames[base] {
		s += 35
	}

	// Маловероятные имена
	for _, bad := range softPenalty {
		if strings.Contains(base, bad) {
			s -= 40
			break
		}
	}

	// .exe внутри папки движка — штраф
	parent := strings.ToLower(filepath.Base(filepath.Dir(path)))
	if engineSubdirs[parent] {
		s -= 30
	}

	// Лёгкий бонус более крупным файлам (главный бинарь обычно не крошечный)
	if info, err := os.Stat(path); err == nil {
		if info.Size() > 5<<20 { // > 5 МБ
			s += 10
		}
		if info.Size() < 256<<10 { // < 256 КБ — вероятно вспомогательный
			s -= 15
		}
	}

	return s
}

// normalizeName убирает всё, кроме букв и цифр, и приводит к нижнему регистру,
// чтобы "My_Game" и "mygame" считались одинаковыми.
func normalizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
