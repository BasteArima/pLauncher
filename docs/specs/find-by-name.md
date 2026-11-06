# Spec: find game info by name

Status: **planned** (next feature after the 2026-11 redesign). The owner approved the flow below; the mockup is
the canvas board «A3 · Game with folder only» (a private claude.ai artifact, so this text is the full spec).

## Why

After a scan most games are bare: a folder name, an exe, no cover/description/tags. Today the only way to fill them
is to find the game's page in a browser and paste the link. This feature searches the public sources by the
folder name and lets the user pick the right result, check it, then save.

## User flow

Where: the game page (`GameDetail`), block «Find game info» (`MetadataPanel` with `prominent`, shown for games with
no cover, no description and no sources). The compact `MetadataPanel` on other games gets the same two tabs, with
«Paste a link» selected by default.

### Step 1: search

- Two tabs: **Search by name** (default in the prominent block) | **Paste a link** (today's input, unchanged).
- Name field prefilled from the folder name: `MoonlitHarbor_v0.3` → «Moonlit Harbor». Under it, only when something
  was removed: «From the folder name, version 0.3 removed». The user can edit the name; Enter = search.
- Source toggles (chips): **Steam**, **F95zone**, **VNDB**; all on by default; remembered in localStorage
  `plauncher_find_sources`.
- A muted note: «Pornolab, Erotorrent, Island of Pleasure: paste a link instead» (they are behind Cloudflare/login).
- **Continue without info**: hides the big prominent block for this game (localStorage `plauncher_noinfo_dismissed`,
  a set of game ids); the compact panel stays so the search is still reachable.

### Step 2: results

- One call returns everything when all sources finish (or time out); show a spinner meanwhile.
- 3–8 rows sorted by score: cover thumbnail, title, `Site · author · version · year` (skip empty parts),
  **Use this** button. The top row gets a «Most likely» badge only when it is clearly ahead (see Scoring).
- Sources that failed or timed out: one quiet line under the list, e.g. «F95zone didn't respond». Others still show.
- Nothing found: «Nothing found. Try a shorter name, or paste a link.»

### Step 3: preview, then save

- **Use this** fetches the page data **without writing anything** and shows the game page filled with it (cover,
  screenshots, description, tags, author, engine, version, languages), read-only, with a sticky bar like edit mode:
  «Found on F95zone — check before saving» + **← Other results** + **Save to library**. Esc = back to results.
- Preview images are remote URLs shown directly (`<img src="https://…">`; there is no CSP). Cover blur / discreet
  mode must apply to them like to local covers (`blurCls`).
- **Save to library** applies the data exactly like today's «paste a link» (`UpdateGameMetadata` rules: non-empty
  fields overwrite, tags are merged, the link becomes a source and the primary one if none, update flags reset),
  downloads cover + screenshots into the data folder, then reloads the game and toasts «Info saved».
- If the site has no version (Steam, VNDB) and the game has none, keep the version taken from the folder name.

## Backend

All new user-visible errors go through `apperr` with `err.<code>` in all 11 locales.

### Name from the folder

`scanner.SuggestSearchName(folderName string) (title, version string)` (new; do **not** change `cleanFolderTitle`,
scan titles stay as they are):
- cut bracket tails `[...]`, `(...)`, `{...}`;
- detect and remove a version: `v0.3`, `_v0.3.1a`, `-0.10.2`, `Ep 6` / `Episode 6`, `Ch.3` / `Chapter 3`,
  `Final`, `Season 2`; return it as `version` (as written, e.g. `0.3`, `Ep 6`);
- drop platform/packaging words: `pc`, `win`, `windows`, `linux`, `mac`, `x64`, `rus`, `eng`, `repack`;
- `_`, `.`, `-` → spaces; split CamelCase (`MoonlitHarbor` → `Moonlit Harbor`, keep acronyms like `VR`, `RPG`);
- table-driven tests with at least 25 real-looking folder names.

### Sources (`internal/parser/search.go` + one file per source)

```go
type SearchHit struct {
    Source  string  `json:"source"`  // same names as SourceName(): "F95zone", "Steam", "VNDB"
    URL     string  `json:"url"`     // page link, later passed to the normal parser
    Title   string  `json:"title"`
    Author  string  `json:"author"`
    Version string  `json:"version"`
    Year    string  `json:"year"`
    Thumb   string  `json:"thumb"`   // remote image URL, may be empty
    Score   float64 `json:"score"`
    Best    bool    `json:"best"`
}
type SearchFailure struct { Source string `json:"source"`; Error string `json:"error"` }
type SearchResult struct { Hits []SearchHit `json:"hits"`; Failed []SearchFailure `json:"failed"` }

func Search(ctx context.Context, client *http.Client, query string, sources []string, hint SearchHint) SearchResult
type SearchHint struct { Version, Engine string }
```

Sources run in parallel, each with its own ~10 s timeout; at most 8 hits per source before scoring.

