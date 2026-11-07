# Spec: multiple launch files

Status: **steps 1 and 2 implemented; Windows automated acceptance passed** (2026-11-07).
Native dialogs/associations and real-library acceptance remain manual.
Design reference: [A2 · Game page](../design/game-page.png), interpreted using
[the design README](../design/README.md): the small dropdown beside Play and the Launch files list in Details.

## Why

A game folder can contain the game itself, a configuration utility and a walkthrough. Today pLauncher
remembers only one launch path. Keep Play predictable while making the other files available from the same
game page, without counting time spent in a guide or settings utility as playtime.

## Agreed scope

- One primary file, marked with a star, remains in `Game.ExecPath` / `exec_path`. Existing libraries and
  single-file games continue to work without conversion by the user.
- Up to eight additional files, each with a path and an optional editable label. Paths use
  the same representation as `exec_path`; do not move them into the data folder or introduce a new URI scheme.
- Play opens the primary file. An always-visible dropdown beside Play combines launch files and the existing game actions.
  Its file section appears only when more than one file is saved; the primary is starred.
- Only the primary file is eligible for playtime/session recording. Additional executables and documents
  open without recording, using the existing direct-execution versus OS-association rules.
- The editor manages the list: make primary, edit label, remove, Add… and Find in folder with checkboxes.
- Details lists the files; clicking a row opens that file. Missing files are muted and an attempted launch
  produces a clear localized `apperr` error.
- Relink transfers all relevant paths. Library backup/restore preserves the list and labels.
- Default labels come from filenames: config/settings → Settings, walkthrough/guide → Walkthrough.
  Users can override them. These are localized labels, not changes to filenames.

## User flow

### Game page: Play and dropdown

- Keep the filled Play button in the right-hand play card. Controls are **Play / chevron / favourite**, as
  in the mockup, with no separate ellipsis button. The chevron is always visible and never launches a file
  by itself; Play continues to open the primary directly.
- One menu combines both uses: when more than one file is saved, the top section lists the primary first
  (starred), then extras in saved order, followed by a separator. Below are all the current ellipsis-menu
  actions (Edit, Ignore, Remove, etc.). With zero/one file, omit the file section and its separator.
- File rows show their effective label, falling back to the filename; keep filename/path secondary text or
  a tooltip to distinguish identical labels. The star has an accessible Primary label.
- Choosing the primary uses exactly the same launch path and tracking policy as Play. Choosing any additional
  file opens it without recording, including a second game executable. There is no per-file tracking toggle.
- Card Play, sidebar Play, the game menu's Play and Ctrl+Enter continue to target the primary. Additional
  files do not become implicit fallbacks when the primary is missing.
- Use the existing custom-menu conventions: anchored placement that stays inside the window, focusable
  entries, keyboard activation and Esc/outside-click dismissal. Closing the menu must not also leave the
  game page. Respect the `e.defaultPrevented` and top-overlay precedence contracts.
- Remove the old game-page ellipsis button. Card right-click menus and other existing menus stay unchanged.
- Metadata preview remains read-only: no new launch controls there. Changing games or locking/panic-hiding
  closes transient menus and discards pending discovery responses.

### Details: Launch files

- Replace the singular Launch file row with Launch files. Keep it in the compact Details card in the right
  column, in the position shown by the mockup; do not create a separate large card.
- Render one text link per file, primary first with a star, followed by extras. Use the same effective labels
  and file identity as the dropdown; expose the full path on hover/focus. Long paths must not widen the card.
- Clicking opens the file, rather than opening the file picker as the current single-file link does.
  Editing the list belongs to edit mode. No files retains a localized action leading to that editor block.
- A missing entry stays in place and retains its label/path. Render it muted with a localized missing-file
  indication. Keep attempted activation capable of reporting the error; do not use an inert disabled control
  that cannot explain the problem. A missing extra does not disable Play or other files.

### Editor: Launch files

- Replace the current single launch-path field with a compact list within `GameEditor`. Each row has the
  filename/path, an optional label input, a star control to make it primary, and Remove. Show the primary
  clearly without relying solely on color. Use icons from `lib/icons.js` through `Icon.svelte`, not emoji.
