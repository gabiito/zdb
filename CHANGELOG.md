# Changelog

All notable changes to zDB will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] — 2026-05-15

### Added
- **Per-connection saved views.** Each connection keeps its own list at
  `~/.config/zdb/views/<name>/views.toml`. Switching connections shows
  only that connection's views.
- **Copy a view from another connection.** Press `C` in the views modal
  to browse views from any other connection, pick one, and the SQL
  editor opens with that SQL prefilled. You can edit it, and the view
  only saves if it actually runs on the current connection.
- **`zdb config import <path>`** — pull a config from another machine
  or setup. The file is validated and migrated to the current format
  before replacing your config.
- **Running version is shown under the logo** on the connection picker.
- **Connection names are unique case-insensitive.** Trying to add or
  rename to a duplicate gets a clear inline error.

### Changed
- **Saves are atomic.** A crash or kill mid-save can no longer leave
  your config or views file corrupt.
- **Automatic backup.** Every save of your config writes a rotating
  `config.toml.bak` next to it so you can recover from a bad save with
  one rename.
- **Unknown keys in your config now error instead of being silently
  dropped.** The error message lists exactly which keys are unknown
  and how to fix them. Useful for catching typos like `nme = "..."`.
- **External-modification protection.** If your `config.toml` is
  changed by something other than zDB between load and save (e.g. you
  edit it in another terminal), zDB refuses to overwrite and tells
  you to reconcile. If you use file-sync tooling (Dropbox, Syncthing)
  that updates mtime spuriously, set `ZDB_SKIP_STALE_CHECK=1` to opt
  out.
- The config file format gained an internal `version` field so
  future format changes can migrate forward automatically. Existing
  configs without it work as before — no action required.

### Fixed
- **Pressing Esc on any list no longer quits the app.** Tables list,
  connection picker, views modal, JOIN wizard — Esc now steps back
  like you'd expect.
- **Ctrl+S in the SQL editor now requires the query to run first.**
  Before, you could save a view with broken SQL (e.g. a typo'd table
  name) that would fail every time you ran it later. Now it executes
  first; the save prompt only opens on success.

### Breaking
- The old global `~/.config/zdb/views.toml` is moved aside to
  `views.toml.legacy.bak` on first launch after upgrade. Views are
  per-connection now, so any saved views from the old format must be
  re-created — you can copy them out of `views.toml.legacy.bak` and
  re-save them on the right connection.

## [0.1.3] — 2026-05-11

### Added
- F1 shortcuts overlay listing every keybinding by screen.
- `--version` flag (and `-v`, `version`) reporting build info.

## [0.1.2] — 2026-05-11

### Added
- Connection picker logo panel (right-hand wordmark when the terminal
  is wide enough).

## [0.1.1] — 2026-05-11

### Added
- MIT license.

## [0.1.0] — 2026-05-11

### Added
- First release: single-binary TUI database viewer for SQLite,
  Postgres, and MySQL. Connection picker, schema browser, data viewer
  with cell edit, raw SQL panel, JOIN filter, secure secret storage,
  AI Ask panel, Catppuccin palette.

[Unreleased]: https://github.com/gabiito/zdb/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/gabiito/zdb/compare/v0.1.3...v0.2.0
[0.1.3]: https://github.com/gabiito/zdb/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/gabiito/zdb/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/gabiito/zdb/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/gabiito/zdb/releases/tag/v0.1.0
