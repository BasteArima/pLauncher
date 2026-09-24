package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"game-launcher/internal/db"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// =========================================================================
// ЭКСПОРТ / ИМПОРТ БИБЛИОТЕКИ
// Архив .zip: games.db (снимок БД: игры, коллекции, папки сканирования),
// covers/ (обложки и скриншоты), languages/ (пользовательские языки) и манифест.
// =========================================================================

const backupManifestName = "plauncher-backup.json"

// Что из папки данных попадает в архив и заменяется при импорте.
var backupEntries = []string{"games.db", "covers", "languages"}

type backupManifest struct {
	App       string `json:"app"`
	Version   string `json:"version"`
	CreatedAt int64  `json:"created_at"`
	Games     int    `json:"games"`
}

// ExportLibrary спрашивает, куда сохранить архив, и пишет туда резервную копию.
// Возвращает путь к архиву ("" — пользователь отменил диалог).
func (a *App) ExportLibrary() (string, error) {
	sqlite, ok := a.repo.(*db.SQLiteRepo)
	if !ok || sqlite == nil {
		return "", fmt.Errorf("data folder is not selected yet")
	}
	dest, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Сохранить резервную копию библиотеки",
		DefaultFilename: "pLauncher-backup-" + time.Now().Format("2006-01-02") + ".zip",
		Filters:         []runtime.FileFilter{{DisplayName: "ZIP (*.zip)", Pattern: "*.zip"}},
	})
	if err != nil || dest == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(dest), ".zip") {
		dest += ".zip"
	}

	// Снимок БД во временный файл: так в архив не попадёт полузаписанное состояние
	snap := filepath.Join(os.TempDir(), fmt.Sprintf("plauncher-export-%d.db", time.Now().UnixNano()))
	defer os.Remove(snap)
	if err := sqlite.BackupTo(a.ctx, snap); err != nil {
		return "", fmt.Errorf("database snapshot failed: %w", err)
	}

	games, _ := a.repo.GetAllGames(a.ctx)
	manifest, _ := json.MarshalIndent(backupManifest{
		App: "pLauncher", Version: appVersion, CreatedAt: time.Now().Unix(), Games: len(games),
	}, "", "  ")

	// Пишем во временный файл рядом и переименовываем в конце — битый архив не останется
	part := dest + ".part"
	if err := writeBackupZip(part, manifest, snap, a.dataDir); err != nil {
		os.Remove(part)
		return "", err
	}
	os.Remove(dest)
	if err := os.Rename(part, dest); err != nil {
		os.Remove(part)
		return "", err
	}
	return dest, nil
}

func writeBackupZip(path string, manifest []byte, dbSnapshot, dataDir string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)

	w, err := zw.Create(backupManifestName)
	if err != nil {
		return err
	}
	if _, err := w.Write(manifest); err != nil {
		return err
	}
	if err := addFileToZip(zw, dbSnapshot, "games.db"); err != nil {
		return err
	}
	for _, dir := range []string{"covers", "languages"} {
		root := filepath.Join(dataDir, dir)
		if _, err := os.Stat(root); err != nil {
			continue
		}
		err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(dataDir, p)
			if err != nil {
				return err
			}
			return addFileToZip(zw, p, filepath.ToSlash(rel))
		})
		if err != nil {
			return err
		}
	}
	return zw.Close()
}

// addFileToZip добавляет файл; картинки уже сжаты — кладём их без повторного сжатия.
func addFileToZip(zw *zip.Writer, src, name string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	method := zip.Deflate
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".avif", ".mp4", ".webm":
		method = zip.Store
	}
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method, Modified: time.Now()})
	if err != nil {
		return err
	}
	_, err = io.Copy(w, in)
	return err
}

// SelectBackupFile открывает диалог выбора архива для импорта.
func (a *App) SelectBackupFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Выберите резервную копию библиотеки",
		Filters: []runtime.FileFilter{{DisplayName: "ZIP (*.zip)", Pattern: "*.zip"}},
	})
}

// ImportLibrary заменяет текущую библиотеку содержимым архива. Возвращает число игр.
// Порядок: распаковка во временную папку → проверка БД → подмена файлов
// (прежние откладываются и возвращаются при сбое) → переоткрытие БД.
func (a *App) ImportLibrary(zipPath string) (int, error) {
	if a.dataDir == "" {
		return 0, fmt.Errorf("data folder is not selected yet")
	}
	tmp := filepath.Join(a.dataDir, ".import-tmp")
	os.RemoveAll(tmp)
	defer os.RemoveAll(tmp)

	if err := extractBackup(zipPath, tmp); err != nil {
		return 0, err
	}
	count, err := prepareImportedDB(filepath.Join(tmp, "games.db"))
	if err != nil {
		return 0, fmt.Errorf("backup database is damaged: %w", err)
	}

	// Закрываем текущую БД, чтобы Windows позволила переместить файл
	a.mu.Lock()
	if old, ok := a.repo.(*db.SQLiteRepo); ok && old != nil {
		old.Close()
	}
	a.repo = nil
	a.mu.Unlock()

	prev := filepath.Join(a.dataDir, ".pre-import")
	os.RemoveAll(prev)
	if err := swapIn(a.dataDir, tmp, prev); err != nil {
		_ = a.initServices(a.dataDir)
		return 0, err
	}
	if err := a.initServices(a.dataDir); err != nil {
		// Новая БД не открылась — возвращаем прежние файлы
		restorePrev(a.dataDir, prev)
		_ = a.initServices(a.dataDir)
		return 0, err
	}
	os.RemoveAll(prev)
	return count, nil
}

