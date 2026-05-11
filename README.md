# zDB

A single-binary terminal database viewer and editor built with Go + Bubbletea.
Supports SQLite, PostgreSQL, and MySQL with optional AI-powered SQL assistance
through any OpenAI-compatible HTTP API (OpenAI, Gemini, Ollama, Groq, …).

Features:

- **Connection management** — add / edit / delete from inside the TUI; passwords
  go to the OS keyring or are asked on demand.
- **Tabs** — Schema is the fixed first tab; opening tables creates data tabs
  you can flip between with `Ctrl+←/→`.
- **Multi-line SQL editor** with chroma syntax highlighting, schema-aware
  autocomplete, and a built-in formatter (`Ctrl+L`).
- **Infinite-scroll data viewer** that lazy-fetches more rows as you scroll
  past the buffer; `COUNT(*)` runs once so you always see `Loaded N / total T`.
- **AI multi-profile** — switch between OpenAI, Gemini, Ollama, Groq, and any
  custom OpenAI-compatible endpoint; the active profile is one keystroke
  away. AI requests log to a JSONL file and an analytics view shows tokens
  + estimated costs.
- **Catppuccin** Mocha (dark) / Latte (light) palette throughout.

## Screenshots

![zDB data viewer](docs/screenshots/table-copy.png)

See [docs/screenshots.md](docs/screenshots.md) for the full gallery — schema browser, staged edits, saved views, Ask AI, AI profiles, and analytics.

## Requirements

You only need **Go ≥ 1.21** to install zDB. All Go module dependencies (Bubbletea,
lipgloss, chroma, pgx, MySQL driver, modernc/sqlite, etc.) are pinned in
`go.mod` / `go.sum` and resolve automatically when you run `go install` or
`make build`.

Runtime extras (all optional):

| Component | When you want it | Linux | macOS | Windows |
|---|---|---|---|---|
| OS keyring | Store passwords / API keys at rest | gnome-keyring or KWallet (via libsecret) | Keychain (built-in) | Credential Manager (built-in) |
| Clipboard | `y`/`Y` cell / row copy | `xclip` or `wl-clipboard` | `pbcopy` (built-in) | built-in |
| Docker | The bundled `test-data/` postgres+mysql containers | Docker Engine | Docker Desktop | Docker Desktop |

zDB still runs without any of these — it just won't have keyring storage,
system-clipboard copy, or the Docker fixtures.

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
plaintext in the config.

## Configure

Config lives at `~/.config/zdb/config.toml` by default. Override with
`ZDB_CONFIG=/path/to/config.toml`. You almost never need to hand-edit
this file — every setting is reachable from inside the TUI.

```toml
[[connections]]
name   = "dev-sqlite"
engine = "sqlite"
dsn    = "/tmp/dev.db"

[[connections]]
name        = "prod-pg"
engine      = "postgres"
dsn         = "postgres://alice:{password}@host:5432/db"
keyring_key = "zdb/prod-pg"

active_ai = "openai-fast"

[[ais]]
name        = "openai-fast"
provider    = "openai-compat"
base_url    = "https://api.openai.com/v1"
model       = "gpt-4o-mini"
keyring_key = "zdb/ai-key/openai-fast"

[[ais]]
name        = "gemini-pro"
provider    = "openai-compat"
base_url    = "https://generativelanguage.googleapis.com/v1beta/openai"
model       = "gemini-2.5-pro"
keyring_key = "zdb/ai-key/gemini-pro"
```

See `examples/config.toml` for more examples.

### Credential modes

For postgres/mysql, the password can be stored three ways:

1. **OS keyring** (default when added via the form) — TOML carries a DSN
   template with `{password}` and a `keyring_key` pointer.
2. **Env var** — set `dsn_env = "MY_DSN_VAR"` in the connection block; the
   whole DSN is read from that env var at connect time.
3. **Ask at connect** — leave the password field empty when adding. zDB
   saves the DSN with a `{password}` placeholder and **no** keyring entry,
   then prompts every time you connect. Useful when you don't want secrets
   at rest.

## Test data

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

## AI features

zDB supports any **OpenAI-compatible** chat completions API:

