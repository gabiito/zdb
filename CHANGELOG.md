# Changelog

All notable changes to zDB will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] — 2026-05-15

First feature-complete release. A single-binary TUI database viewer for
SQLite, Postgres, and MySQL with built-in AI assistance and durable
config management.

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

### Supported platforms
- `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`.
- macOS-specific atomic-rename semantics are honored.
- Windows is not an officially supported target.

[Unreleased]: https://github.com/gabiito/zdb/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/gabiito/zdb/releases/tag/v0.1.0
