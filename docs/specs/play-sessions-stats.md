# Spec: play sessions and statistics

Status: **steps 1 and 2 implemented** (2026-11-06). Uses the recommended detached monitor, local calendar,
completed-session history and permanent sidebar entry. Backend checks and headless Edge UI checks pass;
real-library acceptance on the built application remains manual. See delivery notes below.

## Why

The launcher already stores total playtime, but cannot show when that time was spent. Sessions add a recent
history on the game page and a library-wide Statistics view. Existing playtime remains useful immediately;
dated history starts accumulating with this feature, without inventing sessions for old totals.

## User flow

### Play and keep the session

- One successful tracked launch creates one session: game ID, start time and duration in seconds. Multiple
  launches, including simultaneous launches of the same game, have distinct session IDs.
- Record completion at the existing `cmd.Wait` tracking point in `internal/launcher/launcher.go`. Sessions
  shorter than 60 seconds are discarded; exactly 60 seconds qualifies. A failed launch creates no session.
- Closing the launcher while the game is running must not lose the session. **Proposed:** a small background
  monitor survives the UI process, waits for the game to exit and saves the duration (see Backend and Q1).
  Reopening the launcher must not create another session or another monitor for the same launch.
- Locking or using the panic key hides information but does not stop recording. No session titles or details
  appear in notifications while locked. Sessions have no manual start/stop controls in this version.
- Tracking covers the same directly launched processes as today's playtime. Files opened through OS
  associations (HTML, shortcuts, some scripts, macOS app bundles, etc.) currently have no waitable game
  process: do not fabricate a session from the short-lived opener. Explain unavailable tracking quietly on
  the game page. Tracking child processes after a bootstrapper exits remains a separate feature.

### Game page: Sessions

- A **Sessions** block in the left column after the description and before notes, only on the saved game page;
  omit it from metadata preview and the editor.
- Eight vertical weekly bars: current week plus seven previous weeks, chronological left to right, with zero
  weeks included. Height means played time, not launch count. Show dates and duration on hover and focus.
- Under the chart: **Sessions** (all recorded sessions for this game) and **Longest session** (all recorded
  history, not just the eight weeks). Then the latest five sessions, newest first, with local date/time and
  duration. A recovered incomplete session is marked as such (Q1).
- Keep the existing all-time playtime in the play card, sourced from `time_played`. Empty session history
  says “Sessions will appear after you play for at least a minute”; old playtime remains visible. Add a muted
  explanation that earlier playtime has no dated history. Never display “never played” based on zero sessions.
- **Proposed:** show completed sessions only; refresh when a game exits or after reopening the launcher.
  There is no ticking duration or provisional bar for a currently running session (Q4).

### Main area: Statistics

**Proposed entry:** a permanent, labelled **Statistics** row in the sidebar, directly below the scan/add/update
buttons and above Collections. It stays visible even with an empty library or collapsed collections. This is
a navigation destination, not a collection or another grid/shelves layout. The titlebar already holds search
and privacy/window controls; adding a text-labelled destination in the sidebar is easier to discover (Q2).

- Opens a separate main view headed **Statistics**, with the normal titlebar and sidebar retained. Hide library
  sort, tag chips, selection actions and grid controls here. Sidebar highlight must not also select All games.
- Scope is the whole **visible library**, independent of the previously selected collection, tag or search.
  Typing in titlebar search returns to library search, following today's library-wide search behavior.
- First row: **All-time hours**, **This month**, **This week**. All-time uses `time_played`; month/week use
  sessions. A persistent muted note explains “Period totals and charts include recorded sessions only”.
- **Weekly activity**: 52 bars (current week plus the previous 51), including empty weeks, showing time.
  Month labels plus a date-range/duration tooltip; no daily heatmap. This rolling-year interpretation is
  proposed in Q3.
- **Top games**: period selector **This week / This month / All time**, default This month; up to ten games,
  sorted by duration descending, then title and ID for ties. Show rank, name, duration and a horizontal bar.
  Omit zero-time games. All-time uses each game's `time_played`; other periods use intersecting session time.
  Selecting a game opens its detail; Back restores Statistics, its period and scroll position.
- **Games by status**: counts for playing, want, finished, dropped and **No status**, using the existing labels
  and colors. **Engines**: counts by engine, including **Unknown**; show the largest five and Other, with an
  expandable full list. These are current library counts, unaffected by the Top games period selector.
