package scanner

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// Ограничители обхода: движок определяется по характерным файлам/папкам,
// поэтому глубоко и долго ходить по дереву не нужно.
const (
	engineMaxDepth = 4
	engineMaxFiles = 6000
)

// DetectEngine определяет движок игры по содержимому папки (без чтения файлов,
// только по именам файлов/каталогов). Возвращает каноническое имя
// ("Unity", "Ren'Py", "RPG Maker", ...) или "" если уверенно определить не удалось.
//
// Правила проверяются от специфичного к общему: NW.js-обёртка (nw.dll) бывает и у
// RPG Maker MV/MZ, и у Construct, поэтому их специфичные маркеры (www/, c2/c3runtime.js)
// ловятся раньше общего HTML.
func DetectEngine(folderPath string) string {
	bases := map[string]bool{}     // имена файлов (lower)
	exts := map[string]bool{}      // расширения с точкой (lower)
	dirs := map[string]bool{}      // имена каталогов (lower)
	rootFiles := map[string]bool{} // файлы прямо в корне папки игры
	unityData := false             // есть каталог вида *_Data (Unity)
	rmProject := false             // *.rpgproject / *.rmmvproject / *.rmmzproject

	root := filepath.Clean(folderPath)
	files := 0

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		depth := 0
		if rel, rerr := filepath.Rel(root, path); rerr == nil && rel != "." {
			depth = strings.Count(rel, string(filepath.Separator)) + 1
		}
		name := strings.ToLower(d.Name())

		if d.IsDir() {
			if path != root {
				dirs[name] = true
				if strings.HasSuffix(name, "_data") {
					unityData = true
				}
			}
			if depth >= engineMaxDepth {
				return filepath.SkipDir
			}
			return nil
		}

		files++
		if files > engineMaxFiles {
			return filepath.SkipAll
		}
		bases[name] = true
		if ext := filepath.Ext(name); ext != "" {
			exts[ext] = true
		}
		if depth == 1 {
			rootFiles[name] = true
		}
		if strings.HasSuffix(name, ".rpgproject") ||
			strings.HasSuffix(name, ".rmmvproject") ||
			strings.HasSuffix(name, ".rmmzproject") {
			rmProject = true
		}
		return nil
	})

	switch {
	case dirs["renpy"] && dirs["game"], exts[".rpa"], exts[".rpyc"]:
		return "Ren'Py"
	case exts[".rgss3a"], exts[".rgss2a"], exts[".rgssad"],
		rmProject, dirs["www"],
		bases["package.json"] && dirs["data"] && dirs["img"]:
		return "RPG Maker"
	case exts[".wolf"], bases["gurugurusmf4.dll"]:
		return "Wolf RPG"
	case exts[".xp3"]:
		return "KiriKiri"
	case dirs["tyrano"]:
		return "TyranoBuilder"
	case bases["unityplayer.dll"], bases["globalgamemanagers"], unityData && bases["resources.assets"]:
		return "Unity"
	case exts[".pak"] && dirs["paks"], dirs["engine"] && dirs["binaries"]:
		return "Unreal Engine"
	case exts[".pck"]:
		return "Godot"
	case bases["data.win"]:
		return "Game Maker"
	case bases["c2runtime.js"], bases["c3runtime.js"]:
		return "Construct"
	case exts[".swf"]:
		return "Flash"
	case exts[".qsp"], exts[".qsps"]:
		return "QSP"
	case rootFiles["index.html"]:
		return "HTML"
	}
	return ""
}
