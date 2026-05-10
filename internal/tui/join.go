package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gabiito/zdb/internal/db"
)

// JoinExecMsg is emitted when the wizard finishes successfully — the App
// should run this SQL via the standard query path. AppendedTable/Alias are
// populated when the wizard was in extend mode so the App can update its
// chain bookkeeping.
type JoinExecMsg struct {
	SQL            string
	AppendedTable  string
	AppendedAlias  string
}

// JoinCancelMsg is emitted when the user cancels the wizard.
type JoinCancelMsg struct{}

// JoinCache is what the wizard needs from the schema cache.
type JoinCache interface {
	Table(string) *db.Table
	Tables() []db.TableSummary
}

type joinStep int

const (
	joinStepRightTable joinStep = iota
	joinStepLeftCol
	joinStepRightCol
	joinStepOutputCols
)

type joinTableItem struct{ summary db.TableSummary }

func (i joinTableItem) Title() string       { return i.summary.Name }
func (i joinTableItem) Description() string { return fmt.Sprintf("%d cols", i.summary.ColCount) }
func (i joinTableItem) FilterValue() string { return i.summary.Name }

type joinColItem struct{ col db.Column }

func (i joinColItem) Title() string {
	pk := ""
	if i.col.IsPK {
		pk = " [PK]"
	}
	return i.col.Name + pk
}
func (i joinColItem) Description() string { return i.col.NativeType }
func (i joinColItem) FilterValue() string { return i.col.Name }

type joinOutputCol struct {
	qual    string // "a" (left) or "b" (right)
	name    string
	enabled bool
}

// JoinWizardModel is a 4-step modal: pick right table → pick left column →
// pick right column → trim output columns → emit JoinExecMsg.
//
// In `extend` mode the wizard skips step 4 and emits SQL that appends a
// JOIN clause to an existing query (carried in `extendPrefixSQL`). The
// LEFT side uses `extendLeftAlias` (the alias of leftTable in the prefix
// SQL) and the new RIGHT side gets `extendNextAlias`.
type JoinWizardModel struct {
	step      joinStep
	leftTable *db.Table

	rightTable *db.Table
	leftCol    string
	rightCol   string

	tableList    list.Model
	leftColList  list.Model
	rightColList list.Model

	outputs       []joinOutputCol
	outputCursor  int
	outputViewport viewport.Model

	width, height int
	cache         JoinCache

	// Extend-mode fields
	extend          bool
	extendPrefixSQL string
	extendLeftAlias string
	extendNextAlias string
	extendKeepsAll  bool // true when the prefix's SELECT clause is `*`
}

// NewJoinWizardModelExtend constructs the wizard in extend mode. The new
// JOIN clause is appended to prefixSQL using leftAlias for the LEFT side
// reference and nextAlias for the new RIGHT side.
//
// When the prefixSQL has `SELECT *`, step 4 (column trim) is skipped — the
// new table's columns flow through automatically. When the prefix has a
// specific column list, step 4 runs with ONLY the new table's columns so
// the user can pick which to splice into the existing SELECT clause.
func NewJoinWizardModelExtend(leftTable *db.Table, cache JoinCache, prefixSQL, leftAlias, nextAlias string, width, height int) JoinWizardModel {
	m := NewJoinWizardModel(leftTable, cache, width, height)
	m.extend = true
	m.extendPrefixSQL = prefixSQL
	m.extendLeftAlias = leftAlias
	m.extendNextAlias = nextAlias
	m.extendKeepsAll = sqlSelectIsAll(prefixSQL)
	return m
}

// NewJoinWizardModel constructs a fresh wizard. leftTable is the table the
// user is currently viewing — that table is excluded from the right-table
// picker.
func NewJoinWizardModel(leftTable *db.Table, cache JoinCache, width, height int) JoinWizardModel {
	tables := cache.Tables()
	items := make([]list.Item, 0, len(tables))
	for _, t := range tables {
		if t.Name == leftTable.Name {
			continue
		}
		items = append(items, joinTableItem{summary: t})
	}

	listW, listH := joinListDims(width, height)

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(CtpPink)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(CtpPink)

	tableList := list.New(items, delegate, listW, listH)
	tableList.Title = "Pick the table to JOIN with"
	tableList.Styles.Title = StyleTitle
	tableList.SetShowStatusBar(false)
	tableList.SetShowHelp(false)

	return JoinWizardModel{
		step:      joinStepRightTable,
		leftTable: leftTable,
		cache:     cache,
		tableList: tableList,
		width:     width,
		height:    height,
	}
}

