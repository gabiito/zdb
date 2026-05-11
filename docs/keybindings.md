# Keybindings

zDB shows a context-aware help bar at the bottom — these tables are the
highlights. The help bar surfaces the rest based on what's actionable
right now (marks, staged edits, buffer boundary, …).

## Connection picker

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate |
| `Enter` | Connect |
| `n` | New connection (form) |
| `e` | Edit selected |
| `d` | Delete selected |
| `Ctrl+c` | Quit |

## Connection form (add / edit)

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | Next / previous field |
| `←` / `→` | Choose engine |
| `Enter` | Test + save |
| `Esc` | Cancel |

## Schema browser

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

## Data viewer

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

## SQL Editor

| Key | Action |
|---|---|
| Type | Multi-line entry |
| `Tab` | Autocomplete (schema-aware) |
| `Ctrl+L` | Format SQL |
| `Ctrl+R` | Run |
| `Ctrl+S` | Save as view |
| `Esc` | Back (buffer preserved) |

## Ask Panel

| Key | Action |
|---|---|
| Type | Question in natural language |
| `Enter` | Submit (locks input while loading) |
| `y` / `Ctrl+Enter` | Confirm-execute the previewed SQL (mutating only) |
| `Esc` | Cancel / close |

## AI Debug Panel

| Key | Action |
|---|---|
| Type | Hint to the AI |
| `Enter` | Retry with hint |
| `Ctrl+E` | Open the failed SQL in the SQL editor |
| `Esc` | Cancel |

## AI Profiles

| Key | Action |
|---|---|
| `Enter` | Activate selected |
| `a` / `e` / `d` | Add / edit / delete |
| `g` | Analytics |
| `Esc` | Close |

## AI Analytics

| Key | Action |
|---|---|
| `d` | Today |
| `w` | Last 7 days |
| `m` | Last 30 days |
| `a` | All time |
| `Esc` | Close |

## Confirm modals

| Key | Action |
|---|---|
| `y` | Confirm |
| `n` / `Esc` | Cancel |