- **F95zone**: reuse `f95LatestSearch` (the public `latest_data.php` JSON; it matches a **single word** only). Query
  the longest words from `f95SearchTerms(query)` (stop after the first word that returns rows), then score rows
  locally. Extend `f95LatestRow` with the fields needed for a hit (title, creator, version, date/ts, thread_id,
  cover). **Check the real field names against a live response** before relying on them. Hit URL =
  `https://f95zone.to/threads/<thread_id>/`. Cover → `f95FullImage`.
- **Steam**: `GET https://store.steampowered.com/api/storesearch/?term=<q>&l=english&cc=US` → `items[]`
  (`id`, `name`, `tiny_image`). Hit URL = `https://store.steampowered.com/app/<id>/`. **Verify** whether adult-only
  titles show up for an anonymous request; if not, write it down in AGENTS.md (no workaround, no login).
- **VNDB** (new source): API `POST https://api.vndb.org/kana/vn`, JSON body
  `{"filters":["search","=",q],"fields":"id,title,released,developers.name,image.thumbnail","sort":"searchrank","results":8}`.
  No key needed. Hit URL = `https://vndb.org/v<n>`. Year = first 4 chars of `released`.
  Also add a **VNDB parser** for `vndb.org/v<n>` links (factory `GetParser`, `SupportedSources`, `SourceName`):
  same API with `["id","=","v<n>"]` and fields `title,description,released,languages,developers.name,image.url,
  screenshots.url,tags.name,tags.rating,tags.spoiler,tags.category`; tags with `spoiler == 0`, sorted by rating,
  top ~15; strip VNDB BBCode (`[url=…]text[/url]`, `[spoiler]…[/spoiler]` → drop) from the description;
  languages → the existing language names; engine stays empty (VNDB has none); version empty.
- Use the existing User-Agent/client helpers (`newHTTPClient`), `requestErr`/`httpStatusErr`.

### Scoring

`normTitle`: lower case, remove diacritics and punctuation, drop leading `the`/`a`, collapse spaces.
`score = 0.75 * tokenJaccard + 0.15 * (exact normalized match) + 0.10 * (version match with hint)`, plus a small
bonus when one title starts with the other. `Best = top.Score >= 0.8 && (len==1 || top.Score - second.Score >= 0.15)`.
Unit-test the scoring on fixed inputs.

### App methods (bound to JS; run `wails generate module` via `build.ps1`)

- `SuggestSearchName(gameID string) (NameSuggestion, error)` → `{title, version}` from the game's folder name.
- `SearchGameInfo(query string, sources []string, gameID string) (SearchResult, error)`; the hint comes from the
  game (folder version, detected engine).
- `PreviewGameInfo(url string) (*models.Game, error)`: `parser.GetParser(url).Parse(ctx, url, "")` (metadata-only,
  nothing saved). Metadata-only mode currently drops image URLs in `downloadInto`; make it put the **remote** URLs
  into the returned game instead (`CoverPath` = cover URL, `Images` = screenshot URLs, capped at `maxScreenshots`),
  and make sure update checks (`CheckSourceUpdate`) never save those fields.
- `ApplyGameInfo(gameID, url string) error`: the same as `UpdateGameMetadata` (reuse it; a second fetch is fine) +
  the folder-version fallback above.
- All of them tolerate an unconfigured app (`a.repo == nil`) and the locked state (return nothing while locked,
  like `GetGames`).

## Frontend

- `MetadataPanel.svelte` → tabs + the three steps; keep the old link input as the second tab. Split into
  `FindByName.svelte` if it gets long.
- Preview: `GameDetail` gets an optional `preview` prop (Game-shaped) and renders it read-only with the sticky bar;
  App holds the preview state; Esc / mouse back return to results (keep the `e.defaultPrevented` key contract).
- Line icons from `lib/icons.js`, no emoji. New strings in **all 11** `locales/*.json` (`locales_test.go`).
- Svelte 3 reactivity: pass every piece of state a markup function reads as an argument (see AGENTS.md).
- Add mocks for the new methods to `docs/tools/screenshots/build-ui.cjs` (a few fake hits per source, a fake
  preview) so the flow can be clicked through in headless Edge without the network.

## Out of scope (later)

- Bulk «Find all by name» in the «No cover» smart list (auto-apply only confident single matches).
- Search on Pornolab / Erotorrent / Island (Cloudflare/login).
- Using `SuggestSearchName` for titles at scan time.

## Done when

- `go test ./...`, `go vet ./...` and `.\build.ps1` pass; new tests: folder names, scoring, each source's JSON
  decoding against recorded fixtures under `internal/parser/testdata/` (served by `httptest`, URLs swappable
  like `f95LatestURL`), the VNDB parser and BBCode stripping.
- The live endpoints were tried once by hand (or the owner was asked to), and any surprise is written down here.
- The flow works in the mock UI: search → results (with one failed source) → preview → save → game filled.
- AGENTS.md, CHANGELOG `[Unreleased]` and this spec's status are updated.
