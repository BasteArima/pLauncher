// Package launchfiles owns file identity, labels and list invariants, without disk scans or launches.
package launchfiles

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"game-launcher/internal/apperr"
	"game-launcher/internal/models"
)

const MaxExtras = 8

// Key is for comparison only. Keep the original path for display and execution.
func Key(p string) string {
	if p == "" {
		return ""
	}
	if runtime.GOOS == "windows" {
		p = strings.ReplaceAll(p, "/", `\`)
	}
	abs, err := filepath.Abs(p)
	if err == nil {
		p = abs
	}
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

var camelBoundary = regexp.MustCompile(`([a-z])([A-Z])`)
var words = regexp.MustCompile(`[a-z]+`)

func hints(path string) map[string]bool {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = camelBoundary.ReplaceAllString(base, "$1 $2")
	out := map[string]bool{}
	for _, word := range words.FindAllString(strings.ToLower(base), -1) {
		out[word] = true
	}
	return out
}

// LabelKey is translated by the UI only when the explicit label is empty.
func LabelKey(path string) string {
	h := hints(path)
	if h["walkthrough"] || h["guide"] {
		return "launch.label_walkthrough"
	}
	if h["config"] || h["settings"] {
		return "launch.label_settings"
	}
	return ""
}

func DocumentHint(path string) bool {
	return LabelKey(path) != "" || hints(path)["setup"]
}

func Files(g *models.Game) []models.LaunchFile {
	out := make([]models.LaunchFile, 0, len(g.LaunchFiles)+1)
	if g.ExecPath != "" {
		out = append(out, models.LaunchFile{Path: g.ExecPath, Label: g.ExecLabel})
	}
	return append(out, g.LaunchFiles...)
}

// Validate never stats paths: disconnected drives and missing files are valid saved references.
// A nil old game is used for new games; enforceLimit=false is reserved for relink/import internals.
func Validate(g, old *models.Game, enforceLimit bool) error {
	seen := map[string]bool{}
	for _, f := range Files(g) {
		if strings.TrimSpace(f.Path) == "" || strings.ContainsRune(f.Path, 0) {
			return apperr.New("launch.invalid_path", nil, "launch file path must not be blank or contain NUL")
		}
		k := Key(f.Path)
		if seen[k] {
			return apperr.New("launch.duplicate", apperr.P{"path": f.Path}, "launch file is already listed: %s", f.Path)
		}
		seen[k] = true
	}
	if enforceLimit && len(g.LaunchFiles) > MaxExtras {
		allowed := old != nil && len(g.LaunchFiles) <= len(old.LaunchFiles)
		prior := map[string]bool{}
		if old != nil {
			for _, f := range Files(old) {
				prior[Key(f.Path)] = true
			}
		}
		for k := range seen {
			if !prior[k] {
				allowed = false
			}
		}
		if !allowed {
			return apperr.New("launch.limit", apperr.P{"n": MaxExtras}, "at most %d additional launch files can be added", MaxExtras)
		}
	}
	return nil
}

// Consolidate removes aliases and the primary from extras, retaining the first explicit label.
// Used for detection/relink, never to silently repair an invalid editor submission.
func Consolidate(g *models.Game) {
	out := make([]models.LaunchFile, 0, len(g.LaunchFiles))
	indices := map[string]int{}
	for _, f := range g.LaunchFiles {
		k := Key(f.Path)
		if k == "" {
			continue
		}
		if k == Key(g.ExecPath) {
			if g.ExecLabel == "" {
				g.ExecLabel = f.Label
			}
			continue
		}
		if i, ok := indices[k]; ok {
			if out[i].Label == "" {
				out[i].Label = f.Label
			}
			continue
		}
		indices[k] = len(out)
		out = append(out, f)
	}
	g.LaunchFiles = out
}

func SetDetectedPrimary(g *models.Game, path string) {
	if g.ExecPath != "" || path == "" {
		return
	}
	g.ExecPath = path
	Consolidate(g)
}

// Merge keeps the survivor's role/order/labels. Capacity is deliberately not enforced here.
func Merge(g, duplicate *models.Game) {
	if duplicate != nil {
		if g.ExecPath == "" {
			g.ExecPath, g.ExecLabel = duplicate.ExecPath, duplicate.ExecLabel
		} else if duplicate.ExecPath != "" {
			g.LaunchFiles = append(g.LaunchFiles, models.LaunchFile{Path: duplicate.ExecPath, Label: duplicate.ExecLabel})
		}
		g.LaunchFiles = append(g.LaunchFiles, duplicate.LaunchFiles...)
	}
	Consolidate(g)
}

// RebaseExtra preserves the intended new path even when it doesn't exist. External paths stay external.
func RebaseExtra(oldFolder, path, newFolder string) string {
	if oldFolder == "" || path == "" {
		return path
	}
	rel, err := filepath.Rel(oldFolder, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return path
	}
	return filepath.Join(newFolder, rel)
}
