# Spec: select a game from a Pornolab collection

Status: **step 1 — backend implemented; Windows checks passed; UI pending** (2026-11-09).
The owner approved the behaviour and Decisions below. Step 1 includes the backend, fixtures,
localization keys, generated bindings and mock method stubs. The chooser UI remains a separate step.

Design reference: [Find game info results](../design/find-by-name-results.png), interpreted using
[the design README](../design/README.md). Reuse the flow in [find-by-name.md](find-by-name.md) and the
batch conventions in [smart-lists.md](smart-lists.md).

## Why

A Pornolab release can contain several games in its first post. The current parser reads one release
title, one description and one gallery for the entire post, so it cannot reliably fill a library entry
for just one of those games. Let the user choose a game, preview its metadata and save a source that
continues to refer to that game when checking updates.

Reference release: `https://pornolab.net/forum/viewtopic.php?t=3006469`.
Use the saved source `C:/Users/user/Downloads/pornolab-multi-3006469.html`, decoded as Windows-1251.
Do not fetch the live page or add a Cloudflare/login workaround.

## Agreed scope

- Detect two or more game sections in the **first** `div.post-user-message`. A game section owns an
  `Описание` field. Separate sections at `span.post-hr`, and support one game per enclosing spoiler.
  General information and notes without their own description field are not games.
- A single-game post keeps the existing parser behaviour, including release-title extraction and
  image fallbacks. The collection-specific section heading is a deliberate exception to the normal
  release-title rule in AGENTS.md, not a replacement for that rule on ordinary posts.
- Pasting an unselected collection link shows its games; it does not apply metadata immediately.
  Folder-name confidence is a suggestion only. The user always chooses, previews and saves.
- Remember the selected section in the source URL, for example `viewtopic.php?t=3006469#game=2`.
  Retain the selected title so later parsing can recover from a changed section order.
- Updates for a selected source use that section's version. Update checks for legacy sources without
  a selection retain their existing behaviour.
- Bulk metadata filling never guesses a section. An unselected collection is skipped and counted as
  **needs manual selection**, including when the folder name has a confident match.
- Fetch the release page once for an uninterrupted selection/preview/save flow. Reuse an in-memory
  snapshot; selecting another row or going back from preview must not fetch the page again.

Here “collection” means a multi-game release, not a pLauncher library collection. This feature fills
one existing library entry; it does not create entries or library collections for every game in a post.

## User flow

### Step 1: submit a link

Both prominent and compact `MetadataPanel` layouts use the same routing. A Pornolab URL submitted in
the name field follows the existing handoff to **Paste a link**, including Enter and bare-domain URLs.
Show the usual loading/error states while inspecting the page.

- Ordinary single-game page: continue the current link flow unchanged.
- Collection without `#game=N`: show a selection list in the metadata panel, with the pasted link
  retained. No library fields, source versions or local images are written at this stage.
- Source with a valid saved selection: resolve that game; do not silently fall back to another game.
  An interactive collection import uses the same preview/save flow below.
- A missing or ambiguous saved game shows the localized error and offers the list to select again.
  Explicit reselection may discard the old selection; a background action may not do so.

### Step 2: choose a game

Use the separate bordered, rounded result rows from the design board, with gaps between rows:

- A cover-cropped **16:9 thumbnail**, approximately 136 × 76 at the reference width. Missing artwork
  uses the existing placeholder. All remote thumbnails obey `blurCls`, hover blur and discreet mode.
- A semibold section title, then `version · year · developer`, omitting empty parts and separators.
  The year is the release year, not an update date. Do not add a new persisted Game year field solely
  for this row.
- **Use this** / **Использовать** at the right; keep the small open-page icon from the shared results
  pattern. Opening the URL in a browser opens the ordinary release page with the fragment intact.
- A confident folder match gets the green reason line **Looks like your folder** /
  **Похоже на вашу папку** and a filled button. Other buttons are outlined. No separate “Most likely”
  badge, automatic selection, automatic preview or automatic save.