- Making a row primary updates the draft's `exec_path`. Swap it with the previous primary,
  keeping the old primary in the promoted row's extra slot and preserving both labels. This does not launch either file.
- Remove removes only the library reference. It never deletes a file from disk. Removing the primary leaves
  `exec_path` empty; Play prompts for a primary, with no automatic promotion. The first file added to a game
  without a primary becomes primary.
- Add… opens a native file dialog starting in the game's folder, with supported launch formats and
  HTML/PDF/TXT documents included. Keep an All files option, as today. Cancellation is a no-op. Handling
  a missing starting folder follows the existing dialog fallback, without changing the game's folder.
- Find in folder opens an in-editor candidate picker: checkbox, effective label, filename and relative path.
  It has loading, empty, error/retry and selected-count states, plus Add selected and Cancel. Already-listed
  paths are marked and cannot be added twice. New candidates start unchecked.
- Neither scanning candidates nor changing checkbox selection persists anything. Add selected appends to
  the editor draft; Save/Ctrl+S commits the complete list with the rest of the game. Esc/cancel uses the
  existing dirty-state confirmation and restores paths, primary choice, labels and order together.
- Enforce the chosen capacity in the picker and Add… controls, with a localized explanation. The backend
  validates it too. Do not silently truncate the user's selection.
- Autofill labels as described below; permit editing immediately after adding. Exact editor visuals beyond
  the existing editable page are not supplied by `game-page.png`. For step 2 the owner explicitly chose
  the current GameEditor style; no new editor design or private-board parity is claimed.

## Backend

### Baseline behavior and integration points

- `App.Launch(gameID, exePath, folderPath)` guards unconfigured/locked state and missing folders, then calls
  `launchTracked`. Direct files use the detached session monitor; associated files use `LaunchAssociated`
  and still update `last_launched_at`. Merely passing an extra path to this API would incorrectly record it.
- `launcher.CanTrack` wraps `isDirectExecutable`. On Windows direct files are `.exe`/`.com`; other platforms
  use executable permissions with document/bundle exceptions. Reuse dispatch policy, not tracking side effects.
  Explicitly treat PDF/TXT as documents even if their Unix executable bit is set, like the existing HTML case.
- `FindExecutables` enumerates `.exe` candidates with basic utility exclusions. `scanner.FindBestExecutable`
  and `launchfinder.go` choose one main launcher; their exclusions deliberately remove guides, config files
  and documentation directories. They are not a complete supplementary-file enumerator.
- `rebaseExe` tries the same relative path, then may find another best executable. That fallback is appropriate
  to review for the primary but must not turn a missing walkthrough into a reference to the game executable.

### Storage and compatibility

Storage (the primary-path contract is unchanged):

```go
type LaunchFile struct {
    Path  string `json:"path"`
    Label string `json:"label"` // explicit user text; empty uses a localized filename-based default
}

// Additions to Game; ExecPath remains the sole authoritative primary path.
ExecLabel   string       `json:"exec_label"`
LaunchFiles []LaunchFile `json:"launch_files"` // additional files only, in display order
```

- Store `launch_files` as a JSON array in a new `games` TEXT column and `exec_label` as TEXT. Add columns via `ALTER TABLE`; read with `COALESCE`. Legacy empty/NULL
  values mean no extras and no custom primary label. Do not duplicate the primary path in the array.
- Use one normalized path identity for deduplication, membership checks and promotion. Preserve the selected
  path string for storage/launch. Respect platform path semantics and existing `scanner.NormalizePath`
  conventions; no basename-only matching and no file-content deduplication.
- `UpdateGame` validates and saves primary, labels and extras together. Allow missing paths to remain saved:
  a temporarily disconnected drive must not destroy the list. Reject blank extra paths, duplicates and
  over-capacity additions with localized errors. Existing merge overflow remains editable/removable;
  while over capacity, new paths cannot be added. Missing flags are computed, never persisted in this JSON.
- Scan/drop, `DetectLaunchFiles` and metadata updates preserve extras and explicit labels. Main-file detection
  continues to fill only an empty `exec_path`, never promotes a guide/config candidate or overwrites a chosen
  main file. If a discovered primary already exists among extras, consolidate it without duplicating the path.
