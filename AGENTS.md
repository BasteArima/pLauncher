# pLauncher — agent onboarding

Read this first if you're an AI agent picking up this project with no prior context.
It captures the architecture, conventions and non-obvious gotchas so you can be productive immediately.

## What this is

A desktop **game-library launcher** (think a private, local Steam) for someone who downloads
**many adult games from various torrent/visual-novel sites** and wants to *not forget what they
downloaded* and browse their collection nicely. Core jobs:

1. **Scan** folders of games (each game = a subfolder) and auto-add them, auto-detecting the `.exe`.
2. **Parse metadata** (title, version, description, cover, screenshots, tags, languages) from a
   pasted game-page URL of a supported site.
3. **Browse** the library Steam-style: hero banner, horizontal shelves, collections, search,
   sort, tags, favorites, a discreet/“boss-key” mode to hide covers.

The content is adult, so **discretion features matter** and covers/screenshots are private local files.

## Current state (already implemented)

The app is fully working. Done so far:
- **Scanning**: multi-folder scan (`scan_paths`), each top-level subfolder becomes a game; smart `.exe`
  pick (`FindBestExecutable`); native folder drag&drop; "+ single game".
- **Parsers**: f95zone, pornolab (win1251), erotorrent, island-of-pleasure, Steam (official API).
  Title/version/languages/tags **+ author + engine** from the release title / body fields; cover + up to
  15 screenshots; a "?" popover in the metadata box lists supported sites (`SupportedSources`).
- **Author & Engine fields** (`Game.Author`/`Engine`): parsed per-site (Steam developers; F95/pornolab/island
  body + title; erotorrent dev field). **Engine auto-detection by folder contents** (`scanner.DetectEngine`,
  13 engines) runs on scan/drop and backfills empty engine for existing games on re-scan.
- **Library UI (Steam-like)**: aurora/glass dark theme; resizable + collapsible-tree sidebar with game
  icons; home with hero banner + configurable, drag-reorderable shelves ("edit sections" mode); Steam-style
  detail page (hero header, description, screenshots, lightbox); grid + sort.
- **Collections**: manual + dynamic-by-tags; CRUD; sidebar groups & a `collection` shelf type come from
  them; drag a game onto a collection to add it (works for dynamic too via pinned `game_ids`).
- **Tags & favorites**: auto-tags from parsing + manual editing; favorites (♥) with shelf/filter.
- **Discreet mode** (👁 in the titlebar / Ctrl+H) blurs all covers. Games without a cover are listed under the
  sidebar filter «No cover» (`activeFilter === 'nocover'`, shown only when there are any) — a to-do list for parsing.
- **Cover fit/position editor** (fill/contain/stretch + X/Y), per game.
- **Playtime tracking** via `cmd.Wait` (no polling); shown on detail; sort by playtime.
- **Custom UI chrome**: frameless window + custom titlebar (drag, min/max/close, double-click maximize; a «Library ▾»
  menu with scan/add game/check updates/privacy/hotkeys/settings built by `libraryMenuItems()` in App and shown via
  `ContextMenu` (items support `hint` = shortcut); quick 👁 / 🔒 buttons and a scan/update-check status),
  custom context menus, custom confirm dialog (no native popups), toasts.
- **Persistence**: configurable data folder (first-run wizard, settings, move-on-change); window size;
  sidebar width; sort/layout/collapsed/lang in localStorage.
- **i18n**: en/ru/es built-in + user languages from `data/languages/*.json` (README + `_example.json`
  auto-dropped there); language picker on first run and in settings; backend errors are in English.
- **UX details**: lightbox is a full-window `fixed` overlay (Esc/arrows, mouse back/forward); confirm
  dialog accepts Enter **and** Space; right-click on any text input gives a custom cut/copy/paste/select-all
  menu (native context menu stays disabled elsewhere). Esc closes the top layer (menu → modal → editor → game).