- **Disk space**: sum known `size_bytes` for visible games whose folders exist. Show measured-game count and
  how many sizes are unknown or folders missing; cached sizes are estimates, not a live filesystem scan.
  A checked zero-byte folder is known (`size_checked_at > 0`); an unchecked zero is unknown. Missing folders'
  stale bytes do not contribute. No automatic full rescan on opening Statistics.
- **Library information**: counts of games with updates, without a cover, without a source link. Definitions
  match existing library helpers (`update_available`, empty `cover_path`, no usable source/legacy URL).
  Counts may overlap. First version displays counts only; new smart lists/bulk actions are out of scope.
- Empty library and no activity are normal states: zero totals, empty charts, readable guidance. Fetch errors
  offer Retry and do not masquerade as zero activity. Compact layout stacks sections at smaller widths.

## Backend

### Existing behavior and constraints

`Controller.LaunchGame` currently owns `cmd.Start`/`cmd.Wait` and calls `onExit(minutes)`. `App.addPlaytime`
reads a whole Game, adds minutes and saves it. `shutdown` closes the repository; there is no durable running
session. Therefore a shutdown-only flush cannot record the game's actual exit after the launcher has closed.
Saving a start time and checking a PID on the next startup also cannot reconstruct when an already-ended
process exited, and risks PID reuse. Neither alone satisfies the normal-close requirement.

`time_played` is **integer minutes** today. Keep its schema and meaning: a completed qualifying launch adds
`floor(duration_seconds / 60)` once. Sessions keep seconds. Thus 90 seconds gives 1 stored minute and a
90-second session; totals and session sums can differ slightly even without legacy playtime. Do not rebuild
`time_played` from sessions or backfill fictional history.

### Storage and finalization

- Add a `play_sessions` table: unique `id`, `game_id`, `started_at` (UTC Unix seconds), `duration_seconds`
  (integer, at least 60). **Proposed:** `interrupted` boolean for checkpoint recovery. Index by
  `(game_id, started_at)` and `started_at` for recent history and library date-range queries.
- Keep pending launches in a separate durable table, not in completed history: unique session ID, game ID,
  start time, monitor identity, last confirmed duration/checkpoint, and completion state needed for retry.
  These are internal lifecycle fields, not user-editable Game fields.
- Persist launch intent before starting the game; acknowledge successful start and its actual start timestamp.
  On start failure discard the intent. Measure elapsed time with a monotonic clock in the monitor, floor to
  whole seconds at completion; use UTC timestamps for calendar placement. Do not use minute callbacks as
  the source of session precision.
- Finalize in one transaction: consume the pending launch, insert the qualifying session, atomically increment
  the current game's `time_played`, and mark the session handled. Below-threshold launches only clear pending
  state. Duplicate completion/retry must add neither another session nor more minutes.
- Replace whole-Game playtime writes with a dedicated repository operation. Normal editor/metadata saves must
  no longer overwrite `time_played` or tracking-owned `last_launched_at` with stale values. Relink/import retain
  explicit controlled writes. Parallel exits and edits must not lose time or overwrite personal fields.
- Finalization looks up the current game ID through pending state: relink can reassign it, deletion can cancel
  it. A late completion must never resurrect a removed game. Nonzero game exit codes still count elapsed time.
- Migration is additive and repeatable. Old libraries keep their totals; old backups migrate to empty session
  history. Record history-availability metadata for honest empty-state text; it is not evidence of prior play.

### Surviving launcher closure (proposed; Q1)

Use a **detached monitor process per tracked launch**, implemented as an internal mode of the same executable,
entered before Wails/window initialization and the UI single-instance check. It owns the game process from
`Start` through `Wait`, reusing the launcher tracking code. A goroutine in the UI process cannot survive exit;
starting the monitor only at shutdown would make ownership transfer needlessly fragile.

- Launch it without a console/window; use appropriate process detachment on Windows/Linux/macOS. It owns a
  narrow repository connection to the selected data directory, with no frontend/runtime dependencies.
  Start handshake returns success/failure to the UI, so launch errors still reach the user.
- Persist checkpoints approximately every 30 seconds while waiting. Checkpoints record elapsed duration for
  crash recovery; completion detection remains `cmd.Wait`, not repeated process-list scans.
- Normal UI close terminates the UI only. The monitor remains until the game ends and commits the full
  session. Reopening reads persisted state; an active monitor is identified using a token/process identity
  with start identity or a lifetime lock, never a PID alone. A slow/stale heartbeat alone is not proof of death.
