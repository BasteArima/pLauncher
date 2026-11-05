package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewerVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.0.1", "1.0.0", true},
		{"1.10.0", "1.9.3", true},
		{"v2.0", "1.99.99", true},
		{"1.0.0", "1.0.0", false},
		{"1.0", "1.0.0", false},
		{"1.0.0-beta", "1.0.0", false},
		{"0.9.9", "1.0.0", false},
		{"1.1.0", "1.1.0-rc.1", true}, // релиз новее своего пре-релиза
		{"1.1.0-rc.2", "1.1.0-rc.1", false},
		{"1.2.0-rc.1", "1.1.0", true},
	}
	for _, c := range cases {
		if got := newerVersion(c.a, c.b); got != c.want {
			t.Errorf("newerVersion(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestWhatsNew(t *testing.T) {
	cases := []struct{ body, want string }{
		{"Intro\n\n## What's new\r\n\r\n- One\n- Two\n\n## Download\n\nfiles", "- One\n- Two"},
		{"## What’s new\n- Curly", "- Curly"},
		{"Intro text\n\n## Highlights\n- A\n## First launch\nx", "- A"},
		{"Only intro\n\n## Download\nfiles", "Only intro"},
		{"No headings at all", "No headings at all"},
		{"## Download\nfiles", "## Download\nfiles"},
		{"", ""},
	}
	for _, c := range cases {
		if got := whatsNew(c.body); got != c.want {
			t.Errorf("whatsNew(%q) = %q, want %q", c.body, got, c.want)
		}
	}
}

// fakeUpdate — поддельный GitHub и exe во временной папке.
type fakeUpdate struct {
	exe     string
	binary  []byte
	mu      sync.Mutex
	corrupt bool
	block   chan struct{} // не nil — загрузка зависает, отдав часть файла
}

const fakeOldBinary = "old"

func setupUpdate(t *testing.T) *fakeUpdate {
	t.Helper()
	asset := assetNameFor(goruntime.GOOS, goruntime.GOARCH)
	if asset == "" {
		t.Skip("на этой ОС автоустановки нет")
	}
	f := &fakeUpdate{binary: []byte("new launcher binary")}
	sum := sha256.Sum256(f.binary)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/test/repo/releases":
			assets := fmt.Sprintf(`[
				{"name":"other-os-build","browser_download_url":"%[1]s/dl/other","size":1},
				{"name":"%[2]s","browser_download_url":"%[1]s/dl/bin","size":%[3]d},
				{"name":"%[2]s.sha256","browser_download_url":"%[1]s/dl/bin.sha256","size":64}]`, srv.URL, asset, len(f.binary))
			fmt.Fprintf(w, `[
				{"tag_name":"v10.0.0","draft":true,"body":"draft","assets":[]},
				{"tag_name":"v11.0.0-rc.1","prerelease":true,"body":"## What's new\n- rc","html_url":"%[1]s/rc","assets":[]},
				{"tag_name":"v9.0.0","body":"## What's new\n- older\n## Download\nfiles","html_url":"%[1]s/v9.0.0","assets":[]},
				{"tag_name":"v9.9.9","body":"## What's new\n- latest\n## Download\nfiles","html_url":"%[1]s/v9.9.9","assets":%[2]s},
				{"tag_name":"v0.5.0","body":"ancient","assets":[]}]`, srv.URL, assets)
		case "/dl/bin":
			f.mu.Lock()
			block := f.block
			f.mu.Unlock()
			if block != nil {
				w.Header().Set("Content-Length", fmt.Sprint(len(f.binary)))
				w.Write(f.binary[:3])
				w.(http.Flusher).Flush()
				select {
				case <-block:
				case <-r.Context().Done():
				}
				return
			}
			w.Write(f.binary)
		case "/dl/bin.sha256":
			f.mu.Lock()
			corrupt := f.corrupt
			f.mu.Unlock()
			if corrupt {
				fmt.Fprint(w, "deadbeef  pLauncher.exe")
				return
			}
			fmt.Fprintf(w, "%s  pLauncher.exe\n", hex.EncodeToString(sum[:]))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	// EvalSymlinks: установщик раскрывает ссылки и короткие имена (RUNNER~1, /private/var)
	dir := t.TempDir()
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	f.exe = filepath.Join(dir, "pLauncher.exe")
	if err := os.WriteFile(f.exe, []byte(fakeOldBinary), 0o755); err != nil {
		t.Fatal(err)
	}

	savedAPI, savedRepo, savedVer, savedExe := githubAPI, updateRepo, appVersion, executablePath
	savedStart, savedQuit, savedTimeout, savedFlags := startNewVersion, quitApp, updateConfirmTimeout, startFlags
	githubAPI, updateRepo, appVersion = srv.URL, "test/repo", "1.0.0"
	executablePath = func() (string, error) { return f.exe, nil }
	t.Cleanup(func() {
		githubAPI, updateRepo, appVersion, executablePath = savedAPI, savedRepo, savedVer, savedExe
		startNewVersion, quitApp, updateConfirmTimeout, startFlags = savedStart, savedQuit, savedTimeout, savedFlags
	})
	return f
}

// fakeStart подменяет запуск новой версии. behave получает канал завершения процесса.
type startCall struct {
	exe  string
	args []string
}

func fakeStart(t *testing.T, behave func(n int, exe string, exited chan error)) *[]startCall {
	t.Helper()
	var mu sync.Mutex
	calls := &[]startCall{}
	startNewVersion = func(exe string, args []string) (<-chan error, func(), error) {
		mu.Lock()
		*calls = append(*calls, startCall{exe, args})
		n := len(*calls)
		mu.Unlock()
		exited := make(chan error, 1)
		behave(n, exe, exited)
		return exited, func() { exited <- nil }, nil
	}
	return calls
}

// Новая версия запустилась: интерфейс загружен, метка снята.
func startsFine(_ int, exe string, _ chan error) { os.Remove(exe + suffixCheck) }

func catchQuit() chan struct{} {
	quit := make(chan struct{})
	var once sync.Once
	quitApp = func(*App) { once.Do(func() { close(quit) }) }
	return quit
}

func waitChan(t *testing.T, ch chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatalf("не дождались: %s", what)
	}
}

func waitDownload(t *testing.T, a *App, state string) LauncherDownload {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if d := a.GetLauncherDownload(); d.State == state {
			return d
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("загрузка не перешла в %q: %+v", state, a.GetLauncherDownload())
	return LauncherDownload{}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestCheckLauncherUpdate(t *testing.T) {
	setupUpdate(t)
	a := &App{}

	u, err := a.CheckLauncherUpdate()
	if err != nil {
		t.Fatal(err)
	}
	if !u.Available || !u.CanInstall || u.Latest != "9.9.9" || u.ManualReason != "" {
		t.Fatalf("check: %+v", u)
	}
	// Черновик и пре-релиз не видны; пропущенные версии — новые первыми, только «что нового»
	want := []LauncherRelease{
		{Version: "9.9.9", Notes: "- latest", URL: u.PageURL},
		{Version: "9.0.0", Notes: "- older", URL: strings.TrimSuffix(u.PageURL, "9.9.9") + "9.0.0"},
	}
	if !reflect.DeepEqual(u.Releases, want) {
		t.Errorf("releases = %+v", u.Releases)
	}

	t.Setenv(prereleaseEnv, "1")
	if u, _ := a.CheckLauncherUpdate(); u.Latest != "11.0.0-rc.1" {
		t.Errorf("с пре-релизами latest = %q", u.Latest)
	}
	os.Unsetenv(prereleaseEnv)

	appVersion = "dev"
	if u, _ := a.CheckLauncherUpdate(); u.Available || u.Latest != "9.9.9" {
		t.Errorf("dev-сборка не должна предлагать обновление: %+v", u)
	}
}

// Полный цикл: скачали → файл ещё старый → установка → новая версия подтвердила запуск.
func TestDownloadAndApply(t *testing.T) {
	f := setupUpdate(t)
	dataDir := t.TempDir()
	os.WriteFile(filepath.Join(dataDir, "games.db"), []byte("database"), 0o644)
	calls := fakeStart(t, startsFine)
	quit := catchQuit()

	a := &App{dataDir: dataDir}
	if err := a.ApplyLauncherUpdate(false); err == nil {
		t.Fatal("без загрузки установка должна отказывать")
	}
	if err := a.StartLauncherDownload(); err != nil {
		t.Fatal(err)
	}
	d := waitDownload(t, a, "ready")
	if d.Version != "9.9.9" || d.Done != int64(len(f.binary)) {
		t.Errorf("загрузка: %+v", d)
	}
	if readFile(t, f.exe) != fakeOldBinary || !exists(f.exe+suffixNew) || exists(f.exe+suffixPart) {
		t.Fatal("до перезапуска работающий exe не должен меняться")
	}

	if err := a.ApplyLauncherUpdate(true); err != nil {
		t.Fatal(err)
	}
	waitChan(t, quit, "завершение старой версии")

	if readFile(t, f.exe) != string(f.binary) {
		t.Errorf("exe не заменён")
	}
	if readFile(t, f.exe+suffixOld) != fakeOldBinary {
		t.Errorf("старый exe не отложен в .old")
	}
	if exists(f.exe+suffixNew) || exists(f.exe+suffixVerified) || exists(f.exe+suffixCheck) {
		t.Errorf("остались временные файлы")
	}
	if want := []startCall{{f.exe, []string{afterUpdateFlag, startDiscreetFlag}}}; !reflect.DeepEqual(*calls, want) {
		t.Errorf("запуск: %+v", *calls)
	}
	if readFile(t, filepath.Join(dataDir, "backups", "games-before-9.9.9.db")) != "database" {
		t.Errorf("нет копии базы")
	}

	// Новая версия при запуске сообщает об обновлении один раз
	startFlags.afterUpdate, startFlags.discreet = true, true
	appVersion = "9.9.9"
	if info := a.GetLauncherStartInfo(); info.UpdatedTo != "9.9.9" || !info.Discreet || info.FailedUpdate != "" {
		t.Errorf("start info: %+v", info)
	}
	confirmUpdateStarted()
	deadline := time.Now().Add(5 * time.Second)
	for exists(f.exe+suffixOld) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if exists(f.exe + suffixOld) {
		t.Errorf(".old не удалён после подтверждения")
	}
}

// Новая версия падает при запуске или зависает — возвращаемся на прежнюю.
func TestRollback(t *testing.T) {
	cases := map[string]func(int, string, chan error){
		"падает": func(n int, exe string, exited chan error) {
			if n == 1 {
				exited <- fmt.Errorf("exit status 2")
			}
		},
		"зависает": func(int, string, chan error) {},
	}
	for name, behave := range cases {
		t.Run(name, func(t *testing.T) {
			f := setupUpdate(t)
			updateConfirmTimeout = 300 * time.Millisecond
			calls := fakeStart(t, behave)
			quit := catchQuit()

			a := &App{}
			if err := a.StartLauncherDownload(); err != nil {
				t.Fatal(err)
			}
			waitDownload(t, a, "ready")
			if err := a.ApplyLauncherUpdate(false); err != nil {
				t.Fatal(err)
			}
			waitChan(t, quit, "завершение")

			if readFile(t, f.exe) != fakeOldBinary {
				t.Errorf("exe не возвращён")
			}
			if exists(f.exe+suffixOld) || exists(f.exe+suffixCheck) {
				t.Errorf("остались файлы отката")
			}
			if len(*calls) != 2 || !reflect.DeepEqual((*calls)[1].args, []string{afterUpdateFlag}) {
				t.Errorf("прежняя версия не перезапущена: %+v", *calls)
			}
			// Сообщение показывается один раз и не выдаётся за успешное обновление
			startFlags.afterUpdate = true
			if info := a.GetLauncherStartInfo(); info.FailedUpdate != "9.9.9" || info.UpdatedTo != "" {
				t.Errorf("start info: %+v", info)
			}
			if info := a.GetLauncherStartInfo(); info.FailedUpdate != "" {
				t.Errorf("сообщение об откате повторилось")
			}
		})
	}
}

func TestDownloadErrors(t *testing.T) {
	f := setupUpdate(t)
	a := &App{}

	// Битая контрольная сумма: ничего не ставим
	f.corrupt = true
	if err := a.StartLauncherDownload(); err != nil {
		t.Fatal(err)
	}
	d := waitDownload(t, a, "error")
	if !strings.Contains(d.Error, "update.checksum") {
		t.Errorf("ошибка: %q", d.Error)
	}
	if readFile(t, f.exe) != fakeOldBinary || exists(f.exe+suffixPart) || exists(f.exe+suffixNew) {
		t.Errorf("при битой сумме файлы изменены")
	}

	// После ошибки можно повторить
	f.corrupt = false
	if err := a.StartLauncherDownload(); err != nil {
		t.Fatal(err)
	}
	waitDownload(t, a, "ready")
}

func TestCancelDownload(t *testing.T) {
	f := setupUpdate(t)
	f.block = make(chan struct{})
	defer close(f.block)
	a := &App{}
	if err := a.StartLauncherDownload(); err != nil {
		t.Fatal(err)
	}
	waitDownload(t, a, "downloading")
	a.CancelLauncherDownload()
	waitDownload(t, a, "idle")
	if exists(f.exe + suffixPart) {
		t.Errorf("остался недокачанный файл")
	}
}

// «При следующем запуске»: закрыли лаунчер с готовой версией — ставится при старте.
func TestApplyAtNextLaunch(t *testing.T) {
	f := setupUpdate(t)
	calls := fakeStart(t, startsFine)

	a := &App{}
	if err := a.StartLauncherDownload(); err != nil {
		t.Fatal(err)
	}
	waitDownload(t, a, "ready")

	// Лаунчер ещё работает (второй запуск exe): ставить нельзя
	if applyPendingAtStartup("") {
		t.Fatal("пока лаунчер работает, обновление не применяется")
	}
	a.keepPendingForNextLaunch()
	if !applyPendingAtStartup("") {
		t.Fatal("ожидающая версия не установлена")
	}
	if readFile(t, f.exe) != string(f.binary) || len(*calls) != 1 {
		t.Errorf("exe не заменён, запуски %+v", *calls)
	}
}

func TestPendingRejected(t *testing.T) {
	f := setupUpdate(t)
	calls := fakeStart(t, startsFine)
	put := func(version, sum string) {
		os.WriteFile(f.exe+suffixNew, f.binary, 0o755)
		os.WriteFile(f.exe+suffixOK, []byte(version+" "+sum), 0o644)
	}
	sum := sha256.Sum256(f.binary)
	good := hex.EncodeToString(sum[:])

	put("9.9.9", "deadbeef") // файл изменился после проверки
	if applyPendingAtStartup("") || exists(f.exe+suffixNew) {
		t.Errorf("подменённый файл не должен ставиться")
	}
	put("1.0.0", good) // уже не новее текущей
	if applyPendingAtStartup("") || exists(f.exe+suffixOK) {
		t.Errorf("старая версия не должна ставиться")
	}
	if readFile(t, f.exe) != fakeOldBinary || len(*calls) != 0 {
		t.Errorf("exe тронут: %+v", *calls)
	}
}

func TestBackupDatabasePrunes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "games.db"), []byte("db"), 0o644)
	backups := filepath.Join(dir, "backups")
	for i, v := range []string{"1.1.0", "1.2.0", "1.3.0", "1.4.0"} {
		backupDatabase(dir, v)
		// Явные времена: на Windows точность часов может быть низкой
		mod := time.Now().Add(time.Duration(i-10) * time.Hour)
		os.Chtimes(filepath.Join(backups, "games-before-"+v+".db"), mod, mod)
	}
	backupDatabase(dir, "1.5.0")
	files, _ := filepath.Glob(filepath.Join(backups, "*.db"))
	if len(files) != maxDBBackups {
		t.Fatalf("копий: %v", files)
	}
	if exists(filepath.Join(backups, "games-before-1.1.0.db")) || !exists(filepath.Join(backups, "games-before-1.5.0.db")) {
		t.Errorf("удалены не самые старые: %v", files)
	}
	backupDatabase("", "1.0.0")                       // папка данных ещё не выбрана — ничего не делаем
	backupDatabase(filepath.Join(dir, "no"), "1.0.0") // базы нет — тоже
}

func TestRestartBlocker(t *testing.T) {
	a := &App{}
	if got := a.GetLauncherRestartBlocker(); got != "" {
		t.Errorf("blocker = %q", got)
	}
	a.priv.panicHidden = true
	if got := a.GetLauncherRestartBlocker(); got != "hidden" {
		t.Errorf("blocker = %q, want hidden", got)
	}
}
