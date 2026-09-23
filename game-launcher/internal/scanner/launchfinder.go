package scanner

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

// Поиск запускалок, отличных от .exe: HTML-игры (Twine, RPG Maker MV/MZ в браузерной
// сборке, Construct), bat/cmd-обёртки, Java, Flash, QSP, RAGS, LÖVE, а на Linux/macOS —
// shell-скрипты и нативные бинарники. Вызывается, только если .exe в папке нет.

// otherMaxDepth — глубже не ищем: у таких игр запускалка почти всегда в корне
// или в одной подпапке (www/, game/, bin/).
const otherMaxDepth = 2

// launcherExtWeight — базовый вес расширения: чем специфичнее формат, тем вероятнее,
// что это именно игра (единственный .qsp в папке — почти наверняка она).
var launcherExtWeight = map[string]int{
	".qsp": 60, ".rags": 60, ".love": 60, ".swf": 55, ".jar": 50,
	".html": 40, ".htm": 40,
	".bat": 30, ".cmd": 30,
}

// Нативные запускалки Linux/macOS (на Windows бесполезны). .app — это папка-бандл macOS.
var unixLauncherExtWeight = map[string]int{
	".sh": 45, ".x86_64": 55, ".x86": 45, ".appimage": 60, ".app": 70, ".command": 45,
}

// windowsOnlyExt — обёртки, которые работают только на Windows.
var windowsOnlyExt = map[string]bool{".bat": true, ".cmd": true}

// htmlLauncherNames — имена, под которыми HTML-игра обычно лежит.
var htmlLauncherNames = map[string]bool{
	"index": true, "game": true, "play": true, "start": true, "main": true,
}

// otherSkip — подстроки в имени, которые точно не запускалка
// (документация, установщики, служебные скрипты).
var otherSkip = []string{
	"readme", "read_me", "manual", "license", "licence", "changelog", "changes",
	"credits", "walkthrough", "guide", "faq", "help", "patchnote", "patch_note",
	"install", "setup", "uninst", "update", "config", "settings", "compile", "build",
	"notice", "eula", "legal", "thirdparty", "third_party",
}

// otherSkipDirs — папки, где запускалок не бывает (ассеты, сейвы, исходники движка).
var otherSkipDirs = map[string]bool{
	"saves": true, "save": true, "img": true, "images": true, "audio": true,
	"sound": true, "music": true, "fonts": true, "css": true, "js": true,
	"data": true, "movies": true, "video": true, "locales": true, "node_modules": true,
	"docs": true, "doc": true, "manual": true, "renpy": true, "lib": true,
}

// findOtherLauncher ищет лучшую не-.exe запускалку в папке игры (или "").
// nativeOnly — только нативные запускалки Linux/macOS (unixLauncherExtWeight).
func findOtherLauncher(folderPath string, nativeOnly bool) string {
	folderKey := normalizeName(filepath.Base(folderPath))
	unix := goruntime.GOOS != "windows"

	bestPath := ""
	bestScore := 0 // кандидаты с неположительным баллом не принимаем

	_ = filepath.WalkDir(folderPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(folderPath, path)
		depth := 0
		if rel != "." {
			depth = len(strings.Split(rel, string(os.PathSeparator)))
		}
		name := strings.ToLower(d.Name())
		ext := filepath.Ext(name)
		isBundle := d.IsDir() && ext == ".app" && goruntime.GOOS == "darwin"
		if d.IsDir() && !isBundle {
			if depth > 0 && (depth >= otherMaxDepth || otherSkipDirs[name]) {
				return filepath.SkipDir
			}
			return nil
		}

		weight, ok := 0, false
		if unix {
			weight, ok = unixLauncherExtWeight[ext]
		}
		if !ok && !nativeOnly {
			weight, ok = launcherExtWeight[ext]
			if unix && windowsOnlyExt[ext] {
				ok = false
			}
		}
		if !ok || (ext == ".app" && !isBundle) {
			if isBundle {
				return filepath.SkipDir
			}
			return nil
		}
		base := strings.TrimSuffix(name, ext)
		for _, bad := range otherSkip {
			if strings.Contains(base, bad) {
				return nil
			}
		}

		s := weight - depth*20
		nb := normalizeName(base)
		switch {
		case nb != "" && nb == folderKey:
			s += 40
		case preferredNames[base]:
			s += 30
		}
		// HTML: только «говорящие» имена или совпадение с папкой; иначе это скорее
		// документация/страница-заглушка, чем игра.
		if ext == ".html" || ext == ".htm" {
			if htmlLauncherNames[base] {
				s += 25
			} else if nb != folderKey {
				s -= 45
			}
		}
		// bat/cmd с произвольным именем — тоже слабый сигнал.
		if (ext == ".bat" || ext == ".cmd") && !preferredNames[base] && nb != folderKey {
			s -= 15
		}

		if s > bestScore {
			bestScore = s
			bestPath = path
		}
		if isBundle {
			return filepath.SkipDir // внутрь бандла не заходим
		}
		return nil
	})
	return bestPath
}
