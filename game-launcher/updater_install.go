package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"game-launcher/internal/sessionlock"
	"hash"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"game-launcher/internal/apperr"
)

// =========================================================================
// САМООБНОВЛЕНИЕ ЛАУНЧЕРА: загрузка и установка
//
// Файлы рядом с exe:
//   .new.part      качается сейчас
//   .new           скачан и проверен
//   .new.verified  версия и SHA-256 файла .new (лаунчер ещё работает)
//   .new.ok        то же, лаунчер закрыт штатно: поставить при следующем запуске
//   .old           прежний exe, пока новая версия не подтвердила запуск
//   .update-check  метка «ждём подтверждения»: её снимает новая версия
//   .update-failed версия, с которой пришлось откатиться (для сообщения пользователю)
//
// Установка: exe → .old, .new → exe, запуск новой версии. Старая версия ждёт, пока
// новая снимет метку (интерфейс загрузился), и только тогда завершается. Новая упала
// или зависла — файлы возвращаются обратно. Запущенный exe в Windows нельзя перезаписать,
// но можно переименовать, поэтому .old удаляется уже новой версией.
// =========================================================================

const (
	afterUpdateFlag    = "--after-update"    // запуск сразу после обновления
	startDiscreetFlag  = "--start-discreet"  // обложки были скрыты — скрыть и после перезапуска
	startMinimizedFlag = "--start-minimized" // окно было свёрнуто — оставить свёрнутым

	suffixPart     = ".new.part"
	suffixNew      = ".new"
	suffixVerified = ".new.verified"
	suffixOK       = ".new.ok"
	suffixOld      = ".old"
	suffixFailedEx = ".failed"
	suffixCheck    = ".update-check"
	suffixFailed   = ".update-failed"

	downloadStallTimeout = 60 * time.Second // данных нет так долго — считаем загрузку оборвавшейся
	progressInterval     = 150 * time.Millisecond
	maxDBBackups         = 3
)

// Подменяются в тестах.
var (
	executablePath       = os.Executable
	updateConfirmTimeout = 60 * time.Second
	// startNewVersion запускает exe и возвращает канал завершения и функцию остановки процесса.
	startNewVersion = func(exe string, args []string) (<-chan error, func(), error) {
		cmd := exec.Command(exe, args...)
		if err := cmd.Start(); err != nil {
			return nil, nil, err
		}
		exited := make(chan error, 1)
		go func() { exited <- cmd.Wait() }()
		return exited, func() { cmd.Process.Kill() }, nil
	}
	quitApp = func(a *App) { runtime.Quit(a.ctx) }
)

// Как запущен текущий процесс (заполняется в main).
var startFlags struct{ afterUpdate, discreet, minimized bool }

func currentExe() (string, error) {
	exe, err := executablePath()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return exe, nil
}

// LauncherDownload — состояние загрузки для интерфейса (и событие "launcher-download").
type LauncherDownload struct {
	State   string `json:"state"` // idle | downloading | ready | error
	Version string `json:"version"`
	Done    int64  `json:"done"`
	Total   int64  `json:"total"`
	Speed   int64  `json:"speed"` // байт/с
	Error   string `json:"error"`
}

type updateState struct {
	mu       sync.Mutex
	dl       LauncherDownload
	cancel   context.CancelFunc
	applying bool
}

func (a *App) publishDownload() {
	a.upd.mu.Lock()
	d := a.upd.dl
	a.upd.mu.Unlock()
	a.emit("launcher-download", d)
}

func (a *App) setDownload(f func(*LauncherDownload)) {
	a.upd.mu.Lock()
	f(&a.upd.dl)
	a.upd.mu.Unlock()
	a.publishDownload()
}

// GetLauncherDownload возвращает текущее состояние загрузки.
func (a *App) GetLauncherDownload() LauncherDownload {
	a.upd.mu.Lock()
	defer a.upd.mu.Unlock()
	if a.upd.dl.State == "" {
		return LauncherDownload{State: "idle"}
	}
	return a.upd.dl
}