- If the monitor/OS dies, recover at most the last durable checkpoint, mark the session interrupted, and apply
  the same 60-second threshold. Never count the interval until the next launcher startup as playing. With
  successful checkpoint writes, up to one interval can be lost; storage failures can lose more. Exact duration
  after a power failure is not promised. If the game is
  still alive but its monitor died, this version retains only confirmed time and does not pretend to reattach.
- Transient database errors retry finalization without duplicating minutes. Retain pending completion data
  until committed; surface persistent recording failures through `apperr` when the UI is available/unlocked.
- Coordinate launch with destructive data operations: while any monitor is active, prevent backup **import**,
  changing/moving the data directory, and launcher executable replacement/restart for update. Downloads and
  normal UI close can continue. Use a shared lifecycle gate so a launch cannot race the check. Explain the
  blocked operation through a localized error. Existing in-memory `Running()` is insufficient after reopen;
  update blockers must also consult monitor state. No monitor may keep writing an obsolete/replaced DB.
- The UI refreshes after local completion and on startup; with external monitors, observe a lightweight DB
  revision while the UI is open (or an equivalent local notification), including exits after UI restart.
  Do not broadcast private Game/session payloads while locked.

An alternative is to keep the whole launcher process alive invisibly until games exit and reuse its existing
wait goroutines. It is simpler, but Close no longer exits the launcher, explicit Quit needs a policy, and the
webview/backend stay resident. Q1 asks the owner to choose before implementation; neither approach is a
system-wide service or tracks games launched outside pLauncher.

### Delete, relink and backup

- Every removal path (single, bulk, ignore, delete-all) deletes completed and pending session data with the
  game in a transaction. If using `ON DELETE CASCADE`, enable SQLite foreign keys on **every connection**;
  current connections do not enable them. Completion after removal is a no-op; do not terminate the game.
- Relink without merge keeps IDs and history. Merge moves the duplicate's completed and pending sessions to
  the surviving ID before deletion, and sums stored playtime once. Session IDs stay unchanged. Move history,
  totals and duplicate deletion atomically; preserve the existing collection merge. Never deduplicate distinct
  launches just because their timestamps match. Retry and concurrent completion must be safe.
- Sessions live in `games.db`, so the existing `VACUUM INTO` backup includes them. Export is a consistent
  point-in-time snapshot, allowed during play. On import, pending records in the snapshot are converted to
  interrupted sessions at their saved checkpoints (threshold/once-only rules apply), never attached to a
  process on the destination machine. Already completed sessions are restored unchanged.
- Validate migration, deletion and relink with backup round trips, including archives predating this table.

### Queries, calendar and privacy

Proposed JS-bound contracts (exact Go DTO names may be refined in step 1):

- `GetGameSessions(gameID, visibleGameIDs, timeZone)` returns total session count, longest duration, the eight
  weekly buckets and latest five entries. Return nothing if the game is outside the supplied visible scope.
- `GetLibraryStats(visibleGameIDs, period, timeZone)` returns the three time totals, 52 weekly buckets, top ten
  game IDs/durations, status/engine counts, disk summary and the three library-information counts. Top-games
  period is a validated enum, not arbitrary SQL. Return seconds for durations; format them in the frontend.
- Pass an explicit deduplicated ID list from **`visibleGames`**, not `games` or the filtered grid. Empty/nil
  scope means no games, never all games. Intersect with existing IDs using parameterized queries; avoid N+1
  queries and SQLite parameter-limit failures for large libraries. Unknown IDs contribute nothing.
- This follows the existing app's visibility model: hidden collections are resolved in the frontend through
  `buildCol`, including dynamic tag matches and pinned IDs. Membership in any hidden collection excludes a
  game even if it also belongs to a visible one. Do not invent an independent backend tag-matching rule.
  Passing IDs scopes UI statistics; it is not a new authorization system for arbitrary bridge callers.
- All new read methods return no payload when `repo == nil` or locked, including totals/bucket labels. Check
  lock state again before returning. Frontend clears cached results on lock/panic/hide, invalidates in-flight
  requests and refetches with the new visibility scope after unlock/reveal. No hidden names, bytes or hours
  survive in charts, tooltips or empty-state counts. Recording continues independently of these read guards.
- **Proposed calendar (Q3):** current OS local IANA time zone supplied by the UI and validated by the backend;
  package timezone data where needed. UTC is storage only. Weeks start Monday at local midnight; month means
  current calendar month. Periods end at a single captured query time, with exclusive upper bounds.