- **Relink moved games** (`relink.go`): `Game.FolderMissing` is computed on read (not stored). Missing games
  get a ⚠ badge, a banner on the detail page and a sidebar "Folder not found" entry. `FindMissingGames`
  looks for candidates in scan paths + parents of known game folders: same folder name first, else a unique
  match of the same relative exe (generic names like `Game.exe` are ignored). `RelinkGame` rebases the exe
  path and **merges** a duplicate record that a scan created for the new folder (playtime summed,
  collection membership moved, duplicate deleted).
- **Backup** (`backup.go`): export = zip of a `VACUUM INTO` DB snapshot + `covers/` + `languages/` + manifest;
  import extracts to `<dataDir>/.import-tmp`, validates/migrates the DB, rewrites media paths to `covers/…`,
  swaps files with rollback. Game folders are absolute → after moving to a new PC use relink.
- **Privacy** (`privacy.go`, `hotkey_windows.go`; UI in Settings → Privacy): settings in DB `settings["privacy"]`.
  PIN (PBKDF2-SHA256, 200k iters, 5 tries then 30 s cooldown) → app starts locked; while locked `GetGames/
  SearchGames/GetCollections/FindMissingGames` return nothing and `/media` 404s. Idle auto-lock (frontend timer →
  `Lock`). Panic button = global WinAPI `RegisterHotKey` on its own OS thread; hides/minimizes the window, optionally
  locks; same combo or a second exe launch (SingleInstanceLock) restores. Cover blur mode none/hover/always via
  `lib/privacy.js` stores (`coverBlur`, `blurCls(mode, kind)`); Ctrl+H = discreet (always). Hidden collections
  (`Collection.Hidden`): their games are filtered out of `visibleGames` until Ctrl+Shift+H (asks PIN if set).
- **Settings** are a tabbed modal: `components/settings/{General,Library,Privacy,Data}Tab.svelte`.
- **Launch-file detection** (`scanner/launchfinder.go`): if a folder has no `.exe`, a second pass looks for
  html (index/game/…), bat/cmd, jar, swf, qsp, rags, love (docs/installers skipped). On Linux/macOS native
  launchers (.sh, .x86_64, .AppImage, .app bundles) win over `.exe`. Re-scan fills an empty `ExecPath` of
  existing games; `DetectLaunchFiles(ids)` does it on demand. `launcher.isDirectExecutable` decides direct exec
  vs OS association (`cmd start` / `open` / `xdg-open`).
- **Folder size** (`sizes.go`): `size_bytes/size_checked_at` columns, computed in a background goroutine
  (startup, after scan/drop/relink; stale after 7 days) and written ONLY via `repo.SetGameSize` so UI saves
  never overwrite it. `RecalcGameSize(id)` / `RefreshSizes()`; sort option `size_desc`.
- **Selection & bulk actions**: `lib/view.js` (selection store, Ctrl/Shift-click, geometric arrow-key focus,
  card size store), `lib/bulk.js` (favorite, detect launch, check updates, re-parse, remove) +
  `SelectionBar.svelte`; right-click on a selected card shows bulk items. Backend: `RemoveGames`,
  `SetFavorites`, `DetectLaunchFiles`.
- **Ignored folders** (`ignore.go`, `scanner/ignore.go`): settings key `ignored_paths` (JSON list, compared via
  `NormalizePath`). `ScanFolder` skips them. «Ignore when scanning» = `IgnoreGames(ids)` (adds folders + removes
  games); list managed in Settings → Library (`GetIgnoredPaths/AddIgnoredPath/RemoveIgnoredPath`). An explicit
  drop/«single game» of an ignored folder adds it and un-ignores it.
- **SQLite** is opened with `_pragma=busy_timeout(5000)`: background size calculation writes concurrently with UI.
- **Hotkeys**: handled in `App.svelte#handleKeydown` (+ Enter/Space on focused `GameCard`); the reference list
  lives in `lib/hotkeys.js` (F1 window + Settings → General). F5/Ctrl+R are intercepted (a WebView reload would
  drop state). Card size: slider in `ViewControls`, Ctrl+wheel, Ctrl +/−/0.
