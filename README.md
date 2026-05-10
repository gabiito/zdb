# zDB

A single-binary terminal database viewer and editor built with Go + Bubbletea.
Supports SQLite, PostgreSQL, and MySQL. Optional AI-powered SQL assistance via
any OpenAI-compatible HTTP API (OpenAI, Ollama, Groq, etc.).

## Install

**From source:**

```sh
go install github.com/gabiito/zdb/cmd/zdb@latest
```

**Build locally (CGO-free):**

```sh
git clone git@github.com:gabiito/zdb.git
cd zdb
make build
# binary at bin/zdb
```

**Cross-compile:**

```sh
GOOS=linux  GOARCH=amd64 make build
GOOS=linux  GOARCH=arm64 make build
GOOS=darwin GOARCH=amd64 make build
GOOS=darwin GOARCH=arm64 make build
```

## First run

Just run it:

```sh
zdb
```

If you don't have a config yet, zDB drops you on a welcome screen — press `n`
to add your first connection through a form. Name, engine (selector with
←/→), DSN, and an optional password. The form tests the connection and, on
success, persists it to `~/.config/zdb/config.toml`.

Passwords for postgres/mysql go to your **OS keyring** (gnome-keyring,
KWallet, macOS Keychain, Windows Credential Manager). They never sit in
plaintext in the config file.

## Configure

Config lives at `~/.config/zdb/config.toml` by default.
Override with `ZDB_CONFIG=/path/to/config.toml`.

Most users won't hand-edit this file — the in-app forms manage it. But it's
plain TOML if you want to:

```toml
[[connections]]
name   = "my-sqlite"
engine = "sqlite"
dsn    = "/path/to/database.db"

[[connections]]
name        = "prod-pg"
engine      = "postgres"
dsn         = "postgres://alice:{password}@host:5432/db"
keyring_key = "zdb/prod-pg"   # password stored in OS keyring

# Optional AI section — omit to disable
[ai]
provider    = "openai-compat"
base_url    = "https://api.openai.com/v1"
model       = "gpt-4o-mini"
api_key_env = "OPENAI_API_KEY"
```

See `examples/config.toml` for more provider examples (Ollama, Groq).

### Credential modes

For postgres/mysql, the password can be stored three ways:

1. **OS keyring** (default when adding via the form) — TOML carries a DSN
   template with `{password}` and a `keyring_key` pointer.
2. **Env var** — set `dsn_env = "MY_DSN_VAR"` in the connection block; the
   whole DSN is read from that env var at connect time.
3. **Ask at connect** — leave the password field empty when adding. zDB saves
   the DSN with a `{password}` placeholder and **no keyring entry**, then
   prompts for the password every time you connect. Useful when you don't
   want secrets at rest.

## Test data

`test-data/` ships a portable seed for SQLite, PostgreSQL, and MySQL — same
data, three engines. School information system with table-per-type
inheritance (persons → students/teachers/staff), 100 persons, 1400 attendance
rows.

```sh
./test-data/apply.sh sqlite          # /tmp/dev.db, no Docker needed
./test-data/apply.sh up              # postgres + mysql via docker compose
./test-data/apply.sh all             # apply schema+data to all three

ZDB_CONFIG=$(pwd)/test-data/config.example.toml zdb
```

See `test-data/README.md` for details.

## Keybindings

zDB shows a context-aware help bar at the bottom — what's documented below
is the highlights. Open the app and watch the bar to discover the rest.

### Connection picker

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate |
| `Enter` | Connect |
| `n` | New connection (form) |
| `e` | Edit selected |
| `d` | Delete selected (with confirm) |
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
| `Enter` | Open table |
| `:` | Raw SQL bar |
| `V` | Saved views |
| `s` / `S` / `D` | Save / review / discard staged edits |
| `Esc` | Back to picker |

### Data viewer

| Key | Action |
|---|---|
| `←↑↓→` or `hjkl` | Cell cursor |
| `g` / `G` | Top / bottom of result set |
| `Ctrl+f` / `Ctrl+b` | Page forward / back |
| `0` / `$` | First / last column |
| `Enter` | Edit cell under cursor |
| `v` | View full cell content (modal) |
| `y` | Copy cell value to clipboard |
| `Y` | Copy current row (or marked rows) as TSV with header |
| `Space` | Mark / unmark current row |
| `s` / `S` / `D` | Save / review / discard staged edits |
| `d` | Delete row (red confirm) |
| `:` | Raw SQL bar |
| `J` | Join wizard |
| `V` / `W` | Saved views / save current SQL as view |
| `Ctrl+a` or `F2` | Ask AI |
| `Esc` | Clear marks (if any), else back |
| `Ctrl+c` | Quit |

### Confirm modals

| Key | Action |
|---|---|
| `y` | Confirm |
| `n` / `Esc` | Cancel |

## AI Configuration

AI features require an `[ai]` section pointing to any OpenAI-compatible API.

### OpenAI (cloud)

```toml
[ai]
base_url    = "https://api.openai.com/v1"
model       = "gpt-4o-mini"
api_key_env = "OPENAI_API_KEY"
```

```sh
export OPENAI_API_KEY=sk-...
zdb
```

### Ollama (local)

```toml
[ai]
base_url    = "http://localhost:11434/v1"
model       = "llama3"
api_key_env = "OLLAMA_KEY"   # leave OLLAMA_KEY unset — Ollama doesn't require a key
```

### Groq (cloud)

```toml
[ai]
base_url    = "https://api.groq.com/openai/v1"
model       = "llama3-8b-8192"
api_key_env = "GROQ_API_KEY"
```

AI is silently disabled when:
- The `[ai]` section is absent
- `base_url` or `model` is empty
- The env var named by `api_key_env` is unset (except providers like Ollama
  that don't require a key — in that case the request still goes through
  without a bearer token)

## Safety

- AI-generated SQL is **never auto-executed** — always shown in a preview pane first.
- All mutations (UPDATE, DELETE, raw SQL) run inside an explicit transaction;
  you review and confirm before commit.
- DSNs and API keys never appear in logs or error messages.
- Tables without a primary key are read-only (edit/delete keys disabled).
- Passwords go to the OS keyring or are requested at connect time — never
  stored in plaintext in the config.

## Debug logging

```sh
ZDB_DEBUG=1 zdb
tail -f $XDG_STATE_HOME/zdb/log    # or ~/.local/state/zdb/log
```

## Contributing

```sh
make test               # unit tests (no Docker required)
make test-integration   # conformance tests (uses TEST_POSTGRES_DSN / TEST_MYSQL_DSN)
make lint               # go vet
make fmt                # gofmt
```
