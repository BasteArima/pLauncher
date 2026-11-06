# Spec: multiple launch files

Status: **step 0 — specification only** (2026-11-07). The owner has decided the core behavior below.
Implementation has not started; remaining choices and recommendations are listed under Open questions.
Design reference: [A2 · Game page](../design/game-page.png), interpreted using
[the design README](../design/README.md): the small dropdown beside Play and the Launch files list in Details.

## Why

A game folder can contain the game itself, a configuration utility and a walkthrough. Today pLauncher
remembers only one launch path. Keep Play predictable while making the other files available from the same
game page, without counting time spent in a guide or settings utility as playtime.

## Agreed scope

- One primary file, marked with a star, remains in `Game.ExecPath` / `exec_path`. Existing libraries and
  single-file games continue to work without conversion by the user.
- Up to approximately eight additional files, each with a path and an optional editable label. Paths use
  the same representation as `exec_path`; do not move them into the data folder or introduce a new URI scheme.
- Play opens the primary file. A separate dropdown beside Play lists all files, with the primary starred.
  Without additional files, this dropdown is absent.
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

- Keep the filled Play button in the right-hand play card. Immediately beside it, show a compact chevron
  button when at least one additional file is saved, as in the mockup. It is a distinct target: Play never
  opens a chooser and the chevron never launches a file by itself.
- The dropdown shows the primary first, then the saved additional-file order. Each row shows its effective
  label, falling back to the filename; retain the filename/path as secondary text or a tooltip so identically
  labelled utilities remain distinguishable. The primary has a star and an accessible Primary label.
- Choosing the primary uses exactly the same launch path and tracking policy as Play. Choosing any additional
  file opens it without recording, including a second game executable. There is no per-file tracking toggle.
- Card Play, sidebar Play, the game menu's Play and Ctrl+Enter continue to target the primary. Additional
  files do not become implicit fallbacks when the primary is missing.
- Use the existing custom-menu conventions: anchored placement that stays inside the window, focusable
  entries, keyboard activation and Esc/outside-click dismissal. Closing the menu must not also leave the
  game page. Respect the `e.defaultPrevented` and top-overlay precedence contracts.
- Preserve access to the existing game-actions menu (Edit, Remove, etc.). Its placement alongside the new
  dropdown needs the choice in Q2; the launch menu must not silently replace those actions.
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
- Making a row primary updates the draft's `exec_path`. Proposed behavior: swap it with the previous primary,
  keeping the old primary as an extra and preserving both labels (Q3). This does not launch either file.
- Remove removes only the library reference. It never deletes a file from disk. Primary removal and the
  no-primary state are specified as proposed choices in Q3, not an implicit automatic promotion.
- Add… opens a native file dialog starting in the game's folder, with supported launch formats and
  HTML/PDF/TXT documents included. Keep an All files option, as today. Cancellation is a no-op. Handling
  a missing starting folder follows the existing dialog fallback, without changing the game's folder.
- Find in folder opens an in-editor candidate picker: checkbox, effective label, filename and relative path.
  It has loading, empty, error/retry and selected-count states, plus Add selected and Cancel. Already-listed
  paths are marked and cannot be added twice. Proposed: new candidates start unchecked.
- Neither scanning candidates nor changing checkbox selection persists anything. Add selected appends to
  the editor draft; Save/Ctrl+S commits the complete list with the rest of the game. Esc/cancel uses the
  existing dirty-state confirmation and restores paths, primary choice, labels and order together.
- Enforce the chosen capacity in the picker and Add… controls, with a localized explanation. The backend
  validates it too. Do not silently truncate the user's selection.
- Autofill labels as described below; permit editing immediately after adding. Exact editor visuals beyond
  the existing editable page are not supplied by `game-page.png`. If the private A4 board is required for
  a more detailed layout, obtain its export before implementing that design rather than inventing it.

## Backend

### Existing behavior and integration points

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

Proposed storage (names may be refined during the backend step; the primary-path contract may not):