- **Launcher self-update** (`updater.go`): GitHub Releases of `updateRepo` (ldflag); downloads `pLauncher.exe`,
  verifies `pLauncher.exe.sha256`, renames running exe to `.old`, restarts with `--after-update`.
  Requires the releases repo to be **public** (the code repo currently is private). Silent check ≤1/day.

What's NOT done yet → see "Roadmap" below.

## Stack & layout

- **Wails v2.12** desktop app: **Go** backend + **Svelte 3** + **Tailwind** frontend.
- SQLite via `modernc.org/sqlite` — **pure Go, no CGO** (matters for cross-compiling).
- Everything lives under `game-launcher/`:

```
game-launcher/
  main.go                  Wails setup: frameless window, window-size restore, /media asset handler, appVersion
  app.go                   App struct + most methods bound to JS (the Go<->JS API surface)
  relink.go / backup.go / updater.go   relink moved games / export-import / launcher self-update (also bound)
  appconfig.go             config.json pointer (data dir + window size) in os.UserConfigDir()/pLauncher
  build.ps1                one-command local build (tests → vite → wails build -s), see Build & run
  internal/
    db/db.go               SQLite repo (games + key/value settings table). gameColumns + scanGames helper.
    models/                Game, Collection structs (json tags = JS field names)
    scanner/               ScanFolder (top-level subdirs), FindBestExecutable (scored .exe pick), NewID, NormalizePath
    parser/                one file per site + parser.go (factory, helpers) + extract.go (release-title parsing)
    launcher/launcher.go   launch exe (+ playtime via cmd.Wait goroutine), open folder, find exes
  frontend/src/
    App.svelte             root: library state, derived lists, navigation, wiring (~700 lines)
    components/            Sidebar, GameDetail, GameEditor, MetadataPanel, Hero, Shelf, GameGrid, GameCard,
                           ShelfEditor, SortSelect, Lightbox, TitleBar, ContextMenu, Toast, ConfirmDialog,
                           SetupWizard, SettingsModal, CollectionModal, RelinkModal
    lib/ui.js              showToast() + askConfirm() stores (usable from any component)
    lib/util.js            mediaSrc, coverStyle, sortGames, fmtPlaytime, localStorage helpers, clickOutside
    lib/shelves.js         shelves (load/build) + buildCol (collection membership)
    lib/inputMenu.js       cut/copy/paste menu for text inputs
    lib/view.js / bulk.js / hotkeys.js / privacy.js   selection+card size / bulk actions / hotkey list / blur stores
    i18n.js + locales/{en,ru,es}.json
  ../.github/workflows/    ci.yml (vet/test on Windows+Ubuntu 24.04+macOS), release.yml (tag v* → win exe, linux
                           binary+tar.gz, macOS universal .app zip, each with .sha256; names = updater.assetNameFor)
  wails.json               app name "pLauncher", build metadata, icon source = build/appicon.png
```

`frontend/wailsjs/` is **generated** — never edit by hand. After changing exported `App` methods or
model structs run `wails generate module` (regenerates the JS/TS bindings).

## How data is stored

- The **data folder** (DB + `covers/` + `languages/`) is **user-chosen**, not fixed. Its location is
  saved in a tiny pointer file `config.json` in `os.UserConfigDir()/pLauncher/` (it can't live inside
  the data folder itself). First run shows a 2-step wizard (data folder → games folders).
- `app.repo` / `app.dataDir` are **nil/empty until configured** — exported methods must tolerate that
  (see guards in `GetGames`, `ScanLocalFolder`, `AddGamesFromDrop`, etc.).