func joinListDims(termW, termH int) (int, int) {
	w := termW - 12
	if w < 40 {
		w = 40
	}
	if w > 90 {
		w = 90
	}
	h := termH - 14
	if h < 6 {
		h = 6
	}
	if h > 20 {
		h = 20
	}
	return w, h
}

func newColList(title string, cols []db.Column, w, h int) list.Model {
	items := make([]list.Item, len(cols))
	for i, c := range cols {
		items[i] = joinColItem{col: c}
	}
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(CtpPink)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(CtpPink)

	l := list.New(items, delegate, w, h)
	l.Title = title
	l.Styles.Title = StyleTitle
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	return l
}

// Update advances the wizard. It returns a JoinExecMsg cmd when the user
// confirms the final step.
func (m JoinWizardModel) Update(msg tea.Msg) (JoinWizardModel, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyMsg)

	if isKey {
		switch keyMsg.String() {
		case "esc":
			return m, func() tea.Msg { return JoinCancelMsg{} }
		case "backspace":
			// Step back one (no-op on the first step).
			if m.step > joinStepRightTable {
				m.step--
			}
			return m, nil
		}

		switch m.step {
		case joinStepRightTable:
			if keyMsg.String() == "enter" {
				if item, ok := m.tableList.SelectedItem().(joinTableItem); ok {
					if rt := m.cache.Table(item.summary.Name); rt != nil {
						m.rightTable = rt
						listW, listH := joinListDims(m.width, m.height)
						m.leftColList = newColList(
							"Pick column from "+m.leftTable.Name+" (LEFT side of join)",
							m.leftTable.Columns, listW, listH)
						m.step = joinStepLeftCol
					}
				}
				return m, nil
			}
		case joinStepLeftCol:
			if keyMsg.String() == "enter" {
				if item, ok := m.leftColList.SelectedItem().(joinColItem); ok {
					m.leftCol = item.col.Name
					listW, listH := joinListDims(m.width, m.height)
					m.rightColList = newColList(
						"Pick column from "+m.rightTable.Name+" (RIGHT side of join)",
						m.rightTable.Columns, listW, listH)
					m.step = joinStepRightCol
				}
				return m, nil
			}
		case joinStepRightCol:
			if keyMsg.String() == "enter" {
				if item, ok := m.rightColList.SelectedItem().(joinColItem); ok {
					m.rightCol = item.col.Name
					if m.extend && m.extendKeepsAll {
						// Existing SELECT is `*` — new cols come through automatically.
						sql := buildExtendJoinSQL(
							m.extendPrefixSQL,
							m.rightTable.Name, m.extendNextAlias,
							m.extendLeftAlias, m.leftCol, m.rightCol,
						)
						return m, func() tea.Msg {
							return JoinExecMsg{SQL: sql, AppendedTable: m.rightTable.Name, AppendedAlias: m.extendNextAlias}
						}
					}
					if m.extend {
						// Existing SELECT had specific cols — let the user pick which
						// of the NEW table's columns to splice into the SELECT.
						m.outputs = make([]joinOutputCol, 0, len(m.rightTable.Columns))
						for _, c := range m.rightTable.Columns {
							m.outputs = append(m.outputs, joinOutputCol{
								qual: m.extendNextAlias, name: c.Name, enabled: true,
							})
						}
					} else {
						m.outputs = buildJoinOutputs(m.leftTable, m.rightTable)
					}
					m.outputCursor = 0
					_, listH := joinListDims(m.width, m.height)
					m.outputViewport = viewport.New(60, listH)
					m.outputViewport.SetContent(renderOutputList(m.outputs, m.outputCursor))
					m.step = joinStepOutputCols
				}
				return m, nil
			}
		case joinStepOutputCols:
			switch keyMsg.String() {
			case "up", "k":
				if m.outputCursor > 0 {
					m.outputCursor--
				}
			case "down", "j":
				if m.outputCursor < len(m.outputs)-1 {
					m.outputCursor++
				}
			case " ":
				if m.outputCursor < len(m.outputs) {
					m.outputs[m.outputCursor].enabled = !m.outputs[m.outputCursor].enabled
				}
			case "a":
				for i := range m.outputs {
					m.outputs[i].enabled = true
				}
			case "n":
				for i := range m.outputs {
					m.outputs[i].enabled = false
				}
			case "enter":
				if m.extend {
					var newCols []string
					for _, o := range m.outputs {
						if o.enabled {
							newCols = append(newCols, o.qual+"."+o.name)
						}
					}
					sql := buildExtendJoinSQLWithCols(
						m.extendPrefixSQL, newCols,
						m.rightTable.Name, m.extendNextAlias,
						m.extendLeftAlias, m.leftCol, m.rightCol,
					)
					return m, func() tea.Msg {
						return JoinExecMsg{SQL: sql, AppendedTable: m.rightTable.Name, AppendedAlias: m.extendNextAlias}
					}
				}
				sql := buildJoinSQL(m.leftTable, m.rightTable, m.leftCol, m.rightCol, m.outputs)
				return m, func() tea.Msg { return JoinExecMsg{SQL: sql} }
			}
			m.outputViewport.SetContent(renderOutputList(m.outputs, m.outputCursor))
			m.ensureOutputCursorVisible()
			return m, nil
		}
	}

	// Forward to the active sub-list for steps 1-3.
	var cmd tea.Cmd
	switch m.step {
	case joinStepRightTable:
		m.tableList, cmd = m.tableList.Update(msg)
	case joinStepLeftCol:
		m.leftColList, cmd = m.leftColList.Update(msg)
	case joinStepRightCol:
		m.rightColList, cmd = m.rightColList.Update(msg)
	case joinStepOutputCols:
		m.outputViewport, cmd = m.outputViewport.Update(msg)
	}
	return m, cmd
}

