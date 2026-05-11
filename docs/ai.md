# AI features

zDB supports any **OpenAI-compatible** chat completions API:

| Provider | Notes |
|---|---|
| OpenAI | `gpt-4o-mini` (default), `gpt-4o`, `gpt-4-turbo`, `gpt-3.5-turbo` |
| Gemini | Uses Google's OpenAI-compatible endpoint (since Dec 2024); `gemini-2.5-flash`, `2.5-pro`, `2.5-flash-lite`, `2.0-flash`, … |
| Ollama | Local, no API key required |
| Groq | `llama3-8b-8192`, `llama3-70b-8192`, `mixtral-8x7b-32768` |
| Custom | Any endpoint speaking OpenAI's `/chat/completions` format |

## Setup (first time)

Press `Ctrl+A` (or `F2`) from any data tab. If no AI is configured, a
wizard opens. Pick a preset, optionally override the model from a
selector (each preset has its own list, plus `Other…` for custom IDs),
paste the API key, hit Enter. The key is saved to the OS keyring under
`zdb/ai-key/<profile-name>` — never plaintext in the config.

## Multiple profiles

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

## Asking the AI

`Ctrl+A` on any data tab opens the Ask Panel. Type your question in
natural language; while the AI thinks you'll see `⏳ Asking the AI…`
and the input is locked except for `Esc`. When the response arrives:

- **Read-only SQL** (`SELECT` / `WITH` / `EXPLAIN` / `SHOW` / `PRAGMA` /
  `DESCRIBE`): auto-executes and shows you the result table.
- **Mutating SQL** (`INSERT` / `UPDATE` / `DELETE` / `CREATE` / `DROP` /
  `ALTER`): falls back to a preview — you press `y` to confirm. The AI
  cannot write without explicit user consent.

## Debug recovery loop

When an AI-driven query fails (e.g., a column the AI hallucinated), the
**AI Debug** panel pops up with the full failure context: question, the
SQL the AI generated, and the DB error. Type a hint ("the year column is
in enrollments, not courses"), press `Enter`, and zDB sends everything
back to the AI for a corrected attempt. Loop until success or `Esc`.

`Ctrl+E` from the debug panel opens the failed SQL in the editor for
manual takeover.

## Analytics

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