Use `folderSearchName` / `SuggestSearchName` and the existing `rankSearchHits` scoring and `Best`
threshold (`>= 0.8`, lead `>= 0.15`). Keep confidence in Go rather than duplicating it in JavaScript.
Rank rows using that helper, but assign section identities **before** ranking. `#game=2` always means
the second qualifying game in document order, never the second displayed result. Display every
detected game; do not inherit name search's eight-result cap. Ties have no highlighted winner.

### Step 3: preview, then save

**Use this** parses the selected section from the same page snapshot and opens the existing read-only
preview: media, description, tags, author, engine, version and languages, with **Other results** and
**Save to library**. Remote images remain remote until save and honor privacy settings everywhere,
including the lightbox. Esc/mouse back return to the selection list, preserving its rows and link.
Keep the existing modal/lightbox key precedence and `e.defaultPrevented` contract.

Save uses the existing metadata merge: nonempty values overwrite, tags merge, the source is upserted,
the primary source is set only if absent, and accepted update flags reset. Download only the selected
game's media. Save its selected URL, source title identity and accepted source version together.
An existing folder-version fallback must never become `GameSource.LastVersion`.

Failure leaves the current stage usable. Navigation to another game, lock or panic invalidates pending
results; late responses must neither reveal metadata nor save it. Cancellation and removal of a library
entry retain the existing safeguards against resurrecting it or overwriting intervening edits.

## Parser contract

### Section discovery

Decode Windows-1251 before DOM processing. Limit discovery to the first post; replies and quoted
release descriptions must not add games. Detect the `Описание` **field label**, not an occurrence of
that word in prose. Count sections owning that field, even if its value is empty.

Split at `post-hr` boundaries while preserving DOM structure. For spoiler-per-game layouts, inspect
each game wrapper's body and its own fields. A wrapper containing several child games must not also
be returned as a game. Screenshot, installation and changelog spoilers are not additional games.
Nested decorative separators must not make screenshots or notes into candidates. Do not rely on the
`[Сборник]` title prefix alone; the actual sections establish whether selection is needed.

The reference file contains two sections, each with a 24px heading, a right-aligned cover and labelled
fields. Its source HTML has `.sp-body[title="Скриншоты/Примеры"]` and `.sp-title`; `.sp-head` is created
by site JavaScript. Recognize both source-HTML and rendered `.sp-head` forms without running scripts.

Zero or one qualifying section, with no explicit selection, follows the existing single-game parser.
An explicit selection must still be resolved and validated if the post now contains only one game;
it must not drop into the legacy path merely because another game was removed.

### Selected metadata

Read values only within the selected section. Stop field values at the next field or section boundary;
do not flatten the entire post and accidentally consume the next game's fields.

- **Title:** the section heading (the reference uses a span with `font-size: 24px`, with either CSS
  spacing form), cleaned as text. Never use the combined `[Сборник]` release title as a selected game's
  name. Missing/duplicate headings use localized `Game {n}`, version/year and, when useful, the start
  of the description. This display label is not the saved source identity.
- **Version:** section `Версия`; if the field is absent, use `versionFromTitle` on the shared release title.
- **Developer:** section `Разработчик/Издатель` and existing developer-label variants; when absent,
  use the release-title developer fallback. Do not read another section's developer via a whole-post helper.
- **Genres/tags:** section `Жанр`, normalized through existing tag conventions; when absent, use
  `parseReleaseTags` on the shared title. Do not unconditionally merge the release's combined genres
  into a section that supplies its own genre field. Derive engine with existing helpers from the
  selected metadata, without scanning neighbouring sections.
- **Languages:** selected language fields, including story/interface/voice, normalized and deduplicated
  using existing language names. Fall back to `parseReleaseLanguages` only when section language fields
  are absent, not by searching all text in the post.
- **Description:** prose belonging to the section's `Описание` field, excluding subsequent labelled
  notes, screenshots and other sections; preserve existing cleaning and length limits. A missing field
  disqualifies a section. An empty field leaves the description empty: the shared release title has no
  description to supply, so no synthetic description is invented.
- **Year:** section `Год выпуска` for the chooser; a missing value may use an unambiguous shared release
  year. Do not assign a range such as `2017-2019` to every game as a specific year.

Fallback is per missing field, never a fallback to the first game's metadata. Keep absent fields
distinct from present empty fields while extracting; the final save still follows existing merge rules.

