# Smart lists: delivery notes

Status: **implemented** (2026-11-07).

## Decisions and implementation

- Sidebar entries appear only when nonempty and use `visibleGames`, including the action scope.
  Normal list sorting, history, game position and neighbouring-game navigation are retained.
- **No launch file** means an empty primary path or a primary target absent on disk, excluding
  missing game folders. `Game.LaunchMissing` / `launch_missing` is computed by Go on reads, like
  `FolderMissing`; it is not stored or checked from JavaScript. Permission/I/O errors alone do not
  prove absence. Existing directories remain valid for native `.app` bundles.
- Its banner calls `DetectLaunchFiles` with the visible list's IDs and reports found N of M.
  Only empty primary paths are filled. Missing nonempty paths remain for repair in the editor.
- **No source link** means an empty `sources` list. Its banner searches one game at a time using
  `SuggestSearchName`, `SearchGameInfo` and the saved name-search source selection, with a one-second
  pause between games. Leaving the list does not stop it; progress and Stop reappear on return.
- Confidence belongs in Go (`parser.ConfidentSearchURL`) to reuse the search ranker's normalization
  and test it alongside ranking. The highest-scoring result must have `Best` and an exactly matching
  normalized title. Conflicting marked winners, known differing authors or distinct marked records
  on one site require manual choice. Existing equal-score cross-site ties remain manual (`Best=false`).
- `ApplySmartGameInfo` uses the same metadata application as `ApplyGameInfo`, with a run token and
  cancellable parse. Stop, lock and panic invalidate the token; the final DB write is serialized with
  cancellation. Downloads use an isolated media directory, removed if application is cancelled.
  Old-run cleanup cannot cancel a new run. An intervening manual source addition is preserved.
- Results for manual choice and site failures are cached in memory for the normal name-search panel.
  Privacy transitions clear them. Hiding/removing a queued game also stops the batch; ordinary
  navigation does not. Late replies are discarded. No sources selected means no requests.
- Summary counts are per completed game: applied, manual choice, errors. A successful application
  counts as applied even if another site failed; otherwise any site/apply error counts as an error.
  Unprocessed games after Stop are not classified. Strings are translated in all eleven locales.

## Verification and limits

All six headless scripts passed: 293 assertions (43 smart lists, 42 find, 51 statistics,
39 game page, 69 launch files, 49 metadata layout). Existing Vite accessibility/bundle warnings remain.

- Windows: `go vet ./...`, `go test ./...`, `build.ps1`; Wails bindings regenerated.
  Go tests cover disk changes/read-only flags, detection without replacement, confidence and ambiguity,
  cancellation during parse, fast unlock, new-run isolation, media cleanup and preservation of edits.
- Fictional mock UI in headless Edge: `test-smart-lists.cjs` covers success/manual/site error, saved
  sources, pacing, navigation, stop during search/apply, lock/panic, hidden games and empty entries.
  Existing `test-find`, `test-stats`, `test-game-page`, `test-launch-files` and `test-metadata-layout`
  are regression checks. EN/RU and 900px layout captures remain in ignored `out/`; README images unchanged.
- Not tested: real-library acceptance, live search services/rate limiting, the native Wails window or
  global panic hotkey, disconnected/network drives, Linux/macOS execution. OS-specific existing tests
  ran only where supported on Windows. No real game or utility was launched.