- Attribute duration by intersection of `[start, start + duration)` with each local calendar bucket. Split
  crossing-midnight/week/month sessions for hours/charts without splitting the stored session or its count.
  Use calendar boundaries, not fixed 7*24-hour offsets across DST. A timezone change regroups history without
  changing timestamps. Return bucket boundaries for consistent localized labels.
- Simultaneous sessions add their durations (game-hours, not deduplicated wall-clock hours), matching existing
  total-playtime semantics. Counts/longest/latest use whole sessions. Current library status/engine/disk data
  are not historical snapshots. Removed games no longer contribute to any aggregate.
- All user-visible backend errors use `apperr` and `err.<code>` in all 11 locales.

## Frontend

- Suggested components: `GameSessions.svelte`, `Statistics.svelte`, and a small shared `WeeklyBars.svelte`.
  App owns the Statistics navigation destination, period and scroll restoration; include it in `navStack`.
  Esc/Alt+Left/mouse Back follow existing navigation and top-overlay precedence (`e.defaultPrevented`).
- Charts use simple SVG or CSS bars in the current flat dark palette, no external chart library. Include
  accessible names and textual totals; hover information must also be available through keyboard focus.
  Zero data must not divide by zero. Localize dates/numbers/durations and provide a readable year chart at
  narrow widths (horizontal scrolling is acceptable).
- Icons come from `lib/icons.js`; no emoji. Any game thumbnails use `mediaSrc` and `blurCls`, including
  discreet mode. Prefer text-only Top games rows initially to keep this view compact.
- New copy, plural forms, chart labels, empty/error states and interrupted markers go into **all 11 locales**.
  Language switches update labels immediately. Svelte 3 derived expressions must explicitly reference
  language, game/session data, selected period and visibility state; do not hide dependencies in helpers.
- Reload on recorded-session revisions, library/collection changes and period changes. Recompute calendar
  ranges when returning to the view and crossing a local day boundary. Discard responses from older
  navigation, visibility or request generations. Do not expose another game's history during rapid navigation.
- Extend `docs/tools/screenshots/build-ui.cjs` with deterministic sessions around calendar boundaries, an old
  game with playtime but no sessions, an empty game, hidden/manual/dynamic collection fixtures, interrupted
  sessions, unknown/missing sizes and loading/error responses. Use a fixed clock for reproducible screenshots.

## Delivery steps

### Step 1: backend only

Migration, pending/completed sessions, seconds-based launcher lifecycle, chosen closure
strategy, atomic time accounting, cleanup/relink/backup integration, scoped aggregate queries and App methods.
Add backend tests, localized errors and regenerate bindings with `build.ps1`. No Statistics or Sessions UI yet.
Recording becomes available in this delivery; existing totals and library behavior must keep working.

### Step 2: interface

Add the game-page block, sidebar destination and Statistics view; wire navigation, visibility, refresh,
localization and accessibility. Add screenshot mocks and headless Edge flow checks. Update AGENTS.md,
CHANGELOG `[Unreleased]` and this spec's status after each delivery, recording actual validation and limitations.
Commit each finished step separately without attribution; do not push unless requested.

## Design choices

Both requested steps are implemented using the recommended choices in Q1–Q4, including the sidebar entry
for Q2. No implementation-blocking questions remain. Alternatives below are retained as design context.

1. **How should closing the launcher behave?** Recommended: detached monitor, with checkpoint recovery after
   crashes and guards on import/data moves/update installation while playing. Alternative: keep the launcher
   itself running invisibly until games end (requires an explicit Quit policy). Confirm whether the proposed
   interrupted-session marker and up-to-30-second crash loss are acceptable. Normal UI closure must retain
   the full session with either design; saving only time up to Close does not meet that requirement.
2. **Where should Statistics live?** Recommended: permanent sidebar row below action buttons, above Collections.
   Alternative: a labelled Library/Statistics switch in the main header. Choose one primary entry; no duplicate
   titlebar icon is proposed.
3. **Which calendar conventions?** Recommended: this calendar month, this Monday-based week, and rolling 52
   weeks including the current week, all in the current local timezone. Alternatives: trailing 30/7 days for
   totals, Sunday-start weeks, or the current calendar year (variable 52/53 buckets). These change chart labels
   and backend boundaries and must be settled together.
4. **Include the running session in the UI?** Recommended first version: completed/recovered sessions only,
   refreshed on exit, with all-time hours continuing to use stored `time_played`. Alternative: provisional
   elapsed time and an “In progress” row/bar, requiring refresh and clear separation from committed totals.

