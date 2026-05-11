# zDB — Screenshots

A walkthrough of the main views, in roughly the order you encounter them.

## Schema browser

Fixed first tab, lists every table in the active connection. `Enter` opens a table in the current data tab; `Ctrl+T` opens it in a new one.

![Schema view](screenshots/schema.png)

## Data viewer with row copy

Mark rows with `Space`, copy as TSV (including header) with `Y`. Single-cell copy is `y`.

![Table view with copied rows](screenshots/table-copy.png)

## Staged edits

Every cell mutation goes into a staged-edits buffer inside an explicit transaction. `S` opens the review modal, `s` commits, `D` discards.

![Staged edits review](screenshots/staged-edit.png)

## Saved views

Name and recall frequent queries with `W` (save current SQL as view) and `V` (open the views list).

![Saved views](screenshots/saved-views.png)

## Ask AI

Natural-language question on any data tab. Read-only SQL auto-executes; mutating SQL falls back to preview-and-confirm.

![Ask AI panel](screenshots/ask-ai.png)

## Ask AI — result

The AI answer runs against the active connection and the result table is shown inline.

![Ask AI result](screenshots/ask-ai-result.png)

## AI profiles

Switch between OpenAI, Gemini, Ollama, Groq, or any custom OpenAI-compatible endpoint. The active profile drives every Ask, Suggest, and analytics attribution.

![AI profiles modal](screenshots/ai-profiles.png)

## AI analytics

Per-profile token usage and estimated cost, sourced from the local `~/.local/state/zdb/ai-usage.jsonl` log.

![AI analytics dashboard](screenshots/ai-profiles-analytics.png)
