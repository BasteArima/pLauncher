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
  Title/version/languages/tags from the release title; cover + up to 9 screenshots; a "?" popover in the
  metadata box lists supported sites (`SupportedSources`).
- **Library UI (Steam-like)**: aurora/glass dark theme; resizable + collapsible-tree sidebar with game
  icons; home with hero banner + configurable, drag-reorderable shelves ("edit sections" mode); Steam-style
  detail page (hero header, description, screenshots, lightbox); grid + sort.
- **Collections**: manual + dynamic-by-tags; CRUD; sidebar groups & a `collection` shelf type come from
  them; drag a game onto a collection to add it (works for dynamic too via pinned `game_ids`).
- **Tags & favorites**: auto-tags from parsing + manual editing; favorites (♥) with shelf/filter.
- **Discreet mode** (👁 / Ctrl+H) blurs all covers; "only games with a cover" toggle (✨) hides un-parsed.
- **Cover fit/position editor** (fill/contain/stretch + X/Y), per game.
- **Playtime tracking** via `cmd.Wait` (no polling); shown on detail; sort by playtime.
- **Custom UI chrome**: frameless window + custom titlebar (drag, min/max/close, double-click maximize),
  custom context menus, custom confirm dialog (no native popups), toasts.
- **Persistence**: configurable data folder (first-run wizard, settings, move-on-change); window size;
  sidebar width; sort/layout/collapsed/lang/onlyDressed in localStorage.
- **i18n**: en/ru/es built-in + user languages from `data/languages/*.json` (README + `_example.json`
  auto-dropped there); language picker on first run and in settings; backend errors are in English.

What's NOT done yet → see "Roadmap" below.

## Stack & layout

- **Wails v2.12** desktop app: **Go** backend + **Svelte 3** + **Tailwind** frontend.
- SQLite via `modernc.org/sqlite` — **pure Go, no CGO** (matters for cross-compiling).
- Everything lives under `game-launcher/`:

```
game-launcher/
  main.go                  Wails setup: frameless window, window-size restore, /media asset handler
  app.go                   App struct + ALL methods bound to JS (the Go<->JS API surface)
  appconfig.go             config.json pointer (data dir + window size) in os.UserConfigDir()/pLauncher
  internal/
    db/db.go               SQLite repo (games + key/value settings table). gameColumns + scanGames helper.
    models/                Game, Collection structs (json tags = JS field names)
    scanner/               ScanFolder (top-level subdirs), FindBestExecutable (scored .exe pick), NewID, NormalizePath
    parser/                one file per site + parser.go (factory, helpers) + extract.go (release-title parsing)
    launcher/launcher.go   launch exe (+ playtime via cmd.Wait goroutine), open folder, find exes
  frontend/src/
    App.svelte             ~1900 lines — the entire UI (sidebar, home, detail, modals, context menu)
    GameCard.svelte, Shelf.svelte
    i18n.js + locales/{en,ru,es}.json
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
- `libGames` is the base list everywhere (home/shelves/collections/grids); it's `games` filtered by the
  “only with cover” toggle (`onlyDressed`). Use `libGames`, not `games`, in derived views.
- UI prefs in `localStorage`: `plauncher_sidebar_w`, `plauncher_shelves`, `plauncher_sort`,
  `plauncher_collapsed`, `plauncher_only_dressed`, `plauncher_lang`.
- Window is **frameless**; custom titlebar uses CSS `--wails-draggable`. Native context menu is disabled;
  there's a custom one (`gameMenuItems` / `collectionMenuItems` / `shelfMenuItems` + `openCtx`).
- Drag&drop: external folder drop uses Wails `OnFileDrop` (DOM `file.path` is empty in WebView2). Internal
  game→collection drag is HTML5 DnD; the file-drop overlay is gated on `dataTransfer` containing `Files`.
- Comments in the code are largely in **Russian** — that's fine, keep them; only user-facing strings are localized.

## Build & run

```
cd game-launcher
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
1. **Update checking** — store each game's source URL; on demand or on a schedule, refetch the page,
   parse the version, and flag "update available" with a badge. These (F95/tracker) games update often —
   this is the core "don't forget what changed" feature. Reuse the existing parsers (they already return
   a version).
2. **Play status / backlog** — per-game status (playing / finished / dropped / want-to-play); filters and
   shelves by status. Fits the existing dynamic-collection machinery.
3. **Rating & notes** — personal star rating + a notes field (spoilers under a toggle).

Privacy (matters for this content):
4. **PIN/password on launch** + a global "panic" hotkey that minimizes & blurs even when unfocused;
   optional neutral process name/icon. (Builds on the existing discreet mode / Ctrl+H.)
5. **Blur covers by default until hover** (option) — extends discreet mode.

Polish / convenience:
6. **Auto-detect engine** (Ren'Py / Unity / RPG Maker) from folder contents → better `.exe` pick + an
   engine tag without needing to parse a page.
7. **Multiple executables per game** (game / config / walkthrough) with a quick-launch menu.
8. **Relink moved games** — folder moved → offer to attach to an existing record keeping metadata
   (IDs are random now, dedupe is by path; add a folder-name fingerprint).
9. **Backup/export library** (JSON) + DB backup; a "library health" dashboard (no cover / no description /
   no exe as actionable lists).
10. **Multi-tag filter** (AND/OR, exclude) and a tag blacklist (hide unwanted genres).
11. **Localize parser errors via codes** — currently backend errors are plain English strings; switch to
    codes so the frontend dictionary can translate them.
12. **Self-update for the launcher** + per-OS CI builds (GitHub Actions) for distribution.

Owner's note: #1 (update checking) and #2 (statuses) are expected to give the biggest payoff.

## More detail

Durable project notes live in the agent memory dir
`~/.claude/projects/<this project>/memory/` (parser-design, data-dir-architecture,
ui-state-and-collections-plan). The code comments and `extract_test.go` are also good references.
