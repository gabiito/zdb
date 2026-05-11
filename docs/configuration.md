# Configuration

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

## Credential modes

For postgres/mysql, the password can be stored three ways:

1. **OS keyring** (default when added via the form) — TOML carries a DSN
   template with `{password}` and a `keyring_key` pointer.
2. **Env var** — set `dsn_env = "MY_DSN_VAR"` in the connection block; the
   whole DSN is read from that env var at connect time.
3. **Ask at connect** — leave the password field empty when adding. zDB
   saves the DSN with a `{password}` placeholder and **no** keyring entry,
   then prompts every time you connect. Useful when you don't want secrets
   at rest.

## Debug logging

```sh
ZDB_DEBUG=1 zdb
tail -f $XDG_STATE_HOME/zdb/log    # or ~/.local/state/zdb/log
```
