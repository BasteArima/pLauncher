// Package sessionlock provides OS-owned locks. They survive UI shutdown, but never
// survive the owning monitor's death; PID reuse and stale heartbeats are irrelevant.
package sessionlock

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrBusy = errors.New("session lock is held")

type Lock struct{ file *os.File }

func (l *Lock) Close() error { return l.file.Close() }

// Locks live outside the library so holding the gate doesn't prevent moving it on Windows.
// Never unlink a lock file: another process may already hold the old inode/handle.
func Dir(dataDir string) (string, error) {
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	if runtime.GOOS == "windows" {
		abs = strings.ToLower(abs)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "pLauncher", "session-locks", fmt.Sprintf("%x", sha256.Sum256([]byte(abs)))), nil
}

func Try(dataDir, name string) (*Lock, error) {
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\:`) {
		return nil, fmt.Errorf("invalid lock name")
	}
	dir, err := Dir(dataDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, name+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, err
	}
	return &Lock{f}, nil
}

// Active must be called under the gate when used to authorize a data operation.
// Scan locks too, not just pending rows: deleting a running game removes its row.
func Active(dataDir string) (bool, error) {
	dir, err := Dir(dataDir)
	if err != nil {
		return false, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "run-") || !strings.HasSuffix(e.Name(), ".lock") {
			continue
		}
		l, err := Try(dataDir, strings.TrimSuffix(e.Name(), ".lock"))
		if errors.Is(err, ErrBusy) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		l.Close()
	}
	return false, nil
}
