package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SQLEditorRunMsg is emitted when the user runs the editor's SQL.
type SQLEditorRunMsg struct{ SQL string }

// SQLEditorSaveViewMsg is emitted when the user wants to save the current
// SQL as a named view from inside the editor.
type SQLEditorSaveViewMsg struct{ SQL string }

// SQLEditorCancelMsg is emitted when the user closes the editor with Esc.
type SQLEditorCancelMsg struct{}

// SQLEditorModel is the full-screen multi-line SQL editor. It complements
// the bottom `:` bar (kept for one-liners and JOIN filter clauses) with a
// proper textarea, live syntax-highlighted preview, format-on-demand,
// schema-aware Tab autocomplete, and explicit run / save-view shortcuts.
type SQLEditorModel struct {
	area   textarea.Model
	width  int
	height int

	// Schema pools — fed by the App after introspection.
	tables         []string
	columns        []string
	columnsByTable map[string][]string

	// Active completion state. Reset on any non-Tab key press.
	completing  bool
	candidates  []string
	candidateIx int
	prefixStart int // absolute byte index in Value() where the partial word begins
}

// NewSQLEditorModel builds a fresh editor sized for the terminal.
func NewSQLEditorModel(width, height int) SQLEditorModel {
	ta := textarea.New()
	ta.Placeholder = "Type your SQL · Ctrl+L format · Ctrl+R run · Ctrl+S save view · Esc back"
	ta.Prompt = "│ "
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	ta.SetWidth(max(40, width-2))
	ta.SetHeight(max(8, height-8)) // leave room for preview + help/status
	ta.Focus()

	return SQLEditorModel{
		area:   ta,
		width:  width,
		height: height,
	}
}

// SetSize re-clamps the editor's dimensions on terminal resize.
func (m *SQLEditorModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.area.SetWidth(max(40, w-2))
	m.area.SetHeight(max(8, h-8))
}

// Value returns the current text content.
func (m SQLEditorModel) Value() string { return m.area.Value() }

// SetValue overwrites the editor's content.
func (m *SQLEditorModel) SetValue(s string) {
	m.area.SetValue(s)
	// Move the cursor to the end so the user starts editing where they
	// last left off rather than at (0,0).
	for i := 0; i < strings.Count(s, "\n"); i++ {
		m.area.CursorDown()
	}
	m.area.CursorEnd()
}

// SetSchema feeds the autocomplete pools.
func (m *SQLEditorModel) SetSchema(tables []string, columnsByTable map[string][]string) {
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

// Init satisfies tea.Model.
func (m SQLEditorModel) Init() tea.Cmd { return textarea.Blink }

// Update handles editor-specific keys (run / save / format / cancel /
// autocomplete) and forwards everything else to the textarea.
func (m SQLEditorModel) Update(msg tea.Msg) (SQLEditorModel, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			return m, func() tea.Msg { return SQLEditorCancelMsg{} }
		case "ctrl+r":
			return m, func() tea.Msg { return SQLEditorRunMsg{SQL: m.area.Value()} }
		case "ctrl+s":
			return m, func() tea.Msg { return SQLEditorSaveViewMsg{SQL: m.area.Value()} }
		case "ctrl+l":
			m.area.SetValue(FormatSQL(m.area.Value()))
			m.completing = false
			m.candidates = nil
			return m, nil
		case "tab":
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
		// Any other key resets completion state.
		m.completing = false
		m.candidates = nil
	}

	var cmd tea.Cmd
	m.area, cmd = m.area.Update(msg)
	return m, cmd
}