- Cover/screenshot paths are stored **relative to the data dir** (`covers/<file>`). The frontend renders
  them via `mediaSrc()` → URL `/media/<rel>`, served by `serveMedia` in `main.go` strictly from the data dir.
  Legacy absolute / `data/...` paths are normalized in `app.mediaRel`.
- Scan folders are a **list** (settings key `scan_paths`): `GetScanPaths / AddScanPath / RemoveScanPath / ScanAllFolders`.
- Collections are JSON in the settings table: `GetCollections / SaveCollections`.
- Window size persists via `App.SaveWindowSize` called on JS `resize` (do **not** call WindowGetSize in
  `shutdown` — Wails 2.12 panics with divide-by-zero there).

## Parsers (the fiddly part)

- Factory `parser.GetParser(url)` dispatches by domain. Sites: f95zone, pornolab, erotorrent,
  island-of-pleasure, Steam. Add new ones in the factory **and** in `parser.SupportedSources()`.
- **Title/version/languages/tags come from the “release title”** (`h1.maintitle`/`#topic-title` on
  pornolab, the «Название» field on island) parsed by helpers in `extract.go`
  (`cleanReleaseTitle`, `versionFromTitle`, `parseReleaseLanguages`, `parseReleaseTags`).
  **Do NOT** go back to font-size heuristics for the title — that was the old, broken approach.
- pornolab pages are **windows-1251** (decoded via `charmap`). Image real URLs are in the `title` attr
  of `var.postImg` (the `src` is localized when the page is saved).
- **Steam** uses the official `store.steampowered.com/api/appdetails` JSON API (not scraping); cover =
  `library_600x900.jpg`.
- Real saved sample pages for testing selectors are in the user's `C:/Users/user/Downloads/` (pornolab1-6,
  island-of-pleasure, erotorrent). These sites are behind Cloudflare/login — **can't be fetched from a
  sandbox**; iterate against the saved HTML. Pure string logic is covered by `extract_test.go`.
- Backend error strings are **English**; the “description not found” case leaves description empty so the
  UI shows a localized placeholder.

## Frontend conventions & gotchas

- **Localization**: use `{$t('key')}` in markup, `tr('key', {param})` in script. Add every new
  user-facing string to all three `locales/*.json`. Built-in en/ru/es are bundled into the exe; users can
  drop extra `<code>.json` into `<dataDir>/languages/` (loaded via `GetCustomLocales`).
- **Svelte 3 reactivity trap (important):** a `$:` derived value or a function called in markup only
  recomputes when the **identifiers it textually references** change. Functions like `tr()`, `buildShelf()`,
  `sortGames()` read state that isn't named in the expression, so pass that state in as an argument (see
  `shelfData._lang`, `filterBtn(filter, active)`, `viewTitle = ($langStore, …)`). Forgetting this = stale UI.
- `visibleGames` is the base list everywhere (home/shelves/collections/grids); it's `games` without hidden
  collections (see Privacy). Use `visibleGames`, not `games`, in derived views.
- UI prefs in `localStorage`: `plauncher_sidebar_w`, `plauncher_shelves`, `plauncher_sort`,
  `plauncher_collapsed`, `plauncher_lang`.
- Window is **frameless**; custom titlebar uses CSS `--wails-draggable`. Native context menu is disabled;
  there's a custom one (`openCtx(e, items)` in App; `gameMenuItems`/`shelfMenuItems` in App, collection menu in Sidebar).
- Global keys: components that own keys (ConfirmDialog, Lightbox) call `preventDefault()`; App's window
  handlers skip events with `e.defaultPrevented` — keep this contract when adding key handlers.
- Drag&drop: external folder drop uses Wails `OnFileDrop` (DOM `file.path` is empty in WebView2). Internal
  game→collection drag is HTML5 DnD; the file-drop overlay is gated on `dataTransfer` containing `Files`.
- Comments in the code are largely in **Russian** — that's fine, keep them; only user-facing strings are localized.

## Build & run

