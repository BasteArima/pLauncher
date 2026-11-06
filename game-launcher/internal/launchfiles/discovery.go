package launchfiles

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type FileInfo struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	RelativePath string `json:"relative_path"`
	Label        string `json:"label"`
	LabelKey     string `json:"label_key"`
	Primary      bool   `json:"primary"`
	Availability string `json:"availability"` // available, missing, unavailable, unsupported
}

type Candidates struct {
	Files      []FileInfo `json:"files"`
	Incomplete bool       `json:"incomplete"`
	Reason     string     `json:"reason"` // entries, candidates, time, unreadable; not UI text
}

func Describe(path, label, folder string) FileInfo {
	rel, err := filepath.Rel(folder, path)
	if err != nil {
		rel = path
	}
	f := FileInfo{Path: path, Name: filepath.Base(path), RelativePath: rel, Label: label}
	if label == "" {
		f.LabelKey = LabelKey(path)
	}
	return f
}

func Availability(path string) string {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "missing"
	}
	if err != nil {
		return "unavailable"
	}
	if info.IsDir() {
		if runtime.GOOS == "darwin" && strings.EqualFold(filepath.Ext(path), ".app") {
			return "available"
		}
		return "unsupported"
	}
	if !info.Mode().IsRegular() {
		return "unsupported"
	}
	return "available"
}

var skipDirs = map[string]bool{
	"assets": true,
	"saves":  true, "save": true, "img": true, "images": true, "audio": true, "sound": true,
	"music": true, "fonts": true, "css": true, "js": true, "data": true, "movies": true,
	"video": true, "locales": true, "node_modules": true, "renpy": true, "lib": true,
	"cache": true, "__pycache__": true,
}

func candidate(name string, isDir bool) bool {
	original := name
	name = strings.ToLower(name)
	ext := filepath.Ext(name)
	if isDir {
		return runtime.GOOS == "darwin" && ext == ".app"
	}
	if strings.Contains(name, "unins") || strings.Contains(name, "crash") {
		return false
	}
	switch ext {
	case ".exe", ".com", ".bat", ".cmd", ".jar", ".swf", ".qsp", ".rags", ".love", ".py", ".pyw", ".lnk":
		return true
	case ".sh", ".x86", ".x86_64", ".appimage", ".command":
		return runtime.GOOS != "windows"
	case ".html", ".htm":
		base := strings.TrimSuffix(name, ext)
		return DocumentHint(original) || base == "index" || base == "game" || base == "play" || base == "start" || base == "main"
	case ".pdf", ".txt":
		return DocumentHint(original)
	}
	return false
}

type limits struct {
	entries, candidates int
	duration            time.Duration
}

func Find(ctx context.Context, folder string) (Candidates, error) {
	return find(ctx, folder, limits{10000, 200, 5 * time.Second})
}

func find(ctx context.Context, folder string, bound limits) (Candidates, error) {
	ctx, cancel := context.WithTimeout(ctx, bound.duration)
	defer cancel()
	out := Candidates{Files: []FileInfo{}}
	count := 0
	seen := map[string]bool{}
	stop := false
	mark := func(reason string) {
		out.Incomplete = true
		if out.Reason == "" {
			out.Reason = reason
		}
	}
	var walk func(string, int) error
	walk = func(dir string, depth int) error {
		if ctx.Err() != nil {
			mark("time")
			stop = true
			return nil
		}
		f, err := os.Open(dir)
		if err != nil {
			if depth == 0 {
				return err
			}
			mark("unreadable")
			return nil
		}
		defer f.Close()
		for !stop {
			if ctx.Err() != nil {
				mark("time")
				stop = true
				break
			}
			// Batches bound memory even in folders with millions of entries.
			entries, readErr := f.ReadDir(64)
			if readErr != nil && readErr != io.EOF {
				if depth == 0 {
					return readErr
				}
				mark("unreadable")
				break
			}
			for _, e := range entries {
				if ctx.Err() != nil {
					mark("time")
					stop = true
					break
				}
				if count >= bound.entries {
					mark("entries")
					stop = true
					break
				}
				count++
				if e.Type()&os.ModeSymlink != 0 || strings.HasPrefix(e.Name(), ".") {
					continue
				}
				path := filepath.Join(dir, e.Name())
				if candidate(e.Name(), e.IsDir()) {
					key := Key(path)
					if !seen[key] {
						if len(out.Files) >= bound.candidates {
							mark("candidates")
							stop = true
							break
						}
						seen[key] = true
						out.Files = append(out.Files, Describe(path, "", folder))
					}
				}
				if e.IsDir() && depth < 2 && !skipDirs[strings.ToLower(e.Name())] && !strings.EqualFold(filepath.Ext(e.Name()), ".app") {
					if err := walk(path, depth+1); err != nil {
						return err
					}
					if stop {
						break
					}
				}
			}
			if readErr == io.EOF {
				break
			}
		}
		return nil
	}
	err := walk(folder, 0)
	sort.Slice(out.Files, func(i, j int) bool { return Key(out.Files[i].Path) < Key(out.Files[j].Path) })
	return out, err
}
