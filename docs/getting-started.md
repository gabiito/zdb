# Getting started with zDB

A walkthrough from zero to your first AI-assisted query. We'll install the binary,
spin up the bundled test data, open a table, run some SQL, and try the Ask AI flow.

## Install

**Quick install (recommended):**

```sh
git clone git@github.com:gabiito/zdb.git
cd zdb
./install.sh
```

The installer builds the binary, copies it to a directory that's
already on your `$PATH` (`~/.local/bin`, `~/bin`, or `$GOPATH/bin` —
whichever is reachable), and creates the config + state dirs so the
first run doesn't trip on missing paths. It also tells you exactly what
to add to your shell rc if `$PATH` needs adjusting. Re-running it is
safe — it just overwrites the same destination.

**With `go install`:**

```sh
go install github.com/gabiito/zdb/cmd/zdb@latest
```

`go install` auto-resolves the dependency graph from `go.mod`, downloads,
verifies checksums against `go.sum`, and drops the binary in `$GOPATH/bin`
(usually `~/go/bin`). That directory has to be on your `$PATH` for the
`zdb` command to be visible — check with:

```sh
echo $PATH | tr ':' '\n' | grep go/bin
```

If nothing prints, add `export PATH="$HOME/go/bin:$PATH"` to your shell rc.

**Build only (no install):**

```sh
git clone git@github.com:gabiito/zdb.git
cd zdb
make build       # binary at bin/zdb, CGO-free
```

**Cross-compile:**

```sh
GOOS=linux  GOARCH=amd64 make build
GOOS=linux  GOARCH=arm64 make build
GOOS=darwin GOARCH=amd64 make build
GOOS=darwin GOARCH=arm64 make build
```

## First run

```sh
zdb
```

If you don't have a config yet, zDB drops you on a welcome screen. Press
`n` to add your first connection through a guided form:

- **Name** (e.g., `my-pg`)
- **Engine** — selector (`←/→` cycles between sqlite / postgres / mysql)
- **DSN** (file path for sqlite; URL for postgres/mysql)
- **Password** (optional)

The form tests the connection live and, on success, persists it to
`~/.config/zdb/config.toml`. Passwords go to the OS keyring — never
plaintext in the config. For details on the credential modes and config
file format, see [configuration.md](configuration.md).

## Try the bundled test data

`test-data/` ships portable seed data for SQLite, PostgreSQL, and MySQL —
identical rows across the three engines. School information system with
table-per-type inheritance (persons → students/teachers/staff), 100 persons,
1400 attendance rows.

```sh
./test-data/apply.sh sqlite          # /tmp/dev.db, no Docker needed
./test-data/apply.sh up              # postgres + mysql via docker compose
./test-data/apply.sh all             # apply schema+data to all three

ZDB_CONFIG=$(pwd)/test-data/config.example.toml zdb
```

See `test-data/README.md` for details.

## Tabs

The Schema tab is fixed at index 0 and never closes. Opening a table from
the schema browser activates a data tab — by default the same data tab
gets reused for each table you open. To open in a *new* tab, use
`Ctrl+T`.

```
[Schema] [students] [SQL #1]
   ^        ^         ^
   fixed    active    inactive
```

| Key | Action |
|---|---|
| `Enter` on table | Open in current data tab (or create one) |
| `Ctrl+T` on table | Open in a new tab |
| `Ctrl+W` | Close active data tab |
| `Ctrl+←` / `Ctrl+→` | Cycle tabs |
| `Esc` (in data tab) | Back to Schema tab |

Each data tab snapshots its own viewer state — cursor, marks, JOIN chain,
pagination — so you can flip back and forth without losing context.

## SQL Editor

The bottom `:` bar is great for one-liners and JOIN-result filters. For
complex queries, open the full-screen editor with `Ctrl+E`:

| Key | Action |
|---|---|
| Type | Multi-line text entry, syntax highlighted via chroma |
| `Tab` | Schema-aware autocomplete (cycles candidates) |
| `Ctrl+L` | Format SQL (one major clause per line, indent) |
| `Ctrl+R` | Execute and show result in the active tab |
| `Ctrl+S` | Save current SQL as a named view |
| `Esc` | Back (preserves buffer for next open) |

The formatter understands `SELECT` / `FROM` / `WHERE` / `GROUP BY` /
`ORDER BY` / `JOIN` (incl. `LEFT/RIGHT/FULL [OUTER] JOIN`) / `UNION` /
projections with comma-separated columns. It uses chroma's
engine-agnostic SQL lexer so the same formatter handles SQLite,
PostgreSQL, and MySQL syntax.

## Next steps

- Wire up the AI features: [ai.md](ai.md)
- Customize the config file or set up credential modes: [configuration.md](configuration.md)
- Full keybinding reference: [keybindings.md](keybindings.md)