### Media

Cover = the selected section's poster outside its nested spoilers (`var.postImg.img-right` in the
reference). For a game enclosed in its own spoiler, “outside” is relative to that game body: the
outer game wrapper does not disqualify its cover. Screenshots = images in that game's screenshot
spoiler(s), in document order, deduplicated and capped at **15** after excluding the cover.

Reuse `normalizePostImg`: real image URLs come from `title`, not saved local `src`; retain existing
Fastpic/imgbox normalization and decorative-image filtering. Do not include another game's poster,
general banners, installation images or images from replies. Do not reuse the ordinary parser's
whole-post image fallback for a selected section. If no cover exists, promote the first screenshot
from that same section and exclude it from the remaining screenshots.

## Source identity and updates

`game` is a positive, one-based index over qualifying sections in document order. Parse the URL with
URL helpers; omit the fragment from the HTTP page request, but preserve it through source editing,
preview, apply, backup/restore and opening the source. Invalid selection syntax must not be silently
treated as a legacy unselected URL. Existing non-selection anchors retain ordinary URL behaviour.

The saved section title is independent of the editable library `Game.Title`: optional
`GameSource.SelectedTitle` / `selected_title` alongside the selected URL and `last_version`.
Old source JSON remains readable. Preserve the field across restart, backup/import and relink.
An untitled section can be selected initially but cannot safely resolve a later saved-source parse.

Resolution on a later fetch:

1. Check the indexed section against the saved title using conservative normalized equality.
2. If it differs or no longer exists, search all game sections for that saved title.
3. A unique match resolves the selection. Carry its current index into the next successful source save.
   Do not change the accepted version merely because the index moved.
4. No match, or an ambiguous title match, requires explicit reselection. Never substitute a fuzzy
   folder match or whichever game now occupies the old index.

A newly pasted `#game=N` without saved title metadata can identify the initial section by index;
preview/save captures its title for subsequent use. The source editor must clear stale title identity
when the user intentionally replaces its topic/selection, and preserve it for an unchanged source.

Use `apperr` for a missing selected game, with a message equivalent to **Game not found in this release;
choose it again** / **Игра не найдена в раздаче, выберите заново**. Code:
`parse.collection_game_missing`. The distinct machine-readable selection-required outcome
`parse.collection_choice_required` lets batch callers distinguish manual work from network failures.
All new errors and UI text must have keys in all 11 locales.

`CheckSourceUpdate`, `CheckGameUpdates` and `CheckAllUpdates` resolve selected sources and compare only
the selected section version against that source's `LastVersion`, using existing `normVersion` rules.
They never persist preview image URLs or other parsed metadata. A missing selected game leaves accepted
metadata/version intact and reports the localized failure through the calling flow.

For a legacy source **without** a selection, update checking keeps the current whole-release version
behaviour. This compatibility exception does not authorize legacy whole-post parsing in bulk metadata
filling: `bulkReparse` and smart-list application must stop before writing and classify the game as
manual selection. This distinction must be explicit at the call boundary, not inferred from `saveDir`.

## Backend integration and page reuse

Separate fetching/decoding from discovery and section extraction in `internal/parser/pornolab.go`.
Keep a single fetched document or immutable extracted snapshot available to the whole interaction.
The link-resolution response needs either the ordinary-page result or collection candidates plus an
opaque snapshot handle. Candidate data can reuse `SearchHit` fields with the original section index
and selected URL; the handle must also carry the selected title through preview and save.

Integrate this contract with `PreviewGameInfo`, `ApplyGameInfo`, `UpdateGameMetadata` and the shared
metadata application code. Choose exact App method signatures during implementation; ordinary link
submissions must also consume the inspected snapshot rather than triggering a second fetch.
Do not change existing name search ranking or its result cap as a side effect.

Cache requirements:

- Shared across the separate App calls for resolve/list, preview and save; a cache on a temporary
  parser instance alone is insufficient. Strip selection fragments from the page fetch key.
- At most **5 pages**, evicting the least recently used; expire after **10 minutes of idle time**.
  Share immutable data or clone before destructive DOM extraction.
  Different selections must never contaminate each other's descriptions or galleries.
