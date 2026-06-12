package parser

import (
	"strings"
)

// Эти помощники разбирают «релизные» заголовки трекеров вида:
//   "Название [версия] (разработчик) [cen] [год, теги] [языки]"
// Такой формат встречается у pornolab (h1.maintitle) и island (поле «Название»).

// langCodes сопоставляет коды/слова языков (в нижнем регистре) с каноническим именем.
// Двухбуквенные коды намеренно не включены — они дают ложные срабатывания на тегах.
var langCodes = map[string]string{
	"rus": "Русский", "russian": "Русский", "русский": "Русский",
	"eng": "English", "english": "English", "английский": "English",
	"jap": "日本語", "jpn": "日本語", "japanese": "日本語", "японский": "日本語",
	"chi": "中文", "chn": "中文", "chinese": "中文", "китайский": "中文",
	"kor": "한국어", "korean": "한국어", "корейский": "한국어",
	"ger": "Deutsch", "deu": "Deutsch", "german": "Deutsch", "немецкий": "Deutsch",
	"fre": "Français", "fra": "Français", "french": "Français", "французский": "Français",
	"spa": "Español", "esp": "Español", "spanish": "Español", "испанский": "Español",
	"ita": "Italiano", "italian": "Italiano", "итальянский": "Italiano",
	"por": "Português", "portuguese": "Português", "португальский": "Português",
}

// stripLeadingGroups убирает ведущие группы в скобках/кавычках, которые на трекерах
// служат префиксами раздачи: "[MOD]", "(Сборник)", "[Mods]" и т.п.
func stripLeadingGroups(runes []rune) []rune {
	for {
		runes = trimLeftSpaceRunes(runes)
		if len(runes) == 0 || (runes[0] != '[' && runes[0] != '(') {
			return runes
		}
		close := ']'
		if runes[0] == '(' {
			close = ')'
		}
		j := -1
		for i := 1; i < len(runes); i++ {
			if runes[i] == close {
				j = i
				break
			}
		}
		if j < 0 {
			return runes // незакрытая скобка — дальше не лезем
		}
		runes = runes[j+1:]
	}
}

func trimLeftSpaceRunes(r []rune) []rune {
	i := 0
	for i < len(r) && (r[i] == ' ' || r[i] == '\t' || r[i] == ' ') {
		i++
	}
	return r[i:]
}

// cleanReleaseTitle вытаскивает чистое название: срезает ведущие префиксы-теги,
// затем берёт текст до первой группы [..] или (..).
func cleanReleaseTitle(s string) string {
	runes := stripLeadingGroups([]rune(s))
	end := len(runes)
	for i, r := range runes {
		if r == '[' || r == '(' {
			end = i
			break
		}
	}
	return cleanText(string(runes[:end]))
}

// versionFromTitle возвращает содержимое первой группы [..] после префиксов —
// на трекерах это, как правило, версия/дата релиза.
func versionFromTitle(s string) string {
	runes := stripLeadingGroups([]rune(s))
	start := -1
	for i, r := range runes {
		if r == '[' {
			start = i + 1
			break
		}
		// если раньше встретили '(' (разработчик) — версии в [] нет
		if r == '(' {
			return ""
		}
	}
	if start < 0 {
		return ""
	}
	for i := start; i < len(runes); i++ {
		if runes[i] == ']' {
			return cleanText(string(runes[start:i]))
		}
	}
	return ""
}

// censorshipTokens — метки цензуры, которые не должны попадать в теги.
var censorshipTokens = map[string]bool{
	"cen": true, "uncen": true, "ptcen": true, "censored": true, "uncensored": true,
}

// singleTokenTags — движки/платформы, которые на трекерах часто стоят отдельной
// группой в скобках (например island: "[Ren'Py]"). Ключ — в нижнем регистре.
var singleTokenTags = map[string]bool{
	"ren'py": true, "renpy": true, "unity": true, "rpg maker": true, "rpgm": true,
	"html": true, "flash": true, "java": true, "unreal": true, "wolf rpg": true,
	"kirikiri": true, "godot": true, "tyranobuilder": true,
	"windows": true, "win": true, "linux": true, "lin": true, "android": true,
	"mac": true, "macos": true, "ios": true,
}

// bracketGroups возвращает содержимое всех групп вида [..] (квадратные скобки).
func bracketGroups(s string) []string {
	var groups []string
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '[' {
			continue
		}
		for j := i + 1; j < len(runes); j++ {
			if runes[j] == ']' {
				groups = append(groups, string(runes[i+1:j]))
				i = j
				break
			}
		}
	}
	return groups
}

// isYear сообщает, является ли токен 4-значным годом (19xx/20xx).
func isYear(t string) bool {
	t = strings.TrimSpace(t)
	if len(t) != 4 {
		return false
	}
	for _, r := range t {
		if r < '0' || r > '9' {
			return false
		}
	}
	return t[0] == '1' || t[0] == '2'
}

func groupHasYear(g string) bool {
	for _, part := range strings.FieldsFunc(g, func(r rune) bool { return r == ',' || r == ' ' }) {
		if isYear(part) {
			return true
		}
	}
	return false
}

// parseReleaseTags извлекает жанры/теги из релизного заголовка. Берёт группы-списки
// (с запятыми или содержащие год) и распознаёт отдельные группы движков/платформ.
// Год, метки цензуры и языковые коды в теги не попадают.
func parseReleaseTags(s string) []string {
	seen := make(map[string]bool)
	var tags []string

	add := func(t string) {
		t = strings.TrimSpace(t)
		if t == "" {
			return
		}
		low := strings.ToLower(t)
		if isYear(t) || censorshipTokens[low] || langCodes[low] != "" {
			return
		}
		if seen[low] {
			return
		}
		seen[low] = true
		tags = append(tags, t)
	}

	for _, g := range bracketGroups(s) {
		if strings.Contains(g, ",") || groupHasYear(g) {
			for _, part := range strings.Split(g, ",") {
				add(part)
			}
		} else if singleTokenTags[strings.ToLower(strings.TrimSpace(g))] {
			add(strings.TrimSpace(g))
		}
	}

	if len(tags) > 20 {
		tags = tags[:20]
	}
	return tags
}

// parseReleaseLanguages ищет во ВСЕХ группах заголовка известные коды языков.
// Покрывает и pornolab ("[rus+eng+jap]"), и island ("(Rus/Eng)").
func parseReleaseLanguages(s string) []string {
	lower := strings.ToLower(s)
	seen := make(map[string]bool)
	var out []string

	// Разбиваем на токены по любым разделителям, кроме букв
	var token strings.Builder
	flush := func() {
		if token.Len() == 0 {
			return
		}
		t := token.String()
		token.Reset()
		if canon, ok := langCodes[t]; ok && !seen[canon] {
			seen[canon] = true
			out = append(out, canon)
		}
	}
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= 'а' && r <= 'я') || r == 'ё' {
			token.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}