| Provider | Notes |
|---|---|
| OpenAI | `gpt-4o-mini` (default), `gpt-4o`, `gpt-4-turbo`, `gpt-3.5-turbo` |
| Gemini | Uses Google's OpenAI-compatible endpoint (since Dec 2024); `gemini-2.5-flash`, `2.5-pro`, `2.5-flash-lite`, `2.0-flash`, … |
| Ollama | Local, no API key required |
| Groq | `llama3-8b-8192`, `llama3-70b-8192`, `mixtral-8x7b-32768` |
| Custom | Any endpoint speaking OpenAI's `/chat/completions` format |

### Setup (first time)

Press `Ctrl+A` (or `F2`) from any data tab. If no AI is configured, a
wizard opens. Pick a preset, optionally override the model from a
selector (each preset has its own list, plus `Other…` for custom IDs),
paste the API key, hit Enter. The key is saved to the OS keyring under
`zdb/ai-key/<profile-name>` — never plaintext in the config.

### Multiple profiles

Press `Ctrl+P` to open the **AI Profiles** modal. From there:

| Key | Action |
|---|---|
| `↑/↓` | Navigate |
| `Enter` | Activate the highlighted profile (re-inits the provider) |
| `a` | Add a new profile (opens the wizard) |
| `e` | Edit selected (name is locked, other fields editable) |
| `d` | Delete selected (confirms; drops the keyring entry) |
| `g` | Open the analytics dashboard |
| `Esc` | Close |

The active profile is the one that runs every Ask, Suggest, and analytics
attribution — switching is one keystroke.

### Asking the AI

`Ctrl+A` on any data tab opens the Ask Panel. Type your question in
natural language; while the AI thinks you'll see `⏳ Asking the AI…`
and the input is locked except for `Esc`. When the response arrives:

- **Read-only SQL** (`SELECT` / `WITH` / `EXPLAIN` / `SHOW` / `PRAGMA` /
  `DESCRIBE`): auto-executes and shows you the result table.
- **Mutating SQL** (`INSERT` / `UPDATE` / `DELETE` / `CREATE` / `DROP` /
  `ALTER`): falls back to a preview — you press `y` to confirm. The AI
  cannot write without explicit user consent.

### Debug recovery loop

