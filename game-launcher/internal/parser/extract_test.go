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

func TestAuthorFromTitle(t *testing.T) {
	cases := map[string]string{
		"College of Mysteria [v0.13] [HappySteveGames]": "HappySteveGames",
		"Some Game [v1.2] [Studio X]":                   "Studio X",
		"Only Version [v1.0]":                           "", // одна скобка = версия, не автор
		"No brackets at all":                            "",
	}
	for raw, want := range cases {
		if got := authorFromTitle(raw); got != want {
			t.Errorf("authorFromTitle(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestEngineFromTokens(t *testing.T) {
	cases := []struct {
		tokens []string
		want   string
	}{
		{[]string{"VN", "Ren'Py"}, "Ren'Py"},
		{[]string{"Completed", "Unity"}, "Unity"},
		{[]string{"VN", "Completed"}, ""},
		{[]string{"RPGM"}, "RPG Maker"},
	}
	for _, c := range cases {
		if got := engineFromTokens(c.tokens); got != c.want {
			t.Errorf("engineFromTokens(%v) = %q, want %q", c.tokens, got, c.want)
		}
	}
}

func TestDeveloperFromHTML(t *testing.T) {
	cases := map[string]string{
		// pornolab: двоеточие зажато между закрывающими </span>
		`<span class="post-color-text"><span class="post-b">Разработчик/Издатель</span>:</span> Libero<br>`: "Libero",
		// island: метка целиком в <b>, дальше имя и ссылки через " - " / "|"
		`<b>Разработчик/Издатель:</b> Milk Dragon Studios - <a href="x">patreon</a> | <a>itch.io</a>`: "Milk Dragon Studios",
		`<b>Разработчик/Издатель:</b> Libero`: "Libero",
		`<div>нет такого поля</div>`:           "",
	}
	for in, want := range cases {
		if got := developerFromHTML(in); got != want {
			t.Errorf("developerFromHTML(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIslandDescription(t *testing.T) {
	// Реальный текст блока ss-fstory-content (после cleanText): описание RU+EN,
	// затем технические поля и changelog — всё это должно быть отрезано.
	raw := "Описание:Жизнь не может стать лучше!Ты живёшь в большом городе. " +
		"Life can't get any better!You're living in the big city." +
		"Год выпуска: 2024Жанр: 2dcg, adventureВерсия: v.0.12 Rus / v.0.16 Eng" +
		"\"Changelog:\"v0.16 - 2026-06-13 Tons of small bugs."
	want := "Жизнь не может стать лучше!Ты живёшь в большом городе. " +
		"Life can't get any better!You're living in the big city."
	if got := islandDescription(raw); got != want {
		t.Errorf("islandDescription:\n got = %q\nwant = %q", got, want)
	}
	// Без метки «Год выпуска» — отдаём всё после «Описание:».
	if got := islandDescription("Описание: просто текст"); got != "просто текст" {
		t.Errorf("islandDescription(no year) = %q", got)
	}
}

func TestVersionFromHTML(t *testing.T) {
	cases := map[string]string{
		// island: метка в <b>, двойная версия сохраняется целиком
		`<b><span></span>Версия:</b> v.0.12 Rus / v.0.16 Eng<br>`: "v.0.12 Rus / v.0.16 Eng",
		`<b>Версия:</b> v.0.23 HotFix`:                            "v.0.23 HotFix",
		`<div>нет поля версии</div>`:                              "",
	}
	for in, want := range cases {
		if got := versionFromHTML(in); got != want {
			t.Errorf("versionFromHTML(%q) = %q, want %q", in, got, want)
		}
	}
}