// StartLauncherDownload начинает фоновую загрузку новой версии. Ход — в событии "launcher-download".
func (a *App) StartLauncherDownload() error {
	a.upd.mu.Lock()
	busy := a.upd.dl.State == "downloading" || a.upd.dl.State == "ready"
	a.upd.mu.Unlock()
	if busy {
		return nil
	}
	u, err := a.CheckLauncherUpdate()
	if err != nil {
		return err
	}
	if !u.Available {
		return fmt.Errorf("already up to date (%s)", u.Current)
	}
	if !u.CanInstall {
		if u.ManualReason == "readonly" {
			dir := ""
			if exe, err := currentExe(); err == nil {
				dir = filepath.Dir(exe)
			}
			return apperr.New("update.no_write", apperr.P{"path": dir}, "no write access to %s", dir)
		}
		return apperr.New("update.manual", nil, "automatic update isn’t available on this system — download it from the release page")
	}
	exe, err := currentExe()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.upd.mu.Lock()
	a.upd.cancel = cancel
	a.upd.dl = LauncherDownload{State: "downloading", Version: u.Latest, Total: u.Size}
	a.upd.mu.Unlock()
	a.publishDownload()

	go func() {
		defer cancel()
		sum, err := a.downloadRelease(ctx, u, exe)
		switch {
		case errors.Is(ctx.Err(), context.Canceled): // отменил пользователь
			removeFiles(exe, suffixPart)
			a.setDownload(func(d *LauncherDownload) { *d = LauncherDownload{State: "idle"} })
		case err != nil:
			removeFiles(exe, suffixPart)
			fmt.Printf("Обновление: %v\n", err)
			a.setDownload(func(d *LauncherDownload) { d.State, d.Error = "error", err.Error() })
		default:
			if err := finishDownload(exe, u.Latest, sum); err != nil {
				a.setDownload(func(d *LauncherDownload) { d.State, d.Error = "error", err.Error() })
				return
			}
			a.setDownload(func(d *LauncherDownload) { d.State, d.Done, d.Speed = "ready", d.Total, 0 })
		}
	}()
	return nil
}

