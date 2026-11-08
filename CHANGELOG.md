# Changelog

Notable changes to pLauncher. Dates are UTC.
The text of each GitHub release comes from `docs/releases/vX.Y.Z.md`.

## [Unreleased]

### Added
- Pornolab multi-game release backend: section-specific metadata and update checks, explicit source
  selection (`#game=N` plus a saved title), a bounded page cache and manual-choice results for bulk
  metadata filling. Includes neutral Windows-1251 fixtures and localized errors. The selection
  interface and clickable manual-work summary are pending a separate frontend step.

### Fixed
- Metadata tabs now route links entered in Search by name through the ordinary link parser, including
  addresses without `https://`. Returning to name search restores the suggested folder name without
  repeating the search or clearing results. The link field survives tab changes, and switching from a
  name field containing a link transfers it to Paste a link in both prominent and compact panels.

## [1.2.0] - 2026-11-07

### Added
- **Find game info by name.** pLauncher takes the folder name, searches F95zone, Steam and VNDB, ranks the matches
  and shows why it likes each one. Preview a match before saving it. VNDB links can now be pasted too.
- **Play sessions and statistics.** Every session is recorded, even if you close the launcher while playing.
  The game page shows the last eight weeks; the new **Statistics** view shows hours this week, this month and all
  time, a year of weekly activity, top games, statuses, engines and disk usage.
- **Several launch files per game.** Keep the config tool and the walkthrough next to the game: they open from the
  menu beside Play, and only the main file counts as playtime. Pick them in edit mode or let pLauncher find them
  in the game folder.
- **Play status, rating and notes:** playing / want to play / finished / dropped, 5 stars, and private notes where
  `||text||` stays hidden as a spoiler until clicked. Status chips above the grid filter by status.
- **Smart lists** for what needs attention: updates, no cover, no launch file, no source link and missing folders,
  each with one button for the whole list. «No source link» looks every game up by name and fills in only the
  confident matches, leaving the rest for you to pick.
- Update notices show when the new version was found; notes show when they were last edited.
- The game page shows which list you came from (`Playing · 2 of 12`) with arrows to the previous and next game.
- Back and forward like in a browser: mouse side buttons or Alt+←/→. A «Back to top» button in long lists.
- Filter by several tags at once (all of them or any of them) and exclude the ones you don't want to see.
- Edit mode looks like the game page itself: unsaved changes are marked, Esc cancels and Ctrl+S saves. Screenshots
  can be reordered by dragging, any of them can become the cover, and image files dropped onto the window are added.
- Covers can be zoomed and framed separately for wide and tall cards.
- «Remove from Continue playing» in the game menu. Finished and dropped games no longer show there.

### Changed
- **New look:** a flat dark theme, the Inter font, line icons instead of emoji and a new application icon.
  The library opens as a grid of wide cards (tall ones are in the view options); shelves and the big hero banner
  are optional.
- Search moved to the title bar. The sidebar starts with «Scan folders» and «Statistics», then Collections;
  clicking a collection opens it as a grid.
- The game page has a media viewer with the cover first and a play card with stats, status, rating, updates and
  sites. Games without info show a «Find game info» block.
- Context menus open where you click. F1 lists the new shortcuts.

### Fixed
- Scrolling a large library is smooth again (blur effects no longer run on every card).
- Holding a screenshot no longer gets the window stuck in drag-and-drop mode.
- Full-screen screenshots are no longer smaller than in the gallery.

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

[Unreleased]: https://github.com/BasteArima/pLauncher/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/BasteArima/pLauncher/releases/tag/v1.2.0
[1.1.0]: https://github.com/BasteArima/pLauncher/releases/tag/v1.1.0
[1.0.0]: https://github.com/BasteArima/pLauncher/releases/tag/v1.0.0
