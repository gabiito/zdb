# Changelog

All notable changes to zDB will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] — 2026-05-15

First feature-complete minor bump. Bundles three SDD cycles' worth of
work into a single release: per-connection saved views with a guided
copy-view flow, the full app-owns-config stack (schema versioning,
migration framework, `zdb config import` CLI, external-modification
detection), and the config-durability foundation (atomic writes,
rotating backup, strict TOML parsing). Plus quality-of-life fixes for
Esc handling on internal lists and gated save semantics in the SQL
editor.

### Added

#### Database & navigation
- Connection picker with case-insensitive name uniqueness and secure
  credential storage (OS keyring, env var, or inline DSN — in that
  order of preference).
- Two-pane schema browser (tables on the left, columns on the right)
  with live highlight syncing.
- Inline data viewer with pagination (infinite scroll + row count),
  cell view, cell edit, and row delete.
- Tabbed workspace: Schema tab plus per-table data tabs with
  Ctrl+Left/Right navigation.
- JOIN wizard for guided multi-table queries, including extend mode
  for appending JOIN clauses to an existing query.

#### SQL & views
- Full-screen SQL editor (Ctrl+E) with autocomplete, formatting
  (Ctrl+L), and run-or-save shortcuts (Ctrl+R / Ctrl+S).
- Inline SQL bar (`:` to focus) with tab completion.
- Saved views per connection at `~/.config/zdb/views/<slug>/views.toml`.
- Copy-view-from-another-connection flow: prefills the SQL editor
  from any other connection's view, lets you edit, executes against
  the active connection, and only saves if the query succeeds.
- Force-rename on view name collisions within a connection.

#### AI assistance
- Multi-profile AI integration (openai-compat) with switchable
  profiles, per-profile API key resolution (env var or keyring), and
  inline ghost-text suggestions.
- Ask panel (Ctrl+A or F2) for natural-language queries.
- AI debug panel surfaces errors with full context for prompt fixes.
- AI usage analytics.

#### Config durability
- Atomic writes via tempfile + `Sync` + rename — a crash mid-save
  never leaves a partial config.
- Rotating backup-on-write at `config.toml.bak` (best-effort, never
  blocks the save).
- Strict TOML parsing that rejects unknown keys with a locked
  recovery hint format.
- Schema version field with a forward-only migration framework
  (registry currently empty for v1, ready for v2 onwards).
- External-modification detection: `Save()` refuses to overwrite a
  config that was modified between load and save, returning the
  typed `ErrConfigChangedExternally`. Opt out via
  `ZDB_SKIP_STALE_CHECK=1` for sync-tool environments.

#### CLI & operations
- `zdb config import <path>` — one-shot adoption of a config from
  another setup. Strict-decodes the source, runs forward migrations
  to the current version, validates, and writes atomically over the
  destination via the explicit "I'm the new owner" path.
- `--version` flag reporting build info from `runtime/debug.BuildInfo`.
- F1 shortcuts overlay listing every keybinding by screen.

### Breaking

- The legacy global `~/.config/zdb/views.toml` is moved aside to
  `views.toml.legacy.bak` on first boot post-upgrade. Users with
  saved views in the old format must re-create them in the new
  per-connection structure. (Subsequent migrations append a numeric
  suffix: `.legacy.bak.1`, `.legacy.bak.2`, etc.)

### Fixed
- Pressing **Esc** on any internal `bubbles/list` (table picker in the
  schema browser, connection picker, views modal, JOIN wizard) no
  longer quits the app — the library's default Quit binding on
  `[q, esc]` is now disabled on every list the TUI constructs.
- **Ctrl+S** in the SQL editor is now gated by a successful execute
  against the active connection. Previously the editor could save a
  view whose SQL would fail every time it ran (e.g. after editing a
  copied view to reference a non-existent table). Save still fires
  after the gate passes; on execute failure the editor stays open
  with the error visible.
- `TestLoadFullConfig`, `TestLoadAIDisabledConfig`, and
  `TestDefaultAPIKeyEnv` have been migrated to the `ActiveProfile()`
  API. They were dereferencing the deprecated `cfg.AI` pointer that
  `Load()` zeroes after copying values into `AIs[0]`, panicking on
  every run. The full test suite is now green.

### Supported platforms
- `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`.
- macOS-specific atomic-rename semantics are honored.
- Windows is not an officially supported target.

## [0.1.3] — 2026-05-11

### Added
- F1 shortcuts overlay listing every keybinding by screen.
- `--version` flag (and `-v`, `version`) reporting build info via
  `runtime/debug.BuildInfo`.

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
