package scanner

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	searchBracketTail = regexp.MustCompile(`\s*(\[[^\[\]]*\]|\([^()]*\)|\{[^{}]*\})\s*$`)
	searchVersion     = regexp.MustCompile(`(?i)(?:^|[\s_.-])(?:v(\d+(?:\.\d+)*[a-z]?)|(\d+(?:\.\d+)+[a-z]?)|((?:ep(?:isode)?|ch(?:apter)?)[. _-]*\d+)|((?:season)[ _-]+\d+))(?:$|[\s_.-])`)
	searchNameWord    = regexp.MustCompile(`[\p{L}\p{N}]+`)
	searchFinal       = regexp.MustCompile(`(?i)(?:^|[\s_.-])(final)$`)
)

// SuggestSearchName очищает только поисковый запрос; названия при сканировании не меняются.
func SuggestSearchName(folderName string) (title, version string) {
	title = strings.TrimSpace(folderName)
	// Убираем упаковку до разбора версии, не разрушая десятичные точки.
	packaging := map[string]bool{"pc": true, "win": true, "windows": true, "linux": true, "mac": true, "x64": true, "rus": true, "eng": true, "repack": true}
	title = searchNameWord.ReplaceAllStringFunc(title, func(s string) string {
		if packaging[strings.ToLower(s)] {
			return " "
		}
		return s
	})
	title = strings.Trim(title, " ._-\t")
	var tails []string
	for {
		loc := searchBracketTail.FindStringIndex(title)
		if loc == nil {
			break
		}
		tails = append(tails, strings.TrimSpace(title[loc[0]:]))
		title = strings.Trim(title[:loc[0]], " ._-\t")
	}
	removeVersion := func(s string) (string, string) {
		loc := searchVersion.FindStringSubmatchIndex(s)
		if loc != nil {
			for i := 2; i < len(loc); i += 2 {
				if loc[i] >= 0 {
					return s[:loc[0]] + " " + s[loc[1]:], s[loc[i]:loc[i+1]]
				}
			}
		}
		trimmed := strings.Trim(s, " ._-\t")
		if m := searchFinal.FindStringSubmatchIndex(trimmed); m != nil {
			return trimmed[:m[0]], trimmed[m[2]:m[3]]
		}
		return s, ""
	}
	title, version = removeVersion(title)
	if version == "" {
		for i := len(tails) - 1; i >= 0; i-- {
			_, version = removeVersion(strings.Trim(tails[i], "[](){} "))
			if version != "" {
				break
			}
		}
	}
	title = strings.NewReplacer("_", " ", ".", " ", "-", " ").Replace(title)
	runes := []rune(title)
	var out strings.Builder
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(runes[i-1]) || (unicode.IsUpper(runes[i-1]) && i+1 < len(runes) && unicode.IsLower(runes[i+1]))) {
			out.WriteByte(' ')
		}
		out.WriteRune(r)
	}
	return strings.Join(strings.Fields(out.String()), " "), version
}
