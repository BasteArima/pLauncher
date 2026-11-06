# Design mockups

The owner's mockups live on a private claude.ai canvas that agents can't open, so the boards that matter for
current work are exported here as images. **When a mockup exists for a screen, it is the spec for the layout:**
match its structure, sizes, density and wording; the text spec only adds behaviour. Before finishing UI work,
take a screenshot of the mock UI (`docs/tools/screenshots`) at the same width and compare it with the mockup
side by side; list any remaining differences in the report.

The mockups use placeholder art and slightly different paddings than the app; the flat theme, fonts and colours
of the app (`tailwind.config.js`) win over the mockup's exact hex values.

| File | Board | What it defines |
|---|---|---|
| `find-by-name-results.png` | A3 · Game with folder only | The «Find game info» block with results (see below). |
| `game-page.png` | A2 · Game page | The game page, including the compact «Play sessions» card (see below). |

## Find game info (`find-by-name-results.png`)

- One card: title «Find game info» left, **segmented** tabs «Search by name | Paste a link» right in the same row
  (pill segmented control, not underlined tabs).
- A large input with the name and a filled «Search» button on its right.
- One line: «Search on» + source chips (F95zone, Steam, VNDB; selected = filled, no check marks) + the muted note
  «Pornolab, Erotorrent, Island: paste a link instead» **on the same line**.
- One muted line: «Name taken from the folder, version 0.3 stripped. 3 matches».
- Results are **separate bordered rounded rows** with a gap between them, each:
  - a **16:9 thumbnail** (~136×76, rounded, cover-cropped; blurred like other covers);
  - title (semibold), then meta `Site · version · author · engine · updated <date>` (skip empty parts);
  - an optional **reason line**: green for a strong signal («Version 0.3 matches your folder»), muted for a
    caution («Older free demo of the same game» style is the mockup's example; derive real reasons from data:
    version match, exact title match, «Different title» when the score is low);
  - the button on the right: the best match gets a **filled white «Use this»**, the others an outlined one.
    The «open page» action is a small icon button next to it (keep it, it is useful).
- No separate «Most likely» label above the title: the filled button + the green reason line carry that.
- In the compact panel (games that already have info) the same block appears under the tabs, without the big
  title.

## Play sessions (`game-page.png`)

- A **compact card in the right column, under Details**, not a full-width section in the left column.
- Header row: «PLAY SESSIONS» (small caps like «DETAILS») left, «last 8 weeks» muted right.
- 8 small bars (~64 px tall), no axis labels, no helper text; a bar shows its dates and duration on hover/focus
  (keep the existing accessible tooltip and keyboard support).
- One footer line: «23 sessions · longest 2h 05m».
- **No sessions yet**: the same card with flat bars and one muted line «No sessions yet: they appear after you
  play for a minute.» Nothing else: no big zero numbers, no paragraphs about legacy playtime.
- Games whose launch file can't be tracked (opened via the OS): one muted line saying so, no chart.