When an AI-driven query fails (e.g., a column the AI hallucinated), the
**AI Debug** panel pops up with the full failure context: question, the
SQL the AI generated, and the DB error. Type a hint ("the year column is
in enrollments, not courses"), press `Enter`, and zDB sends everything
back to the AI for a corrected attempt. Loop until success or `Esc`.

`Ctrl+E` from the debug panel opens the failed SQL in the editor for
manual takeover.

### Analytics

`Ctrl+P` → `g` opens the AI Analytics dashboard:

```
AI usage — last 7 days
Requests 47   Tokens in/out 16432   ok 47 / 47   Cost (est.) $0.0143

By profile
  openai-fast    32 req   12410 in   3120 out   $0.0091
  gemini-pro     15 req    4022 in   1090 out   $0.0052

Last 8 requests
  05-10 15:23  ✓  ask         openai-fast      342→127   $0.00009    820ms
  ...

d today · w 7 days · m 30 days · a all · Esc close
```

Each request is logged to `~/.local/state/zdb/ai-usage.jsonl`. Pricing for
known models (gpt-4o-mini, gemini-2.5-flash, llama3-8b, …) is hardcoded
per-1k-token; unknown models log without a cost estimate.

## Keybindings

zDB shows a context-aware help bar at the bottom — these tables are the
highlights. The help bar surfaces the rest based on what's actionable
right now (marks, staged edits, buffer boundary, …).

### Connection picker

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate |
| `Enter` | Connect |
| `n` | New connection (form) |
| `e` | Edit selected |
| `d` | Delete selected |
| `Ctrl+c` | Quit |

### Connection form (add / edit)

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | Next / previous field |
| `←` / `→` | Choose engine |
| `Enter` | Test + save |
| `Esc` | Cancel |

### Schema browser

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate tables |
| `Enter` | Open table (current data tab) |
| `Ctrl+T` | Open in new tab |
| `Ctrl+←` / `Ctrl+→` | Cycle tabs |
| `:` | Raw SQL bar |
| `Ctrl+E` | Full-screen SQL editor |
| `Ctrl+A` / `F2` | Ask AI |
| `Ctrl+P` | AI profiles |
| `V` | Saved views |
| `s` / `S` / `D` | Save / review / discard staged edits |
| `Esc` | Back to picker |

### Data viewer

Tables open with the first 50 rows loaded plus a `COUNT(*)` to show
`Loaded N / total T` in the status line.

**Navigation:**

| Key | Action |
|---|---|
| `←↑↓→` or `hjkl` | Cell cursor |
| `g` / `G` | Top / bottom of loaded buffer |
| `0` / `$` | First / last column |
| `↓` / `j` at last loaded row | **Infinite scroll**: fetches next 50, appends; cursor lands on first new row |
| `Ctrl+f` / `Ctrl+b` | **Page replace**: jumps next/previous DB page (50 rows, buffer replaced) |

**Tabs:**

| Key | Action |
|---|---|
| `Ctrl+W` | Close current tab |
| `Ctrl+←` / `Ctrl+→` | Cycle |
| `Esc` | Back to Schema tab |

**Row selection:**

| Key | Action |
|---|---|
| `Space` | Mark / unmark current row, sets the range anchor |
| `M` or `Shift+Space` | Mark range from anchor to cursor (additive) |
| `Esc` | Clears marks if any, else exits to Schema |

`Shift+Space` only works on terminals with the Kitty keyboard protocol;
`M` is the always-works fallback.

**Clipboard:**

| Key | Action |
|---|---|
| `y` | Copy current cell value |
| `Y` | Copy current row (or all marked rows) as TSV with header |

**Editing:**

| Key | Action |
|---|---|
| `Enter` | Edit cell under cursor |
| `v` | View full cell content (modal) |
| `s` / `S` / `D` | Save / review / discard staged edits |
| `d` | Delete row (red confirm) |

**SQL & AI:**

| Key | Action |
|---|---|
| `:` | Raw SQL bar (filters JOIN result if active, else full statement) |
| `Ctrl+E` | Full-screen SQL editor |
| `J` | Join wizard |
| `V` / `W` | Saved views / save current SQL as view |
| `Ctrl+A` / `F2` | Ask AI |
| `Ctrl+P` | AI profiles |

### SQL Editor

| Key | Action |
|---|---|
| Type | Multi-line entry |
| `Tab` | Autocomplete (schema-aware) |
| `Ctrl+L` | Format SQL |
| `Ctrl+R` | Run |
| `Ctrl+S` | Save as view |
| `Esc` | Back (buffer preserved) |

### Ask Panel

| Key | Action |
|---|---|
| Type | Question in natural language |
| `Enter` | Submit (locks input while loading) |
| `y` / `Ctrl+Enter` | Confirm-execute the previewed SQL (mutating only) |
| `Esc` | Cancel / close |

### AI Debug Panel

| Key | Action |
|---|---|
| Type | Hint to the AI |
| `Enter` | Retry with hint |
| `Ctrl+E` | Open the failed SQL in the SQL editor |
| `Esc` | Cancel |

### AI Profiles

| Key | Action |
|---|---|
| `Enter` | Activate selected |
| `a` / `e` / `d` | Add / edit / delete |
| `g` | Analytics |
| `Esc` | Close |

### AI Analytics

| Key | Action |
|---|---|
| `d` | Today |
| `w` | Last 7 days |
| `m` | Last 30 days |
| `a` | All time |
| `Esc` | Close |

### Confirm modals

| Key | Action |
|---|---|
| `y` | Confirm |
| `n` / `Esc` | Cancel |

## Safety

- AI-generated SQL **auto-executes only when read-only** (`SELECT` family).
  Mutating statements always go through a preview-and-confirm step.
- All mutations (UPDATE, DELETE, raw SQL) run inside an explicit transaction;
  you review and confirm before commit.
- DSNs and API keys never appear in logs or error messages.
- Tables without a primary key are read-only (edit/delete keys disabled).
- Passwords and AI API keys go to the OS keyring or are requested at use
  time — never stored in plaintext in the config file.
- The AI usage log records token counts and metadata only — never the
  prompt itself or the AI's response.

## Debug logging

```sh
ZDB_DEBUG=1 zdb
tail -f $XDG_STATE_HOME/zdb/log    # or ~/.local/state/zdb/log
```

## Contributing

```sh
make test               # unit tests (no Docker required)
make test-integration   # conformance tests (needs TEST_POSTGRES_DSN / TEST_MYSQL_DSN)
make lint               # go vet
make fmt                # gofmt
```