- Existing playtime/session ownership and `notes_updated_at` rules remain intact when saving the new fields.
  Regenerate Wails bindings after changing the model/App methods; never edit generated bindings manually.

### Label inference

- Match the basename without extension, case-insensitively, using word boundaries/separators and common
  variants such as `Game_Config`, `settings-x64` and `walkthrough_en`. Avoid substring accidents such as
  labelling `GuidebookishGame.exe` as a walkthrough. Define tested deterministic precedence for mixed names.
- Representation: store only explicit user text in `label`. When it is empty, infer a stable category
  (`settings`, `walkthrough`, or none) and translate it in the UI; no category falls back to the filename.
  This lets automatic labels react to language switches while preserving custom labels verbatim.
- `setup` is a discovery hint, not an unconditional Settings label: a setup program may be an installer.
  Do not execute candidates to identify their purpose or read PDF/TXT contents for classification.

### Candidate discovery

- Add a separate read-only candidate enumeration operation, reusing executable/native-launcher knowledge
  from the current finders. Do not relax `FindBestExecutable`'s main-game ranking/exclusions globally.
- Include other `.exe` files and the existing supported native/script/game launch formats. Additionally
  include HTML/HTM/PDF/TXT files with names suggesting config/settings/setup/walkthrough/guide; include
  normal HTML game entry points. Return every eligible candidate within the chosen bounds, not just the
  highest-scoring launcher. Only hint-matched documents are suggested; ordinary README/manual files are excluded.
- Inspect only the requested game's folder, in the root plus two subdirectory levels: at most
  10,000 entries, 200 candidates and five seconds, with an incomplete-result flag. Do not follow directory symlinks outside
  it or recurse inside `.app` bundles. Skip assets/caches/save directories, but allow shallow `docs`, `doc`
  and `manual` folders for guides. Existing game-library scans do not automatically add these entries.
- Keep uninstallers and crash reporters out of suggestions. A setup-named candidate is never auto-launched
  or automatically made primary. Manual Add… remains available for files omitted by the heuristic.
- Return path, filename/relative display path and inferred label category. Deduplicate normalized paths,
  order deterministically, and report missing/unreadable root errors rather than pretending the folder is empty.
  If a result/time bound is reached, report that the list is incomplete and retain Add… as a fallback.

### Launching and availability

Implemented App contracts (DTO details below):

- `FindLaunchFileCandidates(gameID)` reads the saved game folder and returns candidate rows and any
  incomplete-result indication. It neither edits the game nor starts a process.
- `GetLaunchFileAvailability(gameID)` checks the small saved list and returns missing/unavailable status by
  normalized path. Fetch when opening the page/menu; avoid filesystem walks/stat checks per library card.
- `LaunchGameFile(gameID, path)` resolves the selected path against the saved primary/extras. The backend,
  not a caller-supplied tracking boolean, decides whether this is a primary or supplementary launch. Reject
  stale/unlisted paths; always recheck existence at launch because availability may have changed.
- Keep the existing `Launch` signature for compatibility, but route it through the same saved-list validation
  and primary-only accounting policy. Frontend Play actions must use the saved primary. The game folder
  remains the working directory for direct and associated launches, as with today's `exec_path`.
- Direct additional executables need an untracked launch path: start them with the same platform execution
  behavior and release/wait for process resources without creating a monitor, pending session or completed
  session. Normal UI closure must not kill the utility. Associated extras use the OS opener without accounting.
- Primary direct launches keep existing durable monitoring, thresholds, recovery and once-only totals.
  Primary association-based launches remain untracked; this feature does not invent sessions for an opener.
  Classify the role at launch time: promoting/demoting a file later cannot retroactively change a running session.
- Extras do not update `last_launched_at`, Continue playing or running-game/update blockers.
  They must never change `time_played`, session counts, longest-session values or statistics activity.
- New operations tolerate an unconfigured app and locked state without exposing file paths or launching.
  Recheck lock state before returning discovery data; the frontend invalidates late replies on lock/navigation.
