package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SqlBarModel is the persistent single-line SQL command bar that lives at the
// bottom of the data viewer. When focused, the textinput receives keystrokes
// and a syntax-highlighted preview of the typed SQL renders directly below
// the input. Enter executes via SqlExecuteMsg; Esc unfocuses; Tab cycles
// completions over a deduped pool of SQL keywords + schema names.
type SqlBarModel struct {
	input   textinput.Model
	focused bool
	width   int

	// Schema sources for autocomplete — set by the App via SetSchema after
	// the schema cache is built.
	tables         []string
	columns        []string
	columnsByTable map[string][]string

	// Active completion state. Reset on any non-Tab key press.
	completing  bool
	candidates  []string
	candidateIx int
	prefixStart int // input value index where the partial word started
}

// NewSqlBarModel constructs a fresh SQL bar sized for the given terminal width.
func NewSqlBarModel(width int) SqlBarModel {
	ti := textinput.New()
	ti.Placeholder = "press : to enter SQL · Enter run · Esc unfocus"
	ti.Prompt = "› "
	ti.PromptStyle = lipgloss.NewStyle().
		Foreground(CtpMauve).
		Bold(true)
	ti.PlaceholderStyle = StyleDim
	ti.CharLimit = 4096
	ti.Width = max(20, width-6)
	return SqlBarModel{input: ti, width: width}
}

// Focus puts the bar into edit mode and starts the cursor blink.
func (m *SqlBarModel) Focus() tea.Cmd {
	m.focused = true
	return m.input.Focus()
}

// Blur takes the bar out of edit mode without clearing the value.
func (m *SqlBarModel) Blur() {
	m.focused = false
	m.input.Blur()
}

// IsFocused reports whether the bar currently owns keyboard input.
func (m SqlBarModel) IsFocused() bool { return m.focused }

// Value returns the current text content.
func (m SqlBarModel) Value() string { return m.input.Value() }

// SetValue overwrites the current text content (used after AI suggestions).
func (m *SqlBarModel) SetValue(v string) { m.input.SetValue(v) }

// Clear empties the bar's content.
func (m *SqlBarModel) Clear() { m.input.SetValue("") }

// SetWidth re-clamps the input width on terminal resize.
func (m *SqlBarModel) SetWidth(w int) {
	m.width = w
	m.input.Width = max(20, w-6)
}

// SetPlaceholder overrides the placeholder hint shown when the bar is empty.
// The App calls this when context shifts (e.g., entering a JOIN view) so
// the bar can advertise that filter clauses will be appended to the
// active query instead of replacing it.
func (m *SqlBarModel) SetPlaceholder(s string) { m.input.Placeholder = s }

// SetSchema feeds the autocomplete pools — table names, a deduped flat list
// of column names across all tables, plus the per-table column map used for
// scoped column completion (e.g. only columns of tables in FROM).
func (m *SqlBarModel) SetSchema(tables []string, columnsByTable map[string][]string) {
	m.tables = append(m.tables[:0], tables...)
	sort.Strings(m.tables)

	m.columnsByTable = make(map[string][]string, len(columnsByTable))
	for k, v := range columnsByTable {
		dup := make([]string, len(v))
		copy(dup, v)
		m.columnsByTable[k] = dup
	}

	seen := make(map[string]bool, 64)
	cols := make([]string, 0, 64)
	for _, list := range columnsByTable {
		for _, c := range list {
			if !seen[c] {
				seen[c] = true
				cols = append(cols, c)
			}
		}
	}
	sort.Strings(cols)
	m.columns = cols
}

