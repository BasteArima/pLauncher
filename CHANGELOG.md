# Changelog

Notable changes to pLauncher. Dates are UTC.
The text of each GitHub release comes from `docs/releases/vX.Y.Z.md`.

## [Unreleased]

### Added
- Multiple launch files: primary labels and up to eight additional files, bounded candidate discovery,
  availability checks and primary-only playtime accounting. An always-visible menu beside Play combines
  launch files and game actions; Details opens individual files. The editor supports primary swaps, labels,
  removal, Add and checkbox discovery with save/discard. Additional tools/documents do not affect sessions
  or recent play. Relink/backup preserve paths, labels and merge overflow. All eleven languages are supported.
- Game-page navigation shows the originating list and position, with previous/next arrows that preserve
  its order and participate in back/forward history. Search text and sort are restored with history.
- Update notices show when the version was discovered; repeated checks retain that date. Personal notes
  show their last text-edit date. Both dates persist in the library and use localized formatting.
- Play-session recording survives launcher closure, keeps 30-second recovery checkpoints and preserves
  existing total playtime. Sessions follow game deletion, relink merges and backups. Added privacy-aware
  statistics queries. Game pages now show eight weeks of activity, session count, longest session and the
  latest five launches, including interrupted recovery markers.
- Statistics in the sidebar: all-time/month/week hours, 52 weeks of activity, top games by period, statuses,
  engines, cached disk usage and library-information counts. Hidden collections are excluded until revealed;
  period and scroll position survive opening a game and returning. All 11 languages are supported.
- Find game info by name across F95zone, Steam and VNDB: folder-name suggestions, source toggles, ranked
  results and a read-only preview before saving. Search and paste-link tabs share the metadata panel;
  dismissing the large prompt keeps the compact panel available. Preview images respect privacy blur.
  Results also let you open the source page directly in your browser.
  Added VNDB URL parsing, remote-image metadata previews and localized errors.
- Play status for every game (playing, want to play, finished, dropped), a 5-star rating and personal notes, where
  `||text||` stays hidden as a spoiler until clicked. Status chips above the grid filter by status.
- Filter by several tags at once: all of them or any of them, and exclude tags you don't want to see.
- Edit mode looks like the game page itself. Unsaved changes are marked, Esc cancels and Ctrl+S saves. Screenshots
  can be reordered by dragging, any screenshot can become the cover, and images dropped onto the window are added
  as screenshots.
- Covers can be zoomed and framed separately for wide and tall cards, by dragging and scrolling over a live preview
  or with sliders.
- Smart lists (updates, no cover, folder not found) with one button that handles the whole list.
- Back and forward like in a browser: mouse side buttons or Alt+←/→. A «Back to top» button in long lists.
- «Remove from Continue playing» in the game menu. Finished and dropped games no longer show there.

### Changed
- F1 and Settings now document history navigation, mouse side buttons, editor save/cancel and returning
  from a found-game preview. Added translations in all 11 languages.
- A simpler lavender p on graphite is now the application icon, matching the flat redesign.
- New look: flat dark theme, Inter font, line icons instead of emoji, wide cards by default (tall ones in view
  options). Hovering a card shows Play, favorite and menu buttons.
- Title bar with search on the right. The sidebar starts with «Scan folders», then Collections with «All games» and
  «Favorites» at the top; clicking a collection opens it as a grid.
- The library opens as a grid with chips and a «Tags» menu. Shelves, card shape and size are in the view options
  popover, and the big hero banner is off by default.
- The game page has a media viewer with the cover first, a play card with stats, status and rating, update and site
  links, and collections in the details. Games without info show a «Find game info» block.
- Context menus open where you click and flip near window edges.

### Fixed
- Scrolling a large library is smooth again (blur effects no longer run on every card).

## [1.1.0] - 2026-11-05

### Added
- Launcher updates are now seamless: a small notice in the title bar opens a window with what's new, the download
  progress and a restart. You can tick "Restart automatically", or install on next launch. pLauncher waits for a
  running game to close, keeps covers hidden if they were, and goes back to the previous version if the new one
  doesn't start. Your database is copied to `backups/` before each update.
- Updates are checked at startup and every 6 hours. You can turn this off in Settings → General.
- An empty library now shows a big **+** button to add your games folder (it's scanned right away), a link to
  add a single game, and a hint when your folders are scanned but no games were found.
- Error messages are translated into all 11 languages: parser errors (unsupported site, site unreachable, blocked
  by Cloudflare or a login wall, HTTP errors), backup, launcher update and hotkey errors.

### Changed
- Dropdowns, scrollbars and checkboxes use the dark theme everywhere. On Linux, dropdowns are no longer light
  on the dark interface.
- The first-run wizard no longer offers to keep data next to the launcher when that folder is read-only
  (Program Files, AppImage).
- The macOS app is ad-hoc signed as a whole bundle. It still isn't notarized, see the README for how to open it
  on macOS 15.

### Fixed
- Erotorrent links work again: the site moved from erotorrent.ru to erotorrent.org. Both addresses are
  recognized, so links already saved in your library keep working.
- F95zone covers and screenshots download again. The site now hides attachments in the first post from guests,
  so pLauncher takes them from its public Latest Updates data.
- F95zone tags are now imported, and languages come from the thread's Language field (Turkish was added).
- Updating metadata from a link no longer removes tags you added by hand.
- Images are saved with their real file extension (AVIF, WebP) instead of the one in the link.

## [1.0.0] - 2026-09-24

First public release: folder scanning with launch-file and engine detection, metadata from F95zone, Pornolab,
Erotorrent, Island of Pleasure and Steam, update tracking, a Steam-style library, privacy features (cover blur,
PIN lock, panic button, hidden collections), bulk actions, backup and restore, self-update, 11 languages.
See the [release notes](https://github.com/BasteArima/pLauncher/releases/tag/v1.0.0).

[Unreleased]: https://github.com/BasteArima/pLauncher/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/BasteArima/pLauncher/releases/tag/v1.1.0
[1.0.0]: https://github.com/BasteArima/pLauncher/releases/tag/v1.0.0