- Distinguish missing file, inaccessible path, unsupported target/failed OS open, unlisted path and list-limit
  errors through `apperr` and all 11 locales. Folder-missing errors continue to use the relink flow. Keep the
  saved path after failure, provide a useful filename/path in the unlocked error, and permit retry after repair.

### Relink and backup

- Rebase each in-folder entry by its relative path under the old root, preserving labels and saved order.
  Extras outside that root retain their existing absolute path, as external primary paths do today.
- For an absent rebased extra: retain the expected new path and mark it missing; never run the
  best-executable fallback for that entry. Retain the existing primary best-executable fallback.
  Deduplicate paths after rebasing; do not silently overwrite explicit labels.
- Duplicate-record merging must include these fields in `SaveRelink`'s transaction with sessions, totals and
  collection membership. Keep the survivor primary/custom labels; append unique duplicate extras and its distinct former primary.
  Preserve overflow beyond eight extras; additions remain blocked until back within capacity.
- `VACUUM INTO` snapshots include the new DB columns automatically. Import migrates old databases and
  preserves new lists/labels. Keep launch paths as game-file paths, not media paths rewritten to `covers/`.
  Backup contains references, not copies of game executables or guide documents. Moving to another machine
  still requires relink; new and old archives both need round-trip tests.

## Frontend and delivery

- Reuse the current dark surfaces, compact controls and Details typography. Keep the Play chevron distinct
  from media and game-list arrows; the chevron also contains the existing game actions. Use accessible localized control names.
- All UI strings, inferred labels, picker states, capacity messages and error codes go into all 11 locales.
  Explicit user labels are not translated. Svelte 3 expressions must name their language/list dependencies.
- Extend `build-ui.cjs` with primary-only and multi-file games, a labelled config exe, an HTML/PDF guide,
  missing primary/extra cases, duplicate candidate paths, capacity and delayed/error discovery responses.
  Mock all new bound methods; do not open real utilities or documents during browser checks.
- Suggested delivery: step 1 implements storage, launch dispatch/accounting isolation, discovery, relink,
  backup coverage and generated bindings; step 2 implements the editor, dropdown and Details flow with mocks.
  Both steps are delivered. The owner authorized step 2 after reviewing the backend delivery.
- On implementation deliveries, update this spec's status, AGENTS.md and CHANGELOG `[Unreleased]`; commit
  without attribution and do not push unless requested. README screenshot refresh stays out of this feature.

## Out of scope

- Per-file command-line arguments, environment variables, custom working directories, launch sequences,
  per-file playtime, tracking overrides, child-process detection or tracking associated documents.
- Automatically launching or adding discovered candidates, scanning entire drives, downloading walkthroughs,
  copying/moving/deleting game files, or changing the library's main-launcher selection policy.
- Extra launch controls on every library card, new smart lists, a generic file manager or a page redesign.

## Done when (implementation)

- Migration and persistence tests cover old/NULL columns, primary-only games, JSON round trips, custom labels,
  normalization/deduplication, capacity, primary promotion/removal and unrelated saves preserving the list.
- Discovery fixtures cover other executables, HTML entry points, config/settings/setup and guide documents,
  nested docs, exclusions, unsupported paths, unreadable/missing roots, symlinks and bounded results.
- Launch tests prove that Play and primary rows share existing recording behavior, while a directly launched
  extra exe and an associated document create no pending/completed sessions or playtime. Test failed starts,
  missing files, role changes during a run, locked/unconfigured state and the chosen recent-play policy.
  Use fake clocks/openers and harmless helper processes; never the owner's games or utilities.
- Relink tests cover nested and external paths, absent targets, deduplication, duplicate merge/capacity and
  rollback. Backup tests cover both old archives and multi-file round trips without media-path rewriting.
- Headless Edge exercises Add…/cancel, checkbox discovery, labels, star promotion, removal, Ctrl+S/Esc,
  dropdown visibility/actions, primary-only behavior, Details activation, missing-file errors, late replies,
  EN/RU language switches and narrow layouts. Existing history/editor/preview/privacy checks still pass.