// View renders the wizard's bordered box for the active step. Now includes:
// (a) a persistent "selections so far" header so the user always sees what
// they picked, (b) a live SQL preview at the bottom built from current
// state, (c) the step-specific picker in the middle.
func (m JoinWizardModel) View() string {
	header := m.selectionsHeader()
	preview := m.livePreview()

	var inner, hint string
	switch m.step {
	case joinStepRightTable:
		inner = m.tableList.View()
		hint = "Step 1/4 · Enter pick table · Esc cancel"
	case joinStepLeftCol:
		inner = m.leftColList.View()
		hint = "Step 2/4 · Enter pick column · Backspace back · Esc cancel"
	case joinStepRightCol:
		inner = m.rightColList.View()
		hint = "Step 3/4 · Enter pick column · Backspace back · Esc cancel"
	case joinStepOutputCols:
		var title string
		if m.extend {
			title = StyleTitle.Render(fmt.Sprintf("New columns from %s (%d/%d enabled)", m.rightTable.Name, countEnabled(m.outputs), len(m.outputs)))
		} else {
			title = StyleTitle.Render(fmt.Sprintf("Output columns (%d/%d enabled)", countEnabled(m.outputs), len(m.outputs)))
		}
		inner = title + "\n\n" + m.outputViewport.View()
		hint = "Step 4/4 · Space toggle · a all · n none · Enter run · Backspace back · Esc cancel"
	}

	boxW := m.width - 8
	if boxW < 50 {
		boxW = 50
	}
	if boxW > 100 {
		boxW = 100
	}

	divider := lipgloss.NewStyle().
		Foreground(CtpOverlay0).
		Render(strings.Repeat("─", boxW-6))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(CtpBlue).
		Padding(1, 2).
		Width(boxW).
		Render(
			StyleTitle.Render("⚷ Join wizard") + "\n\n" +
				header + "\n" +
				divider + "\n\n" +
				inner + "\n\n" +
				divider + "\n" +
				preview + "\n\n" +
				StyleHelp.Render(hint),
		)
}