// Update handles textinput key events when the bar is focused. Action keys
// (Enter / Esc) are intercepted by the App before this method runs. Tab is
// intercepted here to drive autocomplete cycling.
func (m SqlBarModel) Update(msg tea.Msg) (SqlBarModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "tab" {
			if m.completing && len(m.candidates) > 0 {
				m.candidateIx = (m.candidateIx + 1) % len(m.candidates)
				m.applyCandidate()
			} else {
				m.gatherCandidates()
				if len(m.candidates) > 0 {
					m.completing = true
					m.candidateIx = 0
					m.applyCandidate()
				}
			}
			return m, nil
		}
		// Any other key clears completion state.
		m.completing = false
		m.candidates = nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// gatherCandidates inspects the SQL up to the cursor (via CompletionContext)
// and assembles a context-filtered candidate list.
func (m *SqlBarModel) gatherCandidates() {
	val := m.input.Value()
	cursor := m.input.Position()

	info := CompletionContext(val, cursor)
	m.prefixStart = info.PrefixStart

	prefixLower := strings.ToLower(info.Prefix)

	seen := make(map[string]bool, 256)
	add := func(s string) {
		if s == "" {
			return
		}
		key := strings.ToLower(s)
		if seen[key] {
			return
		}
		seen[key] = true
		if prefixLower == "" || strings.HasPrefix(key, prefixLower) {
			m.candidates = append(m.candidates, s)
		}
	}

	switch info.Kind {
	case CompletionAfterFrom:
		for _, t := range m.tables {
			add(t)
		}
	case CompletionAfterSelect:
		// Scope columns to tables already mentioned in the query when possible.
		mentioned := info.MentionedTables
		if len(mentioned) > 0 {
			for _, tname := range mentioned {
				for _, c := range m.columnsByTable[tname] {
					add(c)
				}
			}
		}
		// Always offer common modifiers in this context.
		for _, kw := range []string{"DISTINCT", "AS", "FROM", "WHERE", "ORDER", "GROUP"} {
			add(kw)
		}
		// Fall back to all columns if scoping yielded nothing useful.
		if len(m.candidates) == 0 {
			for _, c := range m.columns {
				add(c)
			}
		}
	case CompletionStatementStart:
		for _, kw := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "EXPLAIN", "WITH", "BEGIN", "COMMIT", "ROLLBACK"} {
			add(kw)
		}
	default: // CompletionAny — combined pool, tables and columns first, keywords last.
		for _, t := range m.tables {
			add(t)
		}
		for _, c := range m.columns {
			add(c)
		}
		for _, kw := range commonSQLKeywords {
			add(kw)
		}
	}

	sort.SliceStable(m.candidates, func(i, j int) bool {
		return strings.ToLower(m.candidates[i]) < strings.ToLower(m.candidates[j])
	})
}

// CurrentCompletion returns metadata describing the active completion for
// status-bar feedback. Returns ("", 0, 0) when no completion is active.
func (m SqlBarModel) CurrentCompletion() (current string, idx, total int) {
	if !m.completing || len(m.candidates) == 0 {
		return "", 0, 0
	}
	return m.candidates[m.candidateIx], m.candidateIx + 1, len(m.candidates)
}

// applyCandidate replaces the partial word at the cursor with the currently
// selected candidate and re-anchors the cursor at the end of the inserted text.
func (m *SqlBarModel) applyCandidate() {
	val := m.input.Value()
	cursor := m.input.Position()
	if cursor > len(val) {
		cursor = len(val)
	}
	candidate := m.candidates[m.candidateIx]
	newVal := val[:m.prefixStart] + candidate + val[cursor:]
	m.input.SetValue(newVal)
	m.input.SetCursor(m.prefixStart + len(candidate))
}

// Height reports how many vertical lines this bar will occupy when rendered.
// Always 2 (input row + preview row) so the data viewer can budget for it
// regardless of focus state.
func (m SqlBarModel) Height() int { return 2 }

// View renders the bar — input row on top, syntax-highlighted preview below.
// When the input is empty and unfocused, the preview row is blank.
func (m SqlBarModel) View() string {
	inputLine := m.input.View()

	val := strings.TrimSpace(m.input.Value())
	previewLine := ""
	if val != "" {
		previewLine = "  " + HighlightSQL(m.input.Value())
	} else if m.focused {
		previewLine = "  " + StyleDim.Render("(empty)")
	}
	// Clip to width so a long line doesn't push the help bar off screen.
	if m.width > 0 && lipgloss.Width(previewLine) > m.width {
		// Truncate by character count — ANSI codes are short, this is approximate
		// but safe given lipgloss width handling.
		previewLine = lipgloss.NewStyle().MaxWidth(m.width).Render(previewLine)
	}

	return inputLine + "\n" + previewLine
}