- Compare mock screenshots of the Play controls and Details with `game-page.png` at the same width, and
  record remaining differences. Do not claim editor mockup parity without an exported editor reference.
- `go vet ./...`, `go test ./...` and `game-launcher/build.ps1` pass; regenerate bindings and pass locale
  checks. Record which OS dispatch paths were actually executed. Native dialog/association behavior and
  the built exe on the real library remain explicit manual acceptance items unless actually tested.

## Step 1 delivery

Implemented in `launch_files.go` and `internal/launchfiles`, with generated Wails bindings:

- `FindLaunchFileCandidates(gameID)` returns `Candidates { files, incomplete, reason }`. `FileInfo` contains
  `path`, `name`, `relative_path`, `label`, `label_key`, `primary`, `availability`. Discovery rows are suggestions;
  their role/availability fields are not saved-file checks. Reasons are `entries`, `candidates`, `time`, `unreadable`.
- `GetLaunchFileAvailability(gameID)` returns saved files, primary first, with `available`, `missing`,
  `unavailable` or `unsupported`. `SelectLaunchFile(gameID)` opens the picker from the saved folder.
- `GetLaunchFileLabel(path)` returns `launch.label_settings`, `launch.label_walkthrough` or an empty key.
  Render explicit `label` first, otherwise translate `label_key`, otherwise use `name`. Walkthrough wins for
  mixed hints. Custom text is never translated; automatic text is never persisted in a label field.
- `LaunchGameFile(gameID, path)` and legacy `Launch` use the saved path's role and working directory.
  Direct extras detach without a monitor; associated files use ShellExecute on Windows and open/xdg-open
  on macOS/Linux. PDF/TXT remain association-based even with executable bits. No shell filename interpolation.
- `UpdateGame` commits the entire draft. Step 2 must construct star swaps and first-add promotion in that
  draft; saving an empty primary deliberately does not promote an existing extra. A missing/null extras array
  from an older caller preserves the stored list; an explicit empty array clears it.
- Relink merges files in the same transaction as existing totals/collections/session transfer. If the primary
  fallback selects a different executable, its old custom label is cleared; matching extra labels can follow
  the selected path. Extra paths never use that fallback. Merge overflow survives ordinary saves and backup.
- New errors and automatic labels exist in all 11 locales. UI components are unchanged. `build-ui.cjs` wires
  `launch-files-fixture.cjs`, including multi-file data, missing files, capacity, delayed/error/partial discovery
  controls (`window.__launchFilesMock`, `?shot=launch`) and all new API stubs; no real files are opened.

Verification performed on **Windows/amd64**: `go vet ./...`, `go test ./...`, `wails generate module`,
`build.ps1`, the packaged-exe `TestBuiltMonitorEntry` smoke check, mock build/syntax check and all 39
existing headless Edge game-page checks passed. Backend
coverage includes old/NULL migrations, round trips, label swaps/removal, duplicate/capacity rejection,
relink overflow/rollback, old/new backup imports, bounded discovery and symlink exclusion, locked/unconfigured
and late-lock requests, failed starts and extra-launch accounting isolation. Real harmless helper processes
exercise Windows direct dispatch and survival after parent exit; association openers are fakes.

Linux/amd64 and macOS/amd64 launcher, discovery and DB test packages were cross-compiled, **not executed**.
The POSIX unreadable-root permission test is skipped on Windows; Windows ACL denial was not exercised.
Native file dialogs, installed document associations and the owner's real library/games remain manual checks.
Step 2 was pending at that delivery; its acceptance and comparison are recorded below. No README images were refreshed.

## Step 2 delivery

- GameDetail now uses Play / chevron / favourite. The always-visible chevron opens the existing ContextMenu
  with launch rows only for two or more saved files, then a separator and all previous game actions.
  File rows have full-path tooltips, filename secondary text, a primary star and muted availability hints.
  Keyboard navigation, activation, Escape, outside dismissal and return focus work for this combined menu.
  Card right-click and other menus retain their actions and layout.
- Details renders primary-first links in the existing card. Each calls LaunchGameFile; Play calls the same
  method for the primary. Empty Details and Play without a primary focus the editor's launch-file block.