// gatherCandidates inspects the SQL up to the cursor and assembles a
// context-filtered candidate list. Reuses the same CompletionContext
// the inline SQL bar uses, so the discoverability rules are identical
// across both editors.
func (m *SQLEditorModel) gatherCandidates() {
	val := m.area.Value()
	cursor := m.absoluteCursor()
	if cursor > len(val) {
		cursor = len(val)
	}

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
		mentioned := info.MentionedTables
		if len(mentioned) > 0 {
			for _, tname := range mentioned {
				for _, c := range m.columnsByTable[tname] {
					add(c)
				}
			}
		}
		for _, kw := range []string{"DISTINCT", "AS", "FROM", "WHERE", "ORDER", "GROUP"} {
			add(kw)
		}
		if len(m.candidates) == 0 {
			for _, c := range m.columns {
				add(c)
			}
		}
	case CompletionStatementStart:
		for _, kw := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "EXPLAIN", "WITH", "BEGIN", "COMMIT", "ROLLBACK"} {
			add(kw)
		}
	default:
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

// applyCandidate replaces the partial word at the cursor with the active
// candidate, preserving the rest of the buffer.
func (m *SQLEditorModel) applyCandidate() {
	val := m.area.Value()
	cursor := m.absoluteCursor()
	if cursor > len(val) {
		cursor = len(val)
	}
	candidate := m.candidates[m.candidateIx]
	newVal := val[:m.prefixStart] + candidate + val[cursor:]
	m.area.SetValue(newVal)
	// Restore cursor to end of inserted candidate.
	target := m.prefixStart + len(candidate)
	m.moveCursorTo(newVal, target)
}

// absoluteCursor maps the textarea's (row, col) cursor to a byte index
// in the value string. Newline-terminated rows are counted with their
// trailing '\n'. Display-column to byte-index mapping is treated as 1:1
// — accurate for ASCII SQL, off for wide characters in identifiers
// (rare in practice).
func (m SQLEditorModel) absoluteCursor() int {
	val := m.area.Value()
	row := m.area.Line()
	col := m.area.LineInfo().ColumnOffset

	pos := 0
	cur := 0
	for i := 0; i < len(val); i++ {
		if cur == row {
			break
		}
		if val[i] == '\n' {
			cur++
		}
		pos++
	}
	pos += col
	if pos > len(val) {
		pos = len(val)
	}
	return pos
}

// moveCursorTo positions the textarea cursor at the given absolute byte
// index in val. Walks the buffer to derive (row, col).
func (m *SQLEditorModel) moveCursorTo(val string, target int) {
	if target < 0 {
		target = 0
	}
	if target > len(val) {
		target = len(val)
	}

	row, col := 0, 0
	for i := 0; i < target && i < len(val); i++ {
		if val[i] == '\n' {
			row++
			col = 0
		} else {
			col++
		}
	}
	// Re-anchor at the buffer start, then walk down to the target row.
	m.area.CursorStart()
	for i := 0; i < m.area.Line(); i++ {
		m.area.CursorUp()
	}
	for i := 0; i < row; i++ {
		m.area.CursorDown()
	}
	m.area.CursorStart()
	m.area.SetCursor(col)
}

// CurrentCompletion returns metadata describing the active completion for
// status-bar feedback. Returns ("", 0, 0) when no completion is active.
func (m SQLEditorModel) CurrentCompletion() (current string, idx, total int) {
	if !m.completing || len(m.candidates) == 0 {
		return "", 0, 0
	}
	return m.candidates[m.candidateIx], m.candidateIx + 1, len(m.candidates)
}

// View renders the editor — title bar + textarea + highlighted preview.
func (m SQLEditorModel) View() string {
	title := StyleTitle.Render("SQL Editor")
	body := m.area.View()

	val := strings.TrimSpace(m.area.Value())
	preview := ""
	if val != "" {
		preview = HighlightSQL(m.area.Value())
	} else {
		preview = StyleDim.Render("(preview appears here)")
	}
	previewBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(CtpOverlay0).
		Padding(0, 1).
		MaxWidth(m.width - 2).
		Render(preview)

	completionLine := ""
	if cur, idx, total := m.CurrentCompletion(); cur != "" {
		completionLine = StyleHelp.Render(
			"completion " +
				itoa(idx) + "/" + itoa(total) +
				": " + cur + "  — Tab cycles",
		)
	}

	parts := []string{title, body, previewBox}
	if completionLine != "" {
		parts = append(parts, completionLine)
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// itoa is a small helper to avoid importing strconv just for one int conversion.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
