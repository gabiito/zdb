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

![zDB data viewer](docs/screenshots/table-copy.gif)

See [docs/screenshots.md](docs/screenshots.md) for the full gallery — schema browser, staged edits, saved views, Ask AI, AI profiles, and analytics.

## Documentation

- **[Getting started](docs/getting-started.md)** — install, first connection, test data, tabs, SQL editor
- **[Configuration](docs/configuration.md)** — `config.toml`, credential modes, debug logging
- **[AI features](docs/ai.md)** — providers, profiles, asking, debug loop, analytics
- **[Keybindings](docs/keybindings.md)** — full reference across every view

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

## Contributing

```sh
make test               # unit tests (no Docker required)
make test-integration   # conformance tests (needs TEST_POSTGRES_DSN / TEST_MYSQL_DSN)
make lint               # go vet
make fmt                # gofmt
```