- LaunchFilesEditor replaces the single path field inside the existing GameEditor card. Star swaps include
  labels; removing the primary leaves it empty; first-add promotion occurs only when adding to an empty
  primary slot. Labels are optional inputs with localized automatic placeholders. Eight-extra limits and
  retained relink overflow are enforced without truncation. No file operations persist before Save/Ctrl+S.
- Find in folder is an inline checkbox picker with loading, empty, failure/retry, partial-list and selection
  states. Already-listed/aliased paths cannot be selected twice. Escape inside the picker closes it first;
  editor Escape uses the existing discard confirmation. API replies are invalidated on destruction,
  preview entry, cancellation and navigation/lock/panic; a late native-picker reply cannot mutate a saved game.
- Uses the five existing launch-file APIs. The backend is unchanged. Twenty UI strings are added to all
  eleven locales; stars/chevrons use lib/icons.js. Explicit labels remain literal when switching language.

Windows/amd64 checks: `go vet ./...`, `go test ./...`, `build.ps1` (including generated bindings) passed.
Vite still reports existing accessibility/unused-export, Browserslist-age and bundle-size warnings.
Headless Edge on the fictional mock library passed **201 assertions**: test-launch-files 69,
test-game-page 39, test-stats 51 and test-find 42. New coverage includes native-picker cancellation (mocked),
checkbox selection, alias rejection, labels and swaps, save/discard/removal, empty/one/many-file menus,
capacity/overflow, keyboard focus, missing files, recent-play isolation, errors/retries/partial results,
late responses after history/lock/panic, EN/RU switches and 900px layouts. Preview tests explicitly exclude
all new launch controls. Native dialogs and OS openers are mocked, not real acceptance tests.

Visual comparison: `test-launch-files.cjs` captures the right column at its natural **340px** width;
`compare-launch-files.py` crops the three reference cards and proportionally displays the mockup's **266px**
column at the same 340px width. It does not override app CSS. Result: `out/launch-files-comparison.png`,
with the raw capture in `out/launch-files-right.png`; editor and narrow EN/RU captures are in the same folder.
The Play / chevron / favourite order and primary-first file list match the board. Remaining differences:

- The app retains larger typography, padding and gaps: the captured column is 1177px tall versus 914px for
  the proportionally scaled reference. These are the existing page styles, not a new editor design.
- Details and Play sessions align with the play card in the app. The board insets these two cards to the right.
- The update actions wrap onto two lines at this width; the board places them on one line.
- Details keeps Added before Collections; the board reverses them. File rows show the agreed custom/automatic
  labels instead of the board's raw filenames. Fixture titles, dates, stats, art and theme colors also differ.

No README screenshot assets changed. This step was executed only on Windows; Linux/macOS UI and native
association/file-dialog behavior, the built window on the real library and actual game launches were not
verified. Step 1's Go process tests still use harmless helpers and fake association openers.

## Decisions

Confirmed by the owner on 2026-11-07; no open implementation questions remain.

1. Exactly eight additional files. Validate additions; retain overflow produced by relink merges.
2. Game-page controls are Play / chevron / favourite. The chevron is always visible: files first only when
   there is more than one, separator, then every existing ellipsis-menu action. Remove the separate ellipsis.
   Right-click on cards and other menus remain unchanged.
3. Star promotion swaps paths and labels with the previous primary. Removing the primary leaves it empty,
   with no automatic promotion. The first added file when there is no primary becomes primary.
4. `exec_label` belongs to the primary path and follows it during swaps. Empty labels use localized inference;
   explicit labels remain literal across language changes.
5. Discovery covers root plus two levels, with limits of 10,000 entries, 200 candidates and five seconds.
   Report incomplete results. Ordinary README/manual documents are excluded; document hints are
   config/settings/setup/walkthrough/guide. Never run a candidate to infer its purpose.
6. Extras do not update last launch, Continue playing, session/playtime totals or restart blockers.
7. Relink keeps the survivor primary/custom labels and appends unique duplicate entries, retaining overflow.
   The primary keeps the current best-executable fallback; extras retain their expected rebased paths when
   missing, without fallback to another executable.