// CancelLauncherDownload прерывает идущую загрузку.
func (a *App) CancelLauncherDownload() {
	a.upd.mu.Lock()
	cancel := a.upd.cancel
	a.upd.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// downloadRelease скачивает сборку в .new.part и сверяет контрольную сумму. Возвращает SHA-256.
func (a *App) downloadRelease(ctx context.Context, u *LauncherUpdate, exe string) (string, error) {
	h := sha256.New()
	err := downloadTo(ctx, u.AssetURL, exe+suffixPart, u.Size, h, func(done, total, speed int64) {
		a.setDownload(func(d *LauncherDownload) {
			d.Done, d.Speed = done, speed
			if total > 0 {
				d.Total = total
			}
		})
	})
	if err != nil {
		if ctx.Err() != nil {
			return "", err
		}
		return "", apperr.New("update.download", nil, "download failed: %w (you can download it manually from the release page)", err)
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if u.shaURL != "" {
		if err := verifySHA256(ctx, sum, u.shaURL); err != nil {
			return "", err
		}
	}
	return sum, nil
}

// finishDownload делает скачанный файл «готовым к установке».
func finishDownload(exe, version, sum string) error {
	if goruntime.GOOS != "windows" {
		os.Chmod(exe+suffixPart, 0o755) // скачанный файл без бита исполнения
	}
	if err := os.Rename(exe+suffixPart, exe+suffixNew); err != nil {
		return err
	}
	return os.WriteFile(exe+suffixVerified, []byte(version+" "+sum), 0o644)
}

// downloadTo качает url в dest: прогресс через onProgress, зависшую передачу обрывает.
func downloadTo(ctx context.Context, url, dest string, wantSize int64, h hash.Hash, onProgress func(done, total, speed int64)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "pLauncher/"+appVersion)
	// Без общего таймаута клиента: на медленном канале загрузка законно идёт долго
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server answered %s", resp.Status)
	}
	total := wantSize
	if total <= 0 {
		total = resp.ContentLength
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	stall := time.AfterFunc(downloadStallTimeout, cancel)
	defer stall.Stop()

	var done, lastDone int64
	var speed int64
	lastReport := time.Now()
	buf := make([]byte, 64*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			stall.Reset(downloadStallTimeout)
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			h.Write(buf[:n])
			done += int64(n)
			if since := time.Since(lastReport); since >= progressInterval {
				speed = int64(float64(done-lastDone) / since.Seconds())
				lastDone, lastReport = done, time.Now()
				onProgress(done, total, speed)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	if total > 0 && done != total {
		return fmt.Errorf("incomplete file: %d of %d bytes", done, total)
	}
	onProgress(done, total, speed)
	return f.Close()
}

// verifySHA256 сверяет сумму файла с опубликованной в виде "<hex>  pLauncher.exe".
func verifySHA256(ctx context.Context, sum, shaURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, shaURL, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't get checksum: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return err
	}
	fields := strings.Fields(string(raw))
	if resp.StatusCode != http.StatusOK || len(fields) == 0 {
		return fmt.Errorf("couldn't get checksum (%s)", resp.Status)
	}
	if !strings.EqualFold(sum, fields[0]) {
		return apperr.New("update.checksum", nil, "checksum mismatch: the downloaded file is damaged")
	}
	return nil
}

// GetLauncherRestartBlocker — почему перезапускаться сейчас нежелательно: "game" (идёт игра,
// оборвётся подсчёт времени), "hidden" (окно спрятано кнопкой паники) или "".
func (a *App) GetLauncherRestartBlocker() string {
	if a.dataDir != "" {
		active, err := sessionlock.Active(a.dataDir)
		if active || err != nil {
			return "game"
		}
	}
	a.priv.mu.Lock()
	hidden := a.priv.panicHidden
	a.priv.mu.Unlock()
	if hidden {
		return "hidden"
	}
	return ""
}

// ApplyLauncherUpdate ставит скачанную версию и перезапускает лаунчер. discreet — обложки
// сейчас скрыты: новая версия запустится с ними же скрытыми.
func (a *App) ApplyLauncherUpdate(discreet bool) error {
	release, err := a.guardSessionData()
	if err != nil {
		return err
	}
	transferred := false
	defer func() {
		if !transferred {
			release()
		}
	}()
	a.upd.mu.Lock()
	ready := a.upd.dl.State == "ready"
	version := a.upd.dl.Version
	a.upd.mu.Unlock()
	if !ready {
		return fmt.Errorf("update isn't downloaded yet")
	}
	exe, err := currentExe()
	if err != nil {
		return err
	}
	if err := verifyPending(exe, suffixVerified, version); err != nil {
		removeFiles(exe, suffixNew, suffixVerified)
		a.setDownload(func(d *LauncherDownload) { *d = LauncherDownload{State: "idle"} })
		return err
	}

	var args []string
	if discreet {
		args = append(args, startDiscreetFlag)
	}
	if a.live && windowMinimised(a.ctx) {
		args = append(args, startMinimizedFlag)
	}
	a.upd.mu.Lock()
	a.upd.applying = true
	a.upd.mu.Unlock()

	transferred = true
	go func() {
		defer release()
		// Освобождаем всё, что нужно новой версии: БД и глобальную горячую клавишу
		if a.live {
			runtime.WindowHide(a.ctx)
		}
		a.closeServices()
		if err := installAndSupervise(exe, version, a.dataDir, args); err != nil {
			fmt.Printf("Обновление: %v\n", err)
			// Файлы уже возвращены: снова запускаем прежнюю версию (замок экземпляра ещё у нас)
			if _, _, serr := startNewVersion(exe, append([]string{afterUpdateFlag}, args...)); serr != nil {
				fmt.Printf("Обновление: не удалось запустить прежнюю версию: %v\n", serr)
			}
		}
		quitApp(a)
	}()
	return nil
}

func windowMinimised(ctx context.Context) (min bool) {
	defer func() { recover() }() // на закрывающемся окне Wails может паниковать
	return runtime.WindowIsMinimised(ctx)
}

// installAndSupervise подменяет exe, запускает новую версию и ждёт подтверждения.
// Новая версия не подтвердила запуск — возвращает файлы и ошибку.
func installAndSupervise(exe, version, dataDir string, args []string) error {
	backupDatabase(dataDir, version)

	os.Remove(exe + suffixOld)
	if err := os.Rename(exe, exe+suffixOld); err != nil {
		removeFiles(exe, suffixNew, suffixVerified, suffixOK)
		markUpdateFailed(exe, version)
		return apperr.New("update.no_write", apperr.P{"path": filepath.Dir(exe)}, "couldn't replace the launcher file (no write access to %s?): %w", filepath.Dir(exe), err)
	}
	if err := os.Rename(exe+suffixNew, exe); err != nil {
		os.Rename(exe+suffixOld, exe)
		removeFiles(exe, suffixNew, suffixVerified, suffixOK)
		markUpdateFailed(exe, version)
		return err
	}
	removeFiles(exe, suffixVerified, suffixOK)
	if err := os.WriteFile(exe+suffixCheck, []byte(version), 0o644); err != nil {
		rollbackUpdate(exe, version)
		return err
	}

	exited, kill, err := startNewVersion(exe, append([]string{afterUpdateFlag}, args...))
	if err != nil {
		rollbackUpdate(exe, version)
		return fmt.Errorf("couldn't start version %s: %w", version, err)
	}
	if !waitUpdateConfirmed(exe+suffixCheck, exited, updateConfirmTimeout) {
		kill()
		rollbackUpdate(exe, version)
		return fmt.Errorf("version %s didn't start, rolled back", version)
	}
	return nil
}

// waitUpdateConfirmed ждёт, пока новая версия снимет метку. false — процесс завершился
// раньше или не успел за timeout.
func waitUpdateConfirmed(marker string, exited <-chan error, timeout time.Duration) bool {
	confirmed := func() bool {
		_, err := os.Stat(marker)
		return os.IsNotExist(err)
	}
	deadline := time.After(timeout)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if confirmed() {
				return true
			}
		case <-exited:
			return confirmed()
		case <-deadline:
			return confirmed()
		}
	}
}