## Out of scope (later)

- Manual session editing/deletion, manual time entry, importing Steam history or inventing dates for old time.
- Tracking games started outside pLauncher, OS association/child-process discovery, foreground/idle detection,
  pause/resume controls, achievements, streaks, goals or cloud telemetry.
- A daily heatmap, arbitrary date-range picker, collection-specific reports, exportable reports, new smart
  lists, or a charting dependency. Statistics do not trigger update checks or disk scans by themselves.
- Exact reconstruction after power failure or monitor termination; retain only durable confirmed time.

## Done when

### Step 1

- Tests cover 0/59/60/61/90-second launches, failed start, nonzero exit, concurrent launches, idempotent
  finalization, stale editor saves and unchanged legacy totals. Use fake clocks/processes, not minute-long sleeps.
- Process integration proves launch -> UI exit -> game exit -> reopen yields one full session and one playtime
  increment. Test reopening while still playing, checkpoint recovery, PID reuse/stale state and recording
  failure/retry. Validate detachment on each supported OS; record platforms not actually tested.
- Delete/bulk/ignore/delete-all, active-session relink, transaction rollback, old/new backup import/export and
  import/data-move/update guards cannot orphan sessions, double time or allow writes to the wrong database.
- Aggregation tests cover old games without sessions, no data, scope deduplication/empty scope, threshold,
  ties, midnight/month/year and DST crossings, overlapping launches, eight/52 zero-filled buckets, unknown
  engine/status/size, missing folders, locked/unconfigured state and lock during a request.
- `go vet ./...`, `go test ./...`, then `game-launcher/build.ps1` pass; generated bindings and locale checks pass.

### Step 2

- Headless Edge clicks Statistics -> change period -> open a top game -> inspect Sessions -> Back, preserving
  period and scroll. Eight/52 bars, summaries, latest five and old all-time totals match fixed mock data.
- Verify hidden collections (manual, dynamic and mixed membership), reveal/hide, PIN lock/panic and late
  replies; hidden data never survives in aggregates. Check metadata preview/editor omit the Sessions block.
- Verify empty/error/retry states, live language changes, keyboard chart access, narrow layout, privacy blur
  if thumbnails are used, and refresh after a completed session. No external requests or chart libraries.
- Repeat vet/test/build, inspect English/Russian screenshots and update delivery notes. The owner separately
  tests the built application with real games; mock results must not be described as real-process validation.

## Backend delivery (2026-11-06)

Implemented detached monitors, OS lifetime locks, 30-second checkpoints, interrupted recovery and atomic
once-only finalization. The internal monitor mode runs before the UI single-instance and updater code;
normal UI close does not terminate it. Game deletion cancels recording but leaves the running game alone.
Recovery occurs at startup and while the UI remains open, including behind the PIN lock. Error events and
read methods remain silent while locked. Association-based launches stay untracked.

A gate serializes monitor startup with import, data-directory changes and update installation. It is also
checked at startup before installing a queued update. Locks are outside the data directory, so holding the
gate does not prevent moving that directory on Windows. Lock identity uses a canonical path hash and a unique
session token, not PID reuse or heartbeat age. Deleted running games still block data/executable replacement
until their monitor exits. Export remains a point-in-time snapshot; imported pending rows become interrupted
sessions at their saved checkpoints (or completed sessions if completion was already durably recorded).

`SaveGame` preserves tracking-owned fields on updates. Dedicated operations set launch time and increment
playtime. Relink moves history, pending ownership, collection membership and current totals transactionally;
all removal paths cascade through both session tables. Legacy time stays in minutes and is never synthesized
into session history. A metadata migration records when session recording became available.

Bound methods: GetGameSessions, GetLibraryStats and GetSessionRevision. Explicit visible-ID scopes use one
JSON SQL parameter, so duplicates and very large libraries cannot double totals or exceed parameter limits.
Statistics are read from a consistent database snapshot. Returned duration fields use seconds; periods are
`week`, `month`, `all`; timezone names are validated against bundled IANA data. Eight/52 weekly buckets use
Monday boundaries and split intervals at local calendar boundaries. Empty scopes have zero activity.
The full engine count list is returned; grouping the first five and Other is a frontend presentation task.
GameSessions also returns `tracking_supported` for the untracked-file explanation.

