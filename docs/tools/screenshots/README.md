# README screenshots

Regenerates `docs/images/*.webp` (and `social-preview.png`) from the real UI with a fictional library and
generated SFW artwork, so screenshots can be refreshed after UI changes.

```powershell
cd game-launcher; .\build.ps1 -NoTests        # the frontend must be built (frontend/dist)
cd ..\docs\tools\screenshots
.\make.ps1                                    # all shots; or .\make.ps1 -Shots home,detail
```

Requirements: Windows with Microsoft Edge, Node.js, Python with Pillow (`pip install pillow`).

| File | What it does |
|---|---|
| `games.cjs` | The fictional library: titles, versions, tags, descriptions, dialogue lines. |
| `art.cjs` | Generates SVG covers (600×900) and in-game shots (1280×720) into `out/media/covers`. |
| `build-ui.cjs` | Copies `frontend/dist` into `out/ui` with a Wails mock: the library data plus shot scenarios (`?shot=home\|detail\|discreet\|bulk\|menu\|settings`). |
| `banner.html` | The README banner / social preview (1280×640). |
| `make.ps1` | Runs everything: art → UI → headless Edge screenshots → WebP in `docs/images`. |
| `convert.py` | PNG → WebP, banner → `social-preview.png`. |

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