// rollbackUpdate возвращает прежний exe на место.
func rollbackUpdate(exe, version string) {
	os.Remove(exe + suffixFailedEx)
	if err := os.Rename(exe, exe+suffixFailedEx); err != nil {
		fmt.Printf("Обновление: откат: %v\n", err)
	}
	if err := os.Rename(exe+suffixOld, exe); err != nil {
		fmt.Printf("Обновление: откат: %v\n", err)
	}
	os.Remove(exe + suffixCheck)
	os.Remove(exe + suffixFailedEx) // не вышло (процесс ещё завершается) — уберёт следующий запуск
	markUpdateFailed(exe, version)
}

func markUpdateFailed(exe, version string) {
	os.WriteFile(exe+suffixFailed, []byte(version), 0o644)
}

// verifyPending сверяет файл .new с версией и суммой из sidecar.
func verifyPending(exe, sidecar, version string) error {
	raw, err := os.ReadFile(exe + sidecar)
	if err != nil {
		return err
	}
	fields := strings.Fields(string(raw))
	if len(fields) != 2 || (version != "" && fields[0] != version) {
		return fmt.Errorf("bad update metadata")
	}
	f, err := os.Open(exe + suffixNew)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), fields[1]) {
		return apperr.New("update.checksum", nil, "checksum mismatch: the downloaded file is damaged")
	}
	return nil
}

// applyPendingAtStartup ставит версию, скачанную в прошлый запуск («При следующем запуске»).
// Вызывается из main до открытия окна. true — новая версия запущена, этому процессу пора выйти.
func applyPendingAtStartup(dataDir string) bool {
	if dataDir != "" {
		l, err := sessionlock.Try(dataDir, "gate")
		if err != nil {
			return false
		}
		defer l.Close()
		active, err := sessionlock.Active(dataDir)
		if active || err != nil {
			return false
		}
	}
	exe, err := currentExe()
	if err != nil {
		return false
	}
	raw, err := os.ReadFile(exe + suffixOK)
	if err != nil {
		return false // ожидающего обновления нет
	}
	fields := strings.Fields(string(raw))
	if len(fields) != 2 || appVersion == "dev" || !newerVersion(fields[0], appVersion) {
		removeFiles(exe, suffixNew, suffixOK)
		return false
	}
	if err := verifyPending(exe, suffixOK, fields[0]); err != nil {
		fmt.Printf("Обновление: %v\n", err)
		removeFiles(exe, suffixNew, suffixOK)
		return false
	}
	if err := installAndSupervise(exe, fields[0], dataDir, nil); err != nil {
		fmt.Printf("Обновление: %v\n", err)
		return false // файлы возвращены: продолжаем работать на текущей версии
	}
	return true
}