```
cd game-launcher
.\build.ps1                       # Windows, recommended: go test → vite build → wails build -s
                                  # (on the owner's machine `wails build`/`wails dev` can't find npm in
                                  #  child processes, so the frontend is built via node directly)
wails dev                         # run with hot reload (note: OnShutdown is unreliable in dev)
wails build                       # build for current OS -> build/bin/pLauncher.exe
wails build -platform windows/amd64 [-nsis]
go build ./... && go vet ./... && go test ./...   # backend checks
cd frontend && npm run build      # frontend only
wails generate module             # after changing exported Go methods / structs
```
Cross-OS builds must run **on the target OS** (or CI) — Wails webview bindings are OS-native, even though
the Go/SQLite parts are CGO-free.

## Roadmap / suggested improvements (not yet implemented)

Ideas the owner wants to pursue. Roughly ordered by value. Discuss/confirm scope before big ones.

High value (turns the app from a gallery into a real collection tracker):
1. ✅ **DONE — Update checking + per-platform sources block.** `Game.Sources` (`{platform,url,last_version}`)
   + `PrimarySource` + `UpdateAvailable/UpdateVersion/UpdateSource/LastCheckedAt`. Sources are stored on
   parse (`UpdateGameMetadata`→`upsertSource`) and editable per-platform in the editor (★ = primary).
   `Parse(ctx,url,"")` is "metadata-only" (no image download). `CheckGameUpdates`/`CheckSourceUpdate`/
   `CheckAllUpdates` compare **per-source** (string diff after `normVersion`, no semver/cross-site). UI:
   clickable source badges (open / load-into-parser / re-check), amber card ring + ⬆ on home cards, amber
   "update: {v}" badge on detail with **Accept** (re-parse), top-bar "Check all", and an "Updates"
   sidebar filter. NOT done: background/scheduled auto-checks. Steam has no version → can't be tracked.
2. **Play status / backlog** — per-game status (playing / finished / dropped / want-to-play); filters and
   shelves by status. Fits the existing dynamic-collection machinery.
3. **Rating & notes** — personal star rating + a notes field (spoilers under a toggle).

Privacy (matters for this content):
4. ✅ **DONE — PIN lock, idle auto-lock, global panic hotkey, hidden collections.** NOT done: neutral
   process name/icon.
5. ✅ **DONE — Blur covers until hover / always** (Settings → Privacy).

Polish / convenience:
6. ✅ **DONE — Auto-detect engine** from folder contents (`scanner.DetectEngine`, 13 engines:
   Ren'Py/Unity/Unreal/Godot/RPG Maker/Game Maker/Construct/Wolf RPG/KiriKiri/TyranoBuilder/Flash/QSP/HTML).
   Runs on scan/drop, fills `Game.Engine` only when empty, backfills existing games on re-scan.
7. **Multiple executables per game** (game / config / walkthrough) with a quick-launch menu.
8. ✅ **DONE — Relink moved games** (see "Current state").
9. ✅ **DONE — Backup/export library** (zip). NOT done: a "library health" dashboard (no cover / no
   description / no exe as actionable lists).
10. **Multi-tag filter** (AND/OR, exclude) and a tag blacklist (hide unwanted genres).
11. **Localize parser errors via codes** — currently backend errors are plain English strings; switch to
    codes so the frontend dictionary can translate them.
12. ✅ **DONE — Self-update + CI releases for Windows/Linux/macOS.** Self-update installs on Windows & Linux;
    macOS (unsigned .app) only links to the release page. Linux builds need `-tags webkit2_41` (WebKitGTK 4.1).
    Releases need a public repo. CI for Linux/macOS is written but not yet run.

Owner's note: #1 (update checking) and #2 (statuses) are expected to give the biggest payoff.

## More detail

Durable project notes live in the agent memory dir
`~/.claude/projects/<this project>/memory/` (parser-design, data-dir-architecture,
ui-state-and-collections-plan). The code comments and `extract_test.go` are also good references.
