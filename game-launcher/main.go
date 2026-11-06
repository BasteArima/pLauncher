package main

import (
	"embed"
	"game-launcher/internal/launcher"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// Английский словарь — встроен, чтобы класть как готовый шаблон в data/languages при первом запуске.
//
//go:embed frontend/src/locales/en.json
var exampleLocale []byte

// appVersion вшивается при сборке: -ldflags "-X main.appVersion=1.2.3" (см. build.ps1 и CI).
var appVersion = "dev"

func main() {
	if len(os.Args) == 4 && os.Args[1] == launcher.MonitorFlag {
		if err := launcher.RunMonitor(os.Args[2], os.Args[3], os.Stdout); err != nil {
			os.Exit(1)
		}
		return
	}
	for _, arg := range os.Args[1:] {
		switch arg {
		case afterUpdateFlag:
			startFlags.afterUpdate = true
		case startDiscreetFlag:
			startFlags.discreet = true
		case startMinimizedFlag:
			startFlags.minimized = true
		}
	}

	cfg := loadConfig()
	if startFlags.afterUpdate {
		// Новый установщик сам освобождает БД и ждёт подтверждения, а версия 1.0 просто
		// завершается после запуска: её метки нет, даём ей время закрыться.
		if exe, err := currentExe(); err == nil {
			if _, err := os.Stat(exe + suffixCheck); os.IsNotExist(err) {
				time.Sleep(1500 * time.Millisecond)
			}
		}
	} else if applyPendingAtStartup(cfg.DataDir) {
		return // версию, скачанную в прошлый раз, поставили и запустили
	}

	app := NewApp()

	// Восстанавливаем размер окна из конфига (по умолчанию 1280x768)
	width, height := 1280, 768
	if cfg.WindowWidth >= 800 && cfg.WindowHeight >= 500 {
		width, height = cfg.WindowWidth, cfg.WindowHeight
	}

	opts := &options.App{
		Title:     "pLauncher",
		Width:     width,
		Height:    height,
		MinWidth:  900,
		MinHeight: 560,
		Frameless: true, // своя рамка/титлбар вместо системной
		AssetServer: &assetserver.Options{
			Assets: assets,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				serveMedia(app, w, r)
			}),
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true, // нативный drag&drop папок (даёт абсолютные пути)
		},
		BackgroundColour: &options.RGBA{R: 10, G: 9, B: 18, A: 1},
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	}
	if cfg.WindowMaximised {
		opts.WindowStartState = options.Maximised
	}
	if startFlags.minimized {
		opts.WindowStartState = options.Minimised
	}
	// Один экземпляр: повторный запуск exe возвращает окно (в т.ч. спрятанное кнопкой
	// паники). После самообновления замок не берём — старый процесс может ещё завершаться.
	if !startFlags.afterUpdate {
		opts.SingleInstanceLock = &options.SingleInstanceLock{
			UniqueId:               "plauncher-8e5c1f0a-2d4b-4b7e-9c3a-single",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.showFromSecondInstance() },
		}
	}

	if err := wails.Run(opts); err != nil {
		log.Fatal(err)
	}
}

// serveMedia отдаёт обложки/скриншоты по URL /media/<путь> ТОЛЬКО из текущей
// папки данных. Встроенные ассеты фронтенда Wails обслуживает до этого хендлера.
// Ограничение каталогом + проверка на ".." закрывают path traversal.
func serveMedia(app *App, w http.ResponseWriter, r *http.Request) {
	const prefix = "/media/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}

	dataDir := app.GetDataDir()
	if dataDir == "" || app.isLocked() { // заблокировано PIN-кодом — медиа не отдаём
		http.NotFound(w, r)
		return
	}

	rel := filepath.Clean(strings.TrimPrefix(r.URL.Path, prefix))
	full := filepath.Join(dataDir, rel)

	// Защита от выхода за пределы папки данных через ".."
	if full != dataDir && !strings.HasPrefix(full, dataDir+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}

	if info, err := os.Stat(full); err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, full)
}
