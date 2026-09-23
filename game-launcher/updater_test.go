package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"
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
	}
	for _, c := range cases {
		if got := newerVersion(c.a, c.b); got != c.want {
			t.Errorf("newerVersion(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// Полный цикл: релиз с exe и контрольной суммой → скачивание → подмена файла.
func TestInstallLauncherUpdate(t *testing.T) {
	asset := assetNameFor(goruntime.GOOS, goruntime.GOARCH)
	if asset == "" {
		t.Skip("на этой ОС автоустановки нет")
	}
	newBinary := []byte("new launcher binary")
	sum := sha256.Sum256(newBinary)
	corrupt := false

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/test/repo/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v9.9.9","body":"notes","html_url":"%[1]s/rel","assets":[
				{"name":"other-os-build","browser_download_url":"%[1]s/dl/other","size":1},
				{"name":"%[3]s","browser_download_url":"%[1]s/dl/bin","size":%[2]d},
				{"name":"%[3]s.sha256","browser_download_url":"%[1]s/dl/bin.sha256","size":64}]}`, srv.URL, len(newBinary), asset)
		case "/dl/bin":
			w.Write(newBinary)
		case "/dl/bin.sha256":
			if corrupt {
				fmt.Fprint(w, "deadbeef  pLauncher.exe")
				return
			}
			fmt.Fprintf(w, "%s  pLauncher.exe\n", hex.EncodeToString(sum[:]))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	exe := filepath.Join(dir, "pLauncher.exe")
	os.WriteFile(exe, []byte("old"), 0755)

	restarted := ""
	saved := [...]interface{}{githubAPI, updateRepo, appVersion, executablePath, restartApp}
	githubAPI, updateRepo, appVersion = srv.URL, "test/repo", "1.0.0"
	executablePath = func() (string, error) { return exe, nil }
	restartApp = func(a *App, p string) error { restarted = p; return nil }
	defer func() {
		githubAPI, updateRepo, appVersion = saved[0].(string), saved[1].(string), saved[2].(string)
		executablePath = saved[3].(func() (string, error))
		restartApp = saved[4].(func(*App, string) error)
	}()

	a := &App{}
	u, err := a.CheckLauncherUpdate()
	if err != nil || !u.Available || u.Latest != "9.9.9" {
		t.Fatalf("check: %+v, %v", u, err)
	}

	// Битая контрольная сумма — файл не трогаем
	corrupt = true
	if err := a.InstallLauncherUpdate(); err == nil {
		t.Fatal("ожидалась ошибка контрольной суммы")
	}
	if b, _ := os.ReadFile(exe); string(b) != "old" {
		t.Fatalf("exe изменён при битой сумме: %q", b)
	}

	corrupt = false
	if err := a.InstallLauncherUpdate(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != string(newBinary) {
		t.Errorf("exe = %q, want new binary", b)
	}
	if b, _ := os.ReadFile(exe + ".old"); string(b) != "old" {
		t.Errorf("старый exe не отложен в .old")
	}
	if restarted != exe {
		t.Errorf("restart = %q, want %q", restarted, exe)
	}

	// dev-сборки не обновляются
	appVersion = "dev"
	if u, _ := a.CheckLauncherUpdate(); u.Available {
		t.Error("dev-сборка не должна предлагать обновление")
	}
}