// extractBackup распаковывает архив в dst, пропуская всё лишнее и защищаясь от zip-slip.
func extractBackup(zipPath, dst string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("couldn't open archive: %w", err)
	}
	defer zr.Close()

	hasDB := false
	for _, f := range zr.File {
		name := filepath.Clean(filepath.FromSlash(f.Name))
		if f.FileInfo().IsDir() || filepath.IsAbs(name) || strings.HasPrefix(name, "..") {
			continue
		}
		top := strings.SplitN(filepath.ToSlash(name), "/", 2)[0]
		if top != "games.db" && top != "covers" && top != "languages" {
			continue
		}
		if name == "games.db" {
			hasDB = true
		}
		out := filepath.Join(dst, name)
		if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
			return err
		}
		if err := extractZipFile(f, out); err != nil {
			return err
		}
	}
	if !hasDB {
		return fmt.Errorf("this archive is not a pLauncher backup (games.db not found)")
	}
	return nil
}

func extractZipFile(f *zip.File, out string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	w, err := os.Create(out)
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, rc)
	return err
}

// prepareImportedDB открывает распакованную БД (заодно накатывая миграции схемы,
// если архив от старой версии) и приводит пути медиа к виду "covers/…", чтобы они
// работали в текущей папке данных. Возвращает число игр.
func prepareImportedDB(path string) (int, error) {
	repo, err := db.NewSQLiteRepo(path)
	if err != nil {
		return 0, err
	}
	defer repo.Close()
	ctx := context.Background()
	games, err := repo.GetAllGames(ctx)
	if err != nil {
		return 0, err
	}
	for _, g := range games {
		changed := false
		if p := portableMediaPath(g.CoverPath); p != g.CoverPath {
			g.CoverPath, changed = p, true
		}
		for i, img := range g.Images {
			if p := portableMediaPath(img); p != img {
				g.Images[i], changed = p, true
			}
		}
		if changed {
			if err := repo.SaveGame(ctx, g); err != nil {
				return 0, err
			}
		}
	}
	return len(games), nil
}

// portableMediaPath превращает абсолютный путь из чужой папки данных
// ("D:/old/data/covers/x.jpg", "data/covers/x.jpg") в относительный "covers/x.jpg".
func portableMediaPath(p string) string {
	// Не filepath.ToSlash: на Linux/macOS он не трогает «\», а бэкап мог быть сделан на Windows
	s := strings.ReplaceAll(p, `\`, "/")
	if s == "" || strings.HasPrefix(s, "covers/") {
		return p
	}
	if i := strings.Index(strings.ToLower(s), "/covers/"); i >= 0 {
		return s[i+1:]
	}
	return p
}

// swapIn откладывает текущие файлы данных в prev и ставит на их место файлы из src.
// При ошибке возвращает всё как было.
func swapIn(dataDir, src, prev string) error {
	if err := os.MkdirAll(prev, 0755); err != nil {
		return err
	}
	// Вместе с БД уносим её служебные файлы журнала, если они есть
	moveOut := append([]string{"games.db-wal", "games.db-shm", "games.db-journal"}, backupEntries...)
	for _, name := range moveOut {
		cur := filepath.Join(dataDir, name)
		if _, err := os.Stat(cur); err != nil {
			continue
		}
		if err := os.Rename(cur, filepath.Join(prev, name)); err != nil {
			restorePrev(dataDir, prev)
			return fmt.Errorf("couldn't replace %s: %w", name, err)
		}
	}
	for _, name := range backupEntries {
		from := filepath.Join(src, name)
		if _, err := os.Stat(from); err != nil {
			continue // например, в архиве нет languages — папку создаст initServices
		}
		if err := os.Rename(from, filepath.Join(dataDir, name)); err != nil {
			restorePrev(dataDir, prev)
			return fmt.Errorf("couldn't install %s: %w", name, err)
		}
	}
	return nil
}

// restorePrev возвращает отложенные файлы из prev обратно в папку данных.
func restorePrev(dataDir, prev string) {
	entries, err := os.ReadDir(prev)
	if err != nil {
		return
	}
	for _, e := range entries {
		dst := filepath.Join(dataDir, e.Name())
		os.RemoveAll(dst)
		os.Rename(filepath.Join(prev, e.Name()), dst)
	}
}
