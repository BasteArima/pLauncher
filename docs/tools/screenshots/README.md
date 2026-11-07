# README screenshots

Regenerates `docs/images/*.webp` (and `social-preview.png`) from the real UI with a fictional library and
generated SFW artwork, so screenshots can be refreshed after UI changes.

```powershell
cd game-launcher; .\build.ps1 -NoTests        # the frontend must be built (frontend/dist)
cd ..\docs\tools\screenshots
.\make.ps1                                    # all ten UI states + banner
.\make.ps1 -Shots home,detail -Port 5394 -CDPPort 9336  # subset / alternate ports
```

Requirements: Windows with Microsoft Edge, Node.js, Python with Pillow (`pip install pillow`).

| File | What it does |
|---|---|
| `games.cjs` | The fictional library: titles, versions, tags, descriptions, dialogue lines. |
| `art.cjs` | Generates SVG covers (600×900) and in-game shots (1280×720) into `out/media/covers`. |
| `build-ui.cjs` | Copies `frontend/dist` into `out/ui` with a Wails mock: the library data plus shot scenarios (`?shot=home\|detail\|discreet\|bulk\|settings\|find\|stats\|launch\|editor\|smart`). |
| `banner.html` | The README banner / social preview (1280×640). |
| `make.ps1` | Runs everything: art → UI → headless Edge screenshots → WebP in `docs/images`. |
| `capture-readme.cjs` | CDP capture with scenario, asset, runtime-error and external-request checks. |
| `convert.py` | PNG → WebP, banner → `social-preview.png`. |

README automation uses `&readme=1`; regression fixtures keep their normal initial state without it.
The ten shots are home, detail, discreet, bulk, settings, find, stats, launch, editor and smart.
Detail/editor use 1440×1500 to include notes/sessions and checkbox discovery; find/stats/launch use
1440×1100, the rest 1440×900. The banner remains 1280×640. The smart shot pauses a fictional response
at 1 of 3; it never sends a request to a real source. Bulk selection opens View options first.

`make.ps1` binds the local server to loopback, uses a dedicated Edge profile, rejects occupied ports,
and fails if a scenario, asset or browser check fails. CDP waits for readiness and fonts; it does not
rely on a fixed virtual-time screenshot. Conversion touches only shots requested in that invocation.
The helper verifies new states before capture; visual inspection is still required before committing.

A new scenario is a branch in the `load` handler at the end of the mock in `build-ui.cjs`.
`out/` is temporary and git-ignored.

## Find-by-name regression

Build the frontend, then run `node docs/tools/screenshots/art.cjs` and
`node docs/tools/screenshots/build-ui.cjs` from the repository root. Serve the mock with:

```powershell
python -m http.server 5393 --bind 127.0.0.1 --directory docs/tools/screenshots/out/ui
```

Start a dedicated headless Edge with `--headless=new --inprivate --disable-extensions --disable-sync
--no-first-run --remote-debugging-port=9334 --window-size=1440,1000` and a separate `--user-data-dir`
inside `docs/tools/screenshots/out/`. Use `Start-Process -WindowStyle Hidden` on Windows. Keep ordinary
browser profiles separate, then run:

```powershell
node docs/tools/screenshots/test-find.cjs
```

The script opens `?shot=find`, clicks the full search/preview/save flow, checks partial failures, empty
results, read-only preview, blur, language changes, persistence and lock-time request invalidation.
It writes `out/find-checks.json` and `out/find-*.png`. Stop only the dedicated server/browser afterward.
The mock exposes `window.__findMock` controls (failedSource, delays, empty, previewError, saveError, calls)
and `window.__mockEmit` for events. Add `&keep=1` to preserve local preferences across reloads.


## Sessions and Statistics regression

Use the same build/server/dedicated Edge setup, then run:

```powershell
node docs/tools/screenshots/test-stats.cjs
```

The script opens `?shot=stats`. `stats-fixture.cjs`, serialized by `build-ui.cjs`, freezes the clock at
2026-11-06 12:00 UTC and supplies GetGameSessions, GetLibraryStats and GetSessionRevision. It includes
boundary-crossing sessions, old playtime without history, an empty/untracked game, interrupted history,
manual/dynamic/pinned hidden collections with mixed membership, and unknown/missing/measured-zero sizes.
The 48 assertions cover totals, charts, navigation/scroll, privacy and late replies, error/retry/empty states,
calendar/zone changes, language changes, keyboard access, editor omission and completed-session refresh.
Outputs: `out/stats-checks.json` and `out/stats-*.png`. Preview omission is checked by `test-find.cjs`.

Both scripts accept alternate ports when the defaults are occupied:

```powershell
$env:UI_PORT = '5394'
$env:CDP_PORT = '9335'
node docs/tools/screenshots/test-stats.cjs
node docs/tools/screenshots/test-find.cjs
```

Start the server and dedicated Edge with matching ports; never reuse a normal browser profile.
`window.__statsMock` exposes delay, error, sessionError, emptyActivity and calls, plus complete(gameID),
advanceDay() and emptyLibrary(). `window.__mockEmit(name, ...args)` simulates revision/privacy/error events.
Only fictional local assets are used. These checks do not open real games or a real library.

## Metadata header layout

Run `node docs/tools/screenshots/test-metadata-layout.cjs` against the same mock server
and dedicated Edge CDP instance (`UI_PORT` / `CDP_PORT` overrides are supported).
It checks compact and prominent panels in English and Russian at 1440, 1280, 1024
and 900 pixels (the native minimum window width): single-line headings, alignment,
clipping and both tab actions. Screenshots are `out/find-header-*.png`; results are
`out/metadata-layout-checks.json`. Responsive sizing follows the card's container
width because GameDetail changes between one and two columns as the window resizes.

## Game-page polish regression

Run `node docs/tools/screenshots/test-game-page.cjs` against the same dedicated server/Edge setup
(`UI_PORT` / `CDP_PORT` supported). It checks originating lists, sort order, neighbour boundaries,
Alt/mouse history, search restoration, sidebar/shelf order, hidden membership, localized dates,
note saves, update acceptance and the shared F1/settings reference. Editor Ctrl+S/Esc are exercised;
preview Esc/Backspace are covered by test-find.cjs.

Outputs: `out/game-page-checks.json` and `out/game-page-*.png`. `game-page-mockup-width.png` is a full
1016px window; `game-page-main.png` is the 1016px main-area crop for comparison with
`docs/design/game-page.png`. The new labels/controls follow the board. Existing differences remain:
the app sidebar, larger typography/spacing and metadata panel; a 1016px full window stacks the columns.
These are mock-library checks, not real-game acceptance. This does not regenerate README images.

## Launch-file interface regression

Use the same fictional UI server and dedicated Edge CDP instance (`UI_PORT` / `CDP_PORT`), then run:

```powershell
node docs/tools/screenshots/test-launch-files.cjs
python docs/tools/screenshots/compare-launch-files.py
```

The 69 checks exercise the combined menu, Details links, primary/extra routing, draft labels/swaps/removal,
native-picker cancellation (mocked), checkbox discovery, capacity/merge overflow, save/discard, EN/RU,
missing/error/partial states, keyboard focus and late replies after cancel/history/lock/panic. The 900px
layout is checked for horizontal overflow in view and edit mode. Existing find/stats/game-page scripts
must pass as well; find explicitly verifies launch controls are absent in read-only preview.

`?shot=launch` adds missing, empty-primary, capacity and overflow games. `window.__launchFilesMock` controls
`delay`, `error`, `empty`, `incomplete`, `candidates`, `selectedPath`, `selectDelay`, `selectError`,
`availabilityDelay`, `availabilityError`, `missing`, `lateReply` and `calls`. `lateReply` deliberately allows
stale mock responses after lock so frontend guards are exercised independently of backend privacy checks.

Outputs stay in git-ignored `out/`: launch-files-checks.json, launch-files-right.png, menu/editor/narrow
screenshots and launch-files-comparison.png. The comparison retains the reference cards' relative sizes,
insets and gaps while scaling the reference column from 266px to the app capture's 340px width; no app CSS
is overridden. See the feature spec for the remaining differences. It needs Pillow, like convert.py.

## Smart-list regression

Run `node docs/tools/screenshots/test-smart-lists.cjs` with the same UI_PORT/CDP_PORT setup.
`?shot=smart` contains an empty primary, a missing nonempty primary, a missing folder, one confident
name match, one manual match, one site failure and a hidden game that must never enter the batch.
`window.__smartMock` exposes delay/saveDelay, search timestamps, applied IDs, detection IDs and fixture games.
The scenario checks visible scope, found N of M, persisted source preferences, sequential pacing,
progress after navigation, cached manual results, site errors, stop during search/apply, lock with fast
unlock, panic, immediate restart, no selected sites, empty list removal and EN/RU/narrow layout.
Outputs: `out/smart-lists-checks.json` and `out/smart-lists-*.png`. Keep all other test-*.cjs passing.
These mocks never contact a search site, open a native dialog or execute a launch file.

All CDP regression scripts emulate an active, focused tab so headless Edge does not suspend menu-focus
animation frames when Windows treats the hidden test browser as a background window.
