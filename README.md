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
git clone https://github.com/gabiito/zdb
cd zDB
make build
# binary at bin/zdb
```

**Cross-compile:**

```sh
GOOS=linux   GOARCH=amd64 make build
GOOS=linux   GOARCH=arm64 make build
GOOS=darwin  GOARCH=amd64 make build
GOOS=darwin  GOARCH=arm64 make build
```

## Configure

Create `~/.config/zdb/config.toml` (or set `$ZDB_CONFIG` to a custom path):

```toml
[[connections]]
name   = "my-sqlite"
engine = "sqlite"
dsn    = "/path/to/database.db"

[[connections]]
name   = "local-pg"
engine = "postgres"
dsn    = "postgres://user:pass@localhost:5432/mydb?sslmode=disable"

# Optional AI section — omit to disable AI features
[ai]
provider    = "openai-compat"
base_url    = "https://api.openai.com/v1"
model       = "gpt-4o-mini"
api_key_env = "OPENAI_API_KEY"
```

See `examples/config.toml` for more provider examples (Ollama, Groq).

## Basic Usage

```
zdb               # read config from default path
ZDB_CONFIG=/path/to/config.toml zdb
ZDB_DEBUG=1 zdb   # enable debug logging (to XDG_STATE_HOME/zdb/log)
```

### Keybindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `g` | Go to top |
| `G` | Go to bottom |
| `Ctrl+f` | Page forward |
| `Ctrl+b` | Page back |
| `Enter` | Select connection / open table / edit cell |
| `v` | View full cell content |
| `s` | Save staged changes (opens confirm banner) |
| `d` | Delete row (opens red confirm banner) |
| `:` | Open raw SQL panel |
| `Ctrl+a` / `F2` | Open AI Ask panel |
| `Ctrl+Space` | AI suggest in SQL panel |
| `y` | Confirm action |
| `Esc` / `n` | Cancel / back |
| `Ctrl+c` | Quit |

## AI Configuration

AI features require an `[ai]` section in your config pointing to any OpenAI-compatible API.

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

AI is silently disabled if:
- The `[ai]` section is absent
- `base_url` or `model` is empty
- The environment variable named by `api_key_env` is unset (except for Ollama-style providers that don't require a key — in that case the request succeeds without a bearer token)

## Safety

- AI-generated SQL is **never auto-executed** — always shown in a preview pane first
- All mutations (UPDATE, DELETE, raw SQL) run in an explicit transaction; you confirm before commit
- DSNs and API keys never appear in logs or error messages
- Tables without a primary key are read-only (edit and delete keys are disabled)

## Contributing

```sh
make test               # unit tests (no Docker required)
make test-integration   # conformance tests (requires TEST_POSTGRES_DSN / TEST_MYSQL_DSN)
make lint               # go vet
make fmt                # gofmt
```