```go
type LaunchFile struct {
    Path  string `json:"path"`
    Label string `json:"label"` // explicit user text; empty uses a localized filename-based default
}

// Additions to Game; ExecPath remains the sole authoritative primary path.
ExecLabel   string       `json:"exec_label"`
LaunchFiles []LaunchFile `json:"launch_files"` // additional files only, in display order
```

- Store `launch_files` as a JSON array in a new `games` TEXT column and `exec_label` as TEXT if primary labels
  are supported as proposed in Q4. Add columns via `ALTER TABLE`; read with `COALESCE`. Legacy empty/NULL
  values mean no extras and no custom primary label. Do not duplicate the primary path in the array.
- Use one normalized path identity for deduplication, membership checks and promotion. Preserve the selected
  path string for storage/launch. Respect platform path semantics and existing `scanner.NormalizePath`
  conventions; no basename-only matching and no file-content deduplication.
- `UpdateGame` validates and saves primary, labels and extras together. Allow missing paths to remain saved:
  a temporarily disconnected drive must not destroy the list. Reject blank extra paths, duplicates and
  over-capacity drafts with localized errors. Missing flags are computed, never persisted in this JSON.
- Scan/drop, `DetectLaunchFiles` and metadata updates preserve extras and explicit labels. Main-file detection
  continues to fill only an empty `exec_path`, never promotes a guide/config candidate or overwrites a chosen
  main file. If a discovered primary already exists among extras, consolidate it without duplicating the path.
- Existing playtime/session ownership and `notes_updated_at` rules remain intact when saving the new fields.
  Regenerate Wails bindings after changing the model/App methods; never edit generated bindings manually.

### Label inference

- Match the basename without extension, case-insensitively, using word boundaries/separators and common
  variants such as `Game_Config`, `settings-x64` and `walkthrough_en`. Avoid substring accidents such as
  labelling `GuidebookishGame.exe` as a walkthrough. Define tested deterministic precedence for mixed names.
- Proposed representation: store only explicit user text in `label`. When it is empty, infer a stable category
  (`settings`, `walkthrough`, or none) and translate it in the UI; no category falls back to the filename.
  This lets automatic labels react to language switches while preserving custom labels verbatim (Q4).
- `setup` is a discovery hint, not an unconditional Settings label: a setup program may be an installer.
  Do not execute candidates to identify their purpose or read PDF/TXT contents for classification.

### Candidate discovery

- Add a separate read-only candidate enumeration operation, reusing executable/native-launcher knowledge
  from the current finders. Do not relax `FindBestExecutable`'s main-game ranking/exclusions globally.
- Include other `.exe` files and the existing supported native/script/game launch formats. Additionally
  include HTML/HTM/PDF/TXT files with names suggesting config/settings/setup/walkthrough/guide; include
  normal HTML game entry points. Return every eligible candidate within the chosen bounds, not just the
  highest-scoring launcher. Final document matching and depth are proposed in Q5.
- Inspect only the requested game's folder, with a bounded walk. Do not follow directory symlinks outside
  it or recurse inside `.app` bundles. Skip assets/caches/save directories, but allow shallow `docs`, `doc`
  and `manual` folders for guides. Existing game-library scans do not automatically add these entries.
- Keep uninstallers and crash reporters out of suggestions. A setup-named candidate is never auto-launched
  or automatically made primary. Manual Add… remains available for files omitted by the heuristic.
- Return path, filename/relative display path and inferred label category. Deduplicate normalized paths,
  order deterministically, and report missing/unreadable root errors rather than pretending the folder is empty.
  If a result/time bound is reached, report that the list is incomplete and retain Add… as a fallback.

### Launching and availability

Suggested App contracts, with exact DTO names to settle in implementation:

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
- Proposed: extras do not update `last_launched_at`, Continue playing or running-game/update blockers (Q6).
  They must never change `time_played`, session counts, longest-session values or statistics activity.
- New operations tolerate an unconfigured app and locked state without exposing file paths or launching.
  Recheck lock state before returning discovery data; the frontend invalidates late replies on lock/navigation.
- Distinguish missing file, inaccessible path, unsupported target/failed OS open, unlisted path and list-limit
  errors through `apperr` and all 11 locales. Folder-missing errors continue to use the relink flow. Keep the
  saved path after failure, provide a useful filename/path in the unlocked error, and permit retry after repair.