// selectionsHeader renders the running "what have I picked so far?" summary.
// Done state items are bright; pending ones dimmed with a gray bullet.
func (m JoinWizardModel) selectionsHeader() string {
	rightTable := pendingMark()
	if m.rightTable != nil {
		rightTable = doneMark() + " " + StyleTitle.Render(m.rightTable.Name)
	} else {
		rightTable += " " + StyleDim.Render("(picking…)")
	}

	leftCol := pendingMark()
	switch {
	case m.leftCol != "":
		leftCol = doneMark() + " " + StyleTitle.Render(m.leftTable.Name+"."+m.leftCol)
	case m.step == joinStepLeftCol:
		leftCol += " " + StyleDim.Render("(picking…)")
	default:
		leftCol += " " + StyleDim.Render("pending")
	}

	rightCol := pendingMark()
	switch {
	case m.rightCol != "":
		rightCol = doneMark() + " " + StyleTitle.Render(m.rightTable.Name+"."+m.rightCol)
	case m.step == joinStepRightCol:
		rightCol += " " + StyleDim.Render("(picking…)")
	default:
		rightCol += " " + StyleDim.Render("pending")
	}

	output := pendingMark()
	switch {
	case len(m.outputs) > 0 && m.step == joinStepOutputCols:
		output = doneMark() + " " + StyleTitle.Render(fmt.Sprintf("%d/%d cols", countEnabled(m.outputs), len(m.outputs)))
	default:
		output += " " + StyleDim.Render("pending")
	}

	return strings.Join([]string{
		StyleDim.Render("LEFT  ") + "  " + StyleTitle.Render(m.leftTable.Name) + StyleDim.Render(" (the table you opened)"),
		StyleDim.Render("RIGHT ") + "  " + rightTable,
		StyleDim.Render("ON L  ") + "  " + leftCol,
		StyleDim.Render("ON R  ") + "  " + rightCol,
		StyleDim.Render("COLS  ") + "  " + output,
	}, "\n")
}

func doneMark() string    { return lipgloss.NewStyle().Foreground(CtpGreen).Bold(true).Render("✓") }
func pendingMark() string { return lipgloss.NewStyle().Foreground(CtpOverlay0).Render("○") }

// livePreview renders the SQL the wizard would emit if Enter were pressed
// right now. Pieces still pending render as dim placeholders.
func (m JoinWizardModel) livePreview() string {
	leftName := m.leftTable.Name
	rightName := "<table>"
	if m.rightTable != nil {
		rightName = m.rightTable.Name
	}
	leftCol := "<col>"
	if m.leftCol != "" {
		leftCol = m.leftCol
	}
	rightCol := "<col>"
	if m.rightCol != "" {
		rightCol = m.rightCol
	}
	cols := "*"
	if len(m.outputs) > 0 {
		picks := make([]string, 0, len(m.outputs))
		for _, o := range m.outputs {
			if o.enabled {
				picks = append(picks, o.qual+"."+o.name)
			}
		}
		if len(picks) > 0 {
			cols = strings.Join(picks, ", ")
		}
	}
	sql := fmt.Sprintf("SELECT %s\nFROM %s a JOIN %s b ON a.%s = b.%s",
		cols, leftName, rightName, leftCol, rightCol)
	return StyleDim.Render("Preview:") + "\n" + HighlightSQL(sql)
}

func (m *JoinWizardModel) ensureOutputCursorVisible() {
	// Scroll viewport to keep the cursor in view.
	if m.outputCursor < m.outputViewport.YOffset {
		m.outputViewport.SetYOffset(m.outputCursor)
	} else if m.outputCursor >= m.outputViewport.YOffset+m.outputViewport.Height {
		m.outputViewport.SetYOffset(m.outputCursor - m.outputViewport.Height + 1)
	}
}

