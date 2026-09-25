# Changelog

Notable changes to pLauncher. Dates are UTC.
The text of each GitHub release comes from `docs/releases/vX.Y.Z.md`.

## [Unreleased]

### Added
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

## [1.0.0] - 2026-09-24

First public release: folder scanning with launch-file and engine detection, metadata from F95zone, Pornolab,
Erotorrent, Island of Pleasure and Steam, update tracking, a Steam-style library, privacy features (cover blur,
PIN lock, panic button, hidden collections), bulk actions, backup and restore, self-update, 11 languages.
See the [release notes](https://github.com/BasteArima/pLauncher/releases/tag/v1.0.0).

[Unreleased]: https://github.com/BasteArima/pLauncher/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/BasteArima/pLauncher/releases/tag/v1.0.0