### Relink and backup

- Rebase each in-folder entry by its relative path under the old root, preserving labels and saved order.
  Extras outside that root retain their existing absolute path, as external primary paths do today.
- Proposed for an absent rebased extra: retain the expected new path and mark it missing; never run the
  best-executable fallback for that entry. Retain the existing primary fallback unless Q7 changes it.
  Deduplicate paths after rebasing; do not silently overwrite explicit labels.
- Duplicate-record merging must include these fields in `SaveRelink`'s transaction with sessions, totals and
  collection membership. Specify conflict/capacity handling before implementing it (Q7); never drop excess
  entries merely to satisfy the normal editor limit.
- `VACUUM INTO` snapshots include the new DB columns automatically. Import migrates old databases and
  preserves new lists/labels. Keep launch paths as game-file paths, not media paths rewritten to `covers/`.
  Backup contains references, not copies of game executables or guide documents. Moving to another machine
  still requires relink; new and old archives both need round-trip tests.

## Frontend and delivery

- Reuse the current dark surfaces, compact controls and Details typography. Keep the Play chevron distinct
  from media arrows, game-list arrows and the game-actions menu. Use accessible localized control names.
- All UI strings, inferred labels, picker states, capacity messages and error codes go into all 11 locales.
  Explicit user labels are not translated. Svelte 3 expressions must name their language/list dependencies.
- Extend `build-ui.cjs` with primary-only and multi-file games, a labelled config exe, an HTML/PDF guide,
  missing primary/extra cases, duplicate candidate paths, capacity and delayed/error discovery responses.
  Mock all new bound methods; do not open real utilities or documents during browser checks.
- Suggested delivery: step 1 implements storage, launch dispatch/accounting isolation, discovery, relink,
  backup coverage and generated bindings; step 2 implements the editor, dropdown and Details flow with mocks.
  These are future steps, not authorization to begin them in the specification-only task.
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

## Open questions

1. **Exact capacity:** make the approximate limit exactly eight additional files (nine including primary)?
   Recommended: eight, enforced on user additions; legacy/merge overflow must be retained or resolved explicitly.
2. **Game-actions menu placement:** the mockup has Play / chevron / favourite, while the app currently has
   Play / ellipsis / favourite. Recommended: use the mockup's three controls and move the ellipsis to the title
   row. Alternative: retain both small buttons beside Play. Existing actions must stay discoverable.
3. **Changing/removing the primary:** recommended promotion swaps the old primary into the promoted row's
   extra slot. Removing the primary leaves `exec_path` empty and retains extras, with Play prompting to choose
   a primary; no automatic promotion. Should adding the first file to an empty game make it primary, or should
   the user always use the star explicitly? Recommended: first added file becomes primary.
4. **Label semantics:** should the primary also have an editable label? Recommended: yes, stored separately
   as `exec_label`, moving with its path on promotion. Is clearing a label allowed to restore automatic naming,
   or must users be able to suppress the inferred label and show only a filename? Recommended: empty restores
   the localized default; a custom label remains literal across language changes.
5. **Discovery bounds and matching:** recommended root plus two subdirectory levels, including shallow docs,
   and only hint-matched PDF/TXT/guide HTML rather than every document. Should generic README/manual files be
   included too? Recommended initial bounds: 10,000 visited entries, 200 candidates, five seconds, with an
   incomplete-results notice. Setup-named executables are included as requested; their purpose is not inferred
   by running them.
6. **Recent activity for extras:** recommended no `last_launched_at`/Continue playing changes and no new
   running-game blocker for utilities. Primary association-based launches keep today's last-launch behavior.
   Only primary files recording time/sessions is already decided; this question concerns ancillary state.
7. **Relink conflicts:** recommended retain the survivor's primary/custom labels, append unique duplicate
   extras (and its distinct former primary), and retain overflow until the user trims the list; block only new
   additions while over capacity. Keep missing extras at their rebased path without best-exe substitution.
   Should a missing primary retain its expected path too, or keep today's best-executable fallback?