Payload-free `sessions-updated` / `games-updated` notifications refresh clients after an external monitor
finishes; `session-recording-error` and GetSessionRevision surface persistent write failures using apperr.
The new error codes are translated in all 11 built-in locales. The frontend event handlers, charts and
navigation are explicitly left for step 2.

Validation: `go vet ./...`, `go test ./...` and `game-launcher/build.ps1` passed on Windows. The build
regenerated Wails bindings and produced the executable; existing Svelte accessibility/unused-export and
bundle-size warnings remain. Source diff checks pass; Wails-generated models retain the generator's normal
blank-line whitespace and were not hand-edited.

Tests cover threshold boundaries (0/59/60/61/90 s), repeated/concurrent finalization, stale saves, checkpoint
and final-write retries, transaction rollback, deletion paths, active relink, backup restoration and legacy
migration. Aggregate tests cover large/duplicate/empty scopes, old totals, rankings, latest five sessions,
unknown metadata/sizes, calendar/year/DST crossings, locked/unconfigured methods and lock during a response.

Real Windows helper-process tests exercise parent exit -> monitor continues -> reopen while playing -> game
exit -> one session/one increment, nonzero game exit, monitor termination and checkpoint recovery. Qualifying
durations use an injected accelerated clock. An additional opt-in `TestBuiltMonitorEntry` ran against the
packaged `build/bin/pLauncher.exe` (`PLAUNCHER_BUILT_EXE`): the early windowless monitor entry and startup
handshake worked, and a short helper launch was discarded with the real clock. Tests use temporary libraries;
none touch the owner's library or launch actual games.

The launcher/monitor test package also compiled for Linux/amd64 and Darwin/amd64; these were compile-only
checks, not executions or full Wails builds. Actual process detachment on Linux/macOS, real user games,
power-loss behavior and the native interactive UI remain untested. The frontend delivery is recorded below.


## Frontend delivery (2026-11-06)

Implemented `GameSessions`, `Statistics` and shared `WeeklyBars` with CSS bars and no chart dependency.
The game page shows eight weeks, all-history count/longest and latest five local dates/durations, interrupted
markers and empty/untracked/legacy-history explanations. Preview/editor omit this block. Statistics is a
permanent sidebar destination, with all three hour totals, 52 weeks, a period-selectable top ten, status
counts, five engines plus expandable Other, cached disk summary and library-information counts.

App navigation retains the Statistics period and scroll position through game details, Esc, Alt+Left/Right
and mouse Back. Library controls are absent from this view. Search returns to the library and filters the
complete in-memory list; it no longer replaces the data needed by hidden collections and statistics.
Queries always receive the full visible scope, including manual/dynamic/pinned/mixed-membership exclusion.

Request generations clear cached hidden aggregates on visibility changes, reject late responses after
navigation/lock/panic and distinguish failures from empty data. Session/library events refresh both views;
recording errors use localized toasts only while unlocked. Local date/timezone changes trigger new queries
on focus, navigation or the 30-second calendar check. Language dependencies are explicit for Svelte 3.
All 40 new strings exist in all 11 locales; Intl handles dates, numbers and duration units. Top games are
text-only. Charts have accessible names, keyboard focus/arrow navigation and textual date/duration tooltips.

Validation: `go vet ./...`, `go test ./...` and `game-launcher/build.ps1` passed on Windows; Wails regenerated
bindings and built the executable. Existing accessibility/unused-export, Browserslist and bundle-size warnings
remain; the new chart/session/statistics components introduce no compiler warnings.

The deterministic `?shot=stats` mock freezes time and supplies all three new bound methods through
`build-ui.cjs` / `stats-fixture.cjs`. Headless Edge passed 48 assertions: fixed totals/month-boundary splitting,
legacy time, eight/52 bars, latest five/interruption, period and exact scroll restoration, engine expansion,
disk exclusions, search/collection independence, manual/dynamic/pinned/mixed hidden membership, reveal/hide,
lock/panic and late results, errors/retries/toasts, editor omission, completed-session refresh, local day and
timezone changes, live EN/RU localization, keyboard charts, narrow layout and empty-library guidance.
No external requests or runtime exceptions occurred. English/Russian and narrow screenshots were inspected.
The existing find-by-name flow also passed 35 assertions, including Sessions omission from preview.

These are mocked browser checks, not tests against the owner's database or actual games. The native Wails
window, real-game interaction, other operating systems and all 11 languages' visual layouts were not manually
exercised in this step. Backend lifecycle coverage remains as documented in step 1.