- During a valid interaction, list → preview → back → another preview → save uses **one page GET**.
  Media requests are separate; only save downloads media into the data folder.
- After expiry/eviction, fetch silently without an extra user step. Show the chooser again only when
  the set of games changes; version/media changes alone do not require reselection. Resolve reordered
  games by unique title; ambiguous identity still requires explicit choice.
  A fresh explicit submission/update-check action can fetch fresh content.
- Clear private snapshots on lock/panic and ignore late completions from the old interaction.
  Tolerate an unconfigured or locked app without network/disk work. Keep existing cancellation rules.

Batch summaries distinguish applied, manual selection and actual errors, retaining the affected game
IDs so the user can open them and choose. Never show a chooser for every item during a batch. Preserve
visible-library scoping, Stop, privacy cancellation and existing smart-list result handoff semantics.

## Fixtures and verification

Implementation must add sanitized fixtures under `game-launcher/internal/parser/testdata/`:

- `pornolab-collection-3006469.html`: reduced from the supplied saved page, retaining Windows-1251,
  the shared title, first-post structure, separators, headings, field labels and raw spoiler markup.
  Replace descriptions with neutral text and strip unrelated page chrome, scripts and personal data.
- `pornolab-collection-three.html`: a derived three-game variant with distinct metadata/media plus
  general-information and notes sections without descriptions; none of those auxiliary sections count.
- `pornolab-collection-spoilers.html`: one game per spoiler, with nested screenshot spoilers and covers
  inside each game wrapper but outside its nested spoilers; no duplicated parent/child candidates.
- `pornolab-single.html`: one game from the same markup, with an ordinary release title, for legacy regression.
- `pornolab-collection-ambiguous.html`: one untitled section and two repeated headings, with distinct excerpts.
- `pornolab-collection-no-cover.html`: a missing poster, requiring promotion of that section's first screenshot.

Use neutral placeholder image URLs and local HTTP test responses; never commit or fetch real artwork
for tests. Preserve relevant `title`/`src` differences and hosting-link structure for normalization
coverage. Record fixture provenance, sanitization and encoding in the testdata README.

Tests must cover section counts and order, scoped fields and missing-field fallbacks, description
boundaries, screenshot ownership/deduplication/15-image cap, raw and rendered spoiler labels, repeated
preview without DOM mutation, ties and confident folder suggestions, and more than eight games.
Add source round-trip tests for reorder recovery, missing/duplicate titles, malformed/out-of-range
indices, initial index-only links, edited library titles, backup/restore and source editing.

Use `httptest` request counters to prove one page fetch across list/preview/save and independent fetches
for fresh actions. Cover cache expiry, lock/panic, late responses and cancellation. Verify selected
updates, unchanged legacy update behaviour, and manual batch classification with no metadata writes.

The old `pornolab1-6` samples no longer exist. Use `pornolab-single.html`, derived from the same markup,
as the committed single-game regression. Compare title, version, author, engine, languages, tags,
description, cover and screenshots. Also run against any new single-game `pornolab*.html` files appearing
in Downloads, using metadata-only parsing without network access. The optional local test reports a skip
when none exist; CI relies on the committed fixture.

## Done when

- All parser, source/update, cache and batch tests above pass, including the committed single-game
  regression fixture and any additional available saved samples.
- New bound methods/models have generated Wails bindings and mocks in `docs/tools/screenshots/build-ui.cjs`.
- Headless Edge/CDP checks exercise both panel sizes: link → choices → preview → back → choose → save,
  an unconfident list, privacy blur, errors, removed/reordered games, batch manual results and cancellation.
- Compare mock UI screenshots at the reference width against the results board and report remaining
  differences. Check EN/RU and runtime locale changes; use fictional media and no real library window.
- `go test ./...`, `go vet ./...` and `game-launcher/build.ps1` pass. Update AGENTS.md, CHANGELOG
  `[Unreleased]` and this spec's status with actual delivery and verification, not planned claims.

## Out of scope

Searching Pornolab by name, authentication/Cloudflare workarounds, auto-importing every game in a release,
new library collection types, multi-game support for other sites, and automatically converting legacy
sources without user selection. The chooser interface and clickable manual-work summary are outside step 1.