func buildJoinOutputs(left, right *db.Table) []joinOutputCol {
	out := make([]joinOutputCol, 0, len(left.Columns)+len(right.Columns))
	for _, c := range left.Columns {
		out = append(out, joinOutputCol{qual: "a", name: c.Name, enabled: true})
	}
	for _, c := range right.Columns {
		out = append(out, joinOutputCol{qual: "b", name: c.Name, enabled: true})
	}
	return out
}

func renderOutputList(outputs []joinOutputCol, cursor int) string {
	var sb strings.Builder
	for i, o := range outputs {
		check := "[ ]"
		if o.enabled {
			check = "[x]"
		}
		line := fmt.Sprintf("%s %s.%s", check, o.qual, o.name)
		if i == cursor {
			line = StyleSelectedRow.Render("→ " + line)
		} else {
			line = "  " + line
		}
		sb.WriteString(line + "\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func countEnabled(outputs []joinOutputCol) int {
	n := 0
	for _, o := range outputs {
		if o.enabled {
			n++
		}
	}
	return n
}

// buildExtendJoinSQL appends a JOIN clause to an existing SELECT. Naive
// — assumes the prefix has no WHERE/GROUP/ORDER/LIMIT clauses (the wizard's
// own output, not user-modified). If the user added clauses the resulting
// SQL will be malformed; that's a v2 concern.
func buildExtendJoinSQL(prefix, rightTable, nextAlias, leftAlias, leftCol, rightCol string) string {
	prefix = strings.TrimRight(prefix, " ;\n\t")
	return fmt.Sprintf("%s JOIN %s %s ON %s.%s = %s.%s",
		prefix, rightTable, nextAlias, leftAlias, leftCol, nextAlias, rightCol)
}

// buildExtendJoinSQLWithCols both injects the new table's selected columns
// into the existing SELECT clause AND appends the new JOIN. Used when the
// prefix had a specific column list (not `*`), so that the new table's
// values actually appear in the result.
func buildExtendJoinSQLWithCols(prefix string, newCols []string, rightTable, nextAlias, leftAlias, leftCol, rightCol string) string {
	prefix = strings.TrimRight(prefix, " ;\n\t")
	withCols := injectIntoSelectClause(prefix, newCols)
	return fmt.Sprintf("%s JOIN %s %s ON %s.%s = %s.%s",
		withCols, rightTable, nextAlias, leftAlias, leftCol, nextAlias, rightCol)
}

// injectIntoSelectClause splices additional column expressions just before
// the ` FROM ` keyword. Empty newCols is a no-op.
func injectIntoSelectClause(sql string, newCols []string) string {
	if len(newCols) == 0 {
		return sql
	}
	upper := strings.ToUpper(sql)
	idx := strings.Index(upper, " FROM ")
	if idx < 0 {
		return sql
	}
	selectClause := strings.TrimRight(sql[:idx], " \t,")
	rest := sql[idx:]
	return selectClause + ", " + strings.Join(newCols, ", ") + rest
}

// sqlSelectIsAll reports whether the SQL string is of the form
// `SELECT * FROM …`. Used to decide whether to skip the column-trim step
// in extend mode.
func sqlSelectIsAll(sql string) bool {
	trimmed := strings.TrimSpace(sql)
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "SELECT") {
		return false
	}
	body := strings.TrimSpace(trimmed[len("SELECT"):])
	upBody := strings.ToUpper(body)
	idx := strings.Index(upBody, " FROM ")
	if idx < 0 {
		return false
	}
	clause := strings.TrimSpace(body[:idx])
	return clause == "*"
}

func buildJoinSQL(left, right *db.Table, leftCol, rightCol string, outputs []joinOutputCol) string {
	cols := make([]string, 0, len(outputs))
	for _, o := range outputs {
		if o.enabled {
			cols = append(cols, o.qual+"."+o.name)
		}
	}
	selectClause := "*"
	if len(cols) > 0 {
		selectClause = strings.Join(cols, ", ")
	}
	return fmt.Sprintf(
		"SELECT %s FROM %s a JOIN %s b ON a.%s = b.%s",
		selectClause, left.Name, right.Name, leftCol, rightCol,
	)
}
