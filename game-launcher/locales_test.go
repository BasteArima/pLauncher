package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Все встроенные переводы должны иметь те же ключи, что en.json,
// и те же плейсхолдеры {n}/{title}/... в каждой строке.
func TestLocalesMatchEnglish(t *testing.T) {
	dir := filepath.Join("frontend", "src", "locales")
	load := func(name string) map[string]string {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]string
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return m
	}
	en := load("en.json")
	ph := regexp.MustCompile(`\{\w+\}`)
	placeholders := func(s string) string {
		p := ph.FindAllString(s, -1)
		sort.Strings(p)
		return strings.Join(p, ",")
	}

	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(files) < 2 {
		t.Fatalf("переводы не найдены в %s", dir)
	}
	for _, f := range files {
		name := filepath.Base(f)
		if name == "en.json" {
			continue
		}
		loc := load(name)
		for k, v := range en {
			tv, ok := loc[k]
			if !ok {
				t.Errorf("%s: нет ключа %q", name, k)
				continue
			}
			if strings.TrimSpace(tv) == "" && strings.TrimSpace(v) != "" {
				t.Errorf("%s: пустое значение %q", name, k)
			}
			if placeholders(tv) != placeholders(v) {
				t.Errorf("%s: %q — плейсхолдеры %q, в en.json %q", name, k, placeholders(tv), placeholders(v))
			}
		}
		for k := range loc {
			if _, ok := en[k]; !ok {
				t.Errorf("%s: лишний ключ %q", name, k)
			}
		}
	}
}