## Decisions

Confirmed by the owner on 2026-11-09:

1. Persist `GameSource.selected_title` and `#game=N`; keep identity across restart, export/import and relink.
2. Missing or repeated headings use localized **Game {n}** / **Игра {n}**, version/year and a description
   excerpt if needed. On a later ambiguous parse, request reselection instead of guessing.
3. With no cover, take the first screenshot of that same section. Never borrow another game's images.
4. Cache at most five pages for ten minutes of idle time. On expiry fetch silently; show the list again
   only when the game set changes. Ambiguous identity still requires a new choice.
5. Manual work uses the existing smart-list handoff: **Manual choice needed: {n}** /
   **Нужен ручной выбор: {n}**, clickable to open the affected games. Step 1 supplies the backend payload;
   wiring the click and rendering the list belong to the subsequent UI step.

## Backend delivery (2026-11-09)

Implemented the section parser and preserved the ordinary parser as a separate legacy path. Titles,
fields and media are scoped to the first-post section; source HTML spoiler names work without scripts.
Candidate identity uses document order before ranking, with no eight-result cap. Missing/duplicate
headings expose `fallback_title`, `index` and a neutralizable description excerpt for localized UI labels.
Selected source title is stored in existing source JSON, without a new SQLite column. Missing/ambiguous
saved identities fail explicitly; backup/import, editor saves and relink preserve the field as specified.

New bound methods and their mock stubs:

- `ResolvePornolabCollection(gameID, url, refresh)` returns `{collection, candidates, snapshot}`. Use
  `refresh=true` for a new submission and `false` to reopen the already fetched changed-game list.
- `PreviewPornolabGame(gameID, url, snapshot, index)` returns read-only Game metadata with remote media.
- `ApplyPornolabGame(gameID, url, snapshot, index)` applies that choice using the existing merge rules.
- `ReparseGameInfo(token, gameID)` uses Begin/StopSmartSearch and returns `{game_id, applied, manual}`.
- `GetManualGameChoices()` returns game IDs, source URLs and candidate snapshots for the existing
  smart-list handoff. The frontend must use this payload and the manual count in its later UI delivery.

Treat snapshot as opaque. It contains a privacy generation, section identities and a metadata digest;
no page DOM is retained by a snapshot. The cache holds up to five raw pages (4 MiB maximum each), with
ten-minute idle expiry, fresh DOMs per parse and shared in-flight requests. A valid list/preview/save
interaction makes one page request. Expiry/eviction fetches silently. Changed membership returns
`parse.collection_changed`; reordered unique titles resolve automatically. Ambiguous changed data
returns `parse.collection_game_missing`. Update checks force a fresh page and refresh the cache, so
accepting a newly detected version cannot accidentally use pre-check metadata.

Metadata application, smart-list lookup and bulk reparse refuse unselected collections before saving.
The legacy UpdateGameMetadata caller receives the coded choice-required error; the new batch method
classifies it as manual and records the handoff. ApplySmartGameInfo returns false for manual cases.
The current bulk UI still needs to switch to the new batch contract in step 2. No Svelte component,
frontend action handler, layout or interaction was changed in this backend step.

Validation on Windows: `go vet ./...`, `go test ./...` and `build.ps1`; Wails bindings regenerated.
Tests cover all six committed Windows-1251 fixture variants, metadata/media boundaries, legacy
single-game equality, missing-field fallbacks, ranking/ambiguity, more than eight games, source
resolution and round trips, selected/legacy update checks and update acceptance, cache request counts,
idle renewal/expiry/LRU/refresh, privacy invalidation, late replies, batch Stop and manual classification.
The original saved topic 3006469 also passed local extraction checks without network requests.
No additional single-game `pornolab*.html` files were present in Downloads; that optional test skipped.
Mock script syntax was checked with Node. Build warnings remain in unchanged Svelte components and
for Browserslist data age and bundle size.

Not checked: a real library, native Wails interaction/global panic hotkey, live Pornolab/Cloudflare,
Linux/macOS execution or visual UI acceptance. The chooser and clickable manual-work summary remain
unimplemented by design for step 2; their headless UI checks and mockup comparison are still pending.
