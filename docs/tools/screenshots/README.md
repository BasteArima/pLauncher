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
