package parser

import (
	"reflect"
	"testing"
)

// Заголовки взяты из реальных сохранённых страниц (pornolab h1.maintitle, island finfo «Название»).
var releaseCases = []struct {
	name    string
	raw     string
	title   string
	version string
	langs   []string
}{
	{
		name:    "pornolab1",
		raw:     "Escape Prison [1.0] (FUJIYAMA SOFT) [cen] [2026, Action, Arcade, RPG Maker] [jap]",
		title:   "Escape Prison",
		version: "1.0",
		langs:   []string{"日本語"},
	},
	{
		name:    "pornolab2",
		raw:     "Project Skysex 2: Final [Ver. 29-05-2020 Fix] (Shenija, SLMP, Bethesda) [uncen] [2018, Action, RPG] [rus]",
		title:   "Project Skysex 2: Final",
		version: "Ver. 29-05-2020 Fix",
		langs:   []string{"Русский"},
	},
	{
		name:    "pornolab4_multilang",
		raw:     "SiNiSistar 2 / シニシスタ２ [1.3.1] (Nennai) [ptcen] [ 2025, Action, Unity ] [rus+eng+jap+chi+kor]",
		title:   "SiNiSistar 2 / シニシスタ２",
		version: "1.3.1",
		langs:   []string{"Русский", "English", "日本語", "中文", "한국어"},
	},
	{
		name:    "pornolab5_prefix",
		raw:     "(Сборник) [Mods] Koikatsu Sunshine / Koikatsu Sunshine EX [BetterRepack R6] (Illusion) [uncen] [2021, SLG, Unity] [Jap+Eng+Rus]",
		title:   "Koikatsu Sunshine / Koikatsu Sunshine EX",
		version: "BetterRepack R6",
		langs:   []string{"日本語", "English", "Русский"},
	},
	{
		name:    "pornolab6_modprefix",
		raw:     "[MOD] Jack-o-nine-tails / Валет Плетей [2.4.0.1] (Old Huntsman) [uncen] [2025, RPG] [rus]",
		title:   "Jack-o-nine-tails / Валет Плетей",
		version: "2.4.0.1",
		langs:   []string{"Русский"},
	},
	{
		name:    "island",
		raw:     "Медвежий Ручей / Black Bear Creek [v.1.1 Rus / v.2.0 Eng] (2026) (Rus/Eng) [Ren'Py] [Linux] [Android] [bbc] [Lex Apps Games]",
		title:   "Медвежий Ручей / Black Bear Creek",
		version: "v.1.1 Rus / v.2.0 Eng",
		langs:   []string{"Русский", "English"},
	},
}

func TestParseReleaseTags(t *testing.T) {
	cases := map[string][]string{
		"Escape Prison [1.0] (FUJIYAMA SOFT) [cen] [2026, Action, Arcade, RPG Maker] [jap]": {"Action", "Arcade", "RPG Maker"},
		"Game [v.1.1] (2026) (Rus/Eng) [Ren'Py] [Linux] [Android] [bbc] [Lex Apps Games]":   {"Ren'Py", "Linux", "Android"},
	}
	for raw, want := range cases {
		if got := parseReleaseTags(raw); !reflect.DeepEqual(got, want) {
			t.Errorf("parseReleaseTags(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestReleaseTitleParsing(t *testing.T) {
	for _, tc := range releaseCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cleanReleaseTitle(tc.raw); got != tc.title {
				t.Errorf("title = %q, want %q", got, tc.title)
			}
			if got := versionFromTitle(tc.raw); got != tc.version {
				t.Errorf("version = %q, want %q", got, tc.version)
			}
			if got := parseReleaseLanguages(tc.raw); !reflect.DeepEqual(got, tc.langs) {
				t.Errorf("langs = %v, want %v", got, tc.langs)
			}
		})
	}
}
