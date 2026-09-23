package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// appConfig — крошечный файл-указатель, который хранится ВНЕ папки data
// (иначе негде было бы запомнить, где сама папка data лежит).
type appConfig struct {
	DataDir         string `json:"data_dir"`
	WindowWidth     int    `json:"window_width,omitempty"`
	WindowHeight    int    `json:"window_height,omitempty"`
	WindowMaximised bool   `json:"window_maximised,omitempty"`
}

// configFilePath возвращает путь к указателю в системной папке конфигурации
// (Windows: %AppData%\pLauncher, macOS: ~/Library/Application Support/pLauncher,
// Linux: ~/.config/pLauncher).
func configFilePath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "pLauncher", "config.json"), nil
}

func loadConfig() appConfig {
	var cfg appConfig
	path, err := configFilePath()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(cfg appConfig) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(path, data, 0644)
}

// defaultDataDir — рекомендованное место для данных, зависит от ОС.
func defaultDataDir() string {
	switch runtime.GOOS {
	case "windows":
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "pLauncher", "data")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "pLauncher", "data")
		}
	default: // linux и прочие — XDG
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "pLauncher", "data")
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share", "pLauncher", "data")
		}
	}
	// Фолбэк — рядом с лаунчером
	return portableDataDir()
}

// portableDataDir — папка data рядом с исполняемым файлом лаунчера.
func portableDataDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "data")
	}
	return filepath.Join(".", "data")
}

// documentsDataDir — папка в Документах пользователя.
func documentsDataDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Documents", "pLauncher", "data")
	}
	return defaultDataDir()
}

// languageReadme — инструкция, которую кладём в data/languages при первом запуске.
const languageReadme = `# Custom languages for pLauncher

Drop a "<code>.json" file in this folder to add a new interface language,
then restart the launcher. The language appears in Settings.

How to:
1. Copy "_example.json" (a full English template) in this folder.
2. Rename the copy to your language code, e.g. "it.json", "ko.json", "nl.json".
3. Translate ONLY the text on the right side of each line. Keep the keys (left) unchanged.
4. "__name" is the language name shown in the menu (e.g. "Italiano").
5. Keep placeholders like {n}, {path}, {title}, {err} and "\n" as they are.
6. Save in UTF-8. Restart the launcher.

Note: files starting with "_" are ignored (so "_example.json" is not loaded as a language).
Tip: you can also fix a built-in language (en/ru/es/pt/de/fr/zh/uk/ja/pl/tr) — put e.g. "en.json" with ONLY the keys
you want to change; they are merged over the built-in ones, the rest stays intact.

------------------------------------------------------------------------------

# Свои языки для pLauncher

Положите сюда файл "<код>.json", чтобы добавить язык интерфейса,
затем перезапустите лаунчер. Язык появится в Настройках.

Как сделать:
1. Скопируйте здесь файл "_example.json" (полный английский шаблон).
2. Переименуйте копию в код языка, напр. "it.json", "ko.json", "nl.json".
3. Переводите ТОЛЬКО текст справа от каждой строки. Ключи (слева) не меняйте.
4. "__name" — название языка в меню (напр. "Italiano").
5. Подстановки {n}, {path}, {title}, {err} и "\n" оставляйте как есть.
6. Сохраняйте в UTF-8. Перезапустите лаунчер.

Примечание: файлы, начинающиеся с "_", игнорируются (поэтому "_example.json" не считается языком).
Совет: можно поправить встроенный язык (en/ru/es/pt/de/fr/zh/uk/ja/pl/tr) — положите, напр., "en.json" ТОЛЬКО с теми
ключами, что хотите изменить; они наложатся поверх встроенных, остальное останется как было.
`

// writeLanguageHelp кладёт README и готовый пример в папку languages (только если их ещё нет).
func writeLanguageHelp(dir string) {
	readme := filepath.Join(dir, "README.md")
	if _, err := os.Stat(readme); os.IsNotExist(err) {
		os.WriteFile(readme, []byte(languageReadme), 0644)
	}
	example := filepath.Join(dir, "_example.json")
	if _, err := os.Stat(example); os.IsNotExist(err) && len(exampleLocale) > 0 {
		os.WriteFile(example, exampleLocale, 0644)
	}
}

// moveDir перемещает содержимое src в dst. Сначала пытается быстрый Rename
// (в пределах одного тома), при неудаче копирует рекурсивно и удаляет источник.
func moveDir(src, dst string) error {
	if src == dst {
		return nil
	}
	if _, err := os.Stat(src); err != nil {
		return nil // нечего переносить
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	// Быстрый путь: переименование (работает в пределах тома, если dst не существует)
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		if err := os.Rename(src, dst); err == nil {
			return nil
		}
	}
	// Медленный путь: рекурсивное копирование + удаление
	if err := copyDir(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(s, d); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