// keepPendingForNextLaunch вызывается при закрытии: скачанная версия поставится при следующем запуске.
func (a *App) keepPendingForNextLaunch() {
	a.upd.mu.Lock()
	ready := a.upd.dl.State == "ready" && !a.upd.applying
	a.upd.mu.Unlock()
	if !ready {
		return
	}
	if exe, err := currentExe(); err == nil {
		os.Rename(exe+suffixVerified, exe+suffixOK)
	}
}

// backupDatabase копирует games.db в <данные>/backups перед установкой. БД к этому моменту
// закрыта. Ошибка копирования обновление не останавливает: база при обновлении не меняется.
func backupDatabase(dataDir, version string) {
	if dataDir == "" {
		return
	}
	src, err := os.Open(filepath.Join(dataDir, "games.db"))
	if err != nil {
		return
	}
	defer src.Close()
	dir := filepath.Join(dataDir, "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Printf("Обновление: копия базы: %v\n", err)
		return
	}
	dst, err := os.Create(filepath.Join(dir, "games-before-"+version+".db"))
	if err != nil {
		fmt.Printf("Обновление: копия базы: %v\n", err)
		return
	}
	_, cerr := io.Copy(dst, src)
	if err := dst.Close(); cerr == nil {
		cerr = err
	}
	if cerr != nil {
		fmt.Printf("Обновление: копия базы: %v\n", cerr)
		os.Remove(dst.Name())
		return
	}
	pruneDBBackups(dir)
}

// pruneDBBackups оставляет maxDBBackups самых свежих копий.
func pruneDBBackups(dir string) {
	files, _ := filepath.Glob(filepath.Join(dir, "games-before-*.db"))
	type item struct {
		path string
		mod  time.Time
	}
	var items []item
	for _, f := range files {
		if st, err := os.Stat(f); err == nil {
			items = append(items, item{f, st.ModTime()})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].mod.After(items[j].mod) })
	for i := maxDBBackups; i < len(items); i++ {
		os.Remove(items[i].path)
	}
}

func removeFiles(exe string, suffixes ...string) {
	for _, s := range suffixes {
		os.Remove(exe + s)
	}
}

// cleanupAfterUpdate убирает остатки обновлений. Вызывается в основном экземпляре (из
// startup): второй запуск exe не должен трогать файлы работающего лаунчера.
func cleanupAfterUpdate() {
	exe, err := currentExe()
	if err != nil {
		return
	}
	removeFiles(exe, suffixPart, suffixNew, suffixVerified, suffixOK, suffixFailedEx)
	if _, err := os.Stat(exe + suffixCheck); os.IsNotExist(err) {
		os.Remove(exe + suffixOld) // иначе .old ещё нужен: его уберёт confirmUpdateStarted
	}
}

// confirmUpdateStarted снимает метку: интерфейс загрузился, значит новая версия рабочая.
// Прежний процесс после этого завершается и освобождает свой exe.
func confirmUpdateStarted() {
	exe, err := currentExe()
	if err != nil {
		return
	}
	os.Remove(exe + suffixCheck)
	go func() {
		for i := 0; i < 40; i++ {
			err := os.Remove(exe + suffixOld)
			if err == nil || os.IsNotExist(err) {
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}()
}

// LauncherStartInfo — что сообщить пользователю при запуске.
type LauncherStartInfo struct {
	Discreet     bool   `json:"discreet"`      // обложки были скрыты до перезапуска
	UpdatedTo    string `json:"updated_to"`    // лаунчер только что обновился до этой версии
	FailedUpdate string `json:"failed_update"` // эта версия не запустилась, вернулись на прежнюю
}

// GetLauncherStartInfo читает результат прошлого обновления (сообщение показывается один раз).
func (a *App) GetLauncherStartInfo() LauncherStartInfo {
	info := LauncherStartInfo{Discreet: startFlags.discreet}
	if exe, err := currentExe(); err == nil {
		if raw, err := os.ReadFile(exe + suffixFailed); err == nil {
			info.FailedUpdate = strings.TrimSpace(string(raw))
			os.Remove(exe + suffixFailed)
		}
	}
	if startFlags.afterUpdate && info.FailedUpdate == "" {
		info.UpdatedTo = appVersion
	}
	return info
}
