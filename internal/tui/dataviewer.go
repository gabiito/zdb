package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/gabiito/zdb/internal/db"
)

// DataViewerModel renders a table with a real cell cursor (row + column).
// Navigation: h/j/k/l + arrows for cell movement, g/G top/bottom of resultset,
// Ctrl+f/b for page, 0/$ for first/last column. When the resultset has more
// columns than fit in the current width, the visible window scrolls
// horizontally to keep the cursor on screen.
type DataViewerModel struct {
	tableRef  *db.Table
	resultSet *db.ResultSet

	// Cell cursor (indexes into resultSet, not into the visible page).
	selectedRow int
	selectedCol int

	// Index of the leftmost column currently rendered. Used to scroll
	// horizontally when there are more columns than fit on screen.
	firstVisibleCol int

	pageSize int
	width    int
	height   int

	// Cell view state (for the 'v' modal)
	cellViewActive  bool
	cellViewContent string

	// Optional legend rendered above the table when the data comes from a
	// JOIN. Set by the App from its joinChain state.
	legend string

	// Optional per-column prefix (typically a JOIN alias). Same length as
	// resultSet.Columns when set; nil disables prefixing.
	colPrefixes []string

	// Marked rows for multi-row copy. Keys are absolute indexes into
	// resultSet.Rows. Empty map means no marks.
	markedRows map[int]bool

	// dbOffset is the offset of the first loaded row within the full
	// result set on the database side. The App owns the canonical value;
	// the viewer holds it only to render the absolute row range in the
	// status line.
	dbOffset int

	// markAnchor is the row that anchors a Shift+Space range mark — the
	// last row toggled with plain Space. -1 means no anchor exists yet.
	markAnchor int

	// totalRows is the total row count of the underlying table, fetched
	// once via COUNT(*) when the table is opened. -1 means "unknown" and
	// suppresses the "loaded/total" suffix in the status line — used for
	// derived results from raw SQL or while the count is in flight.
	totalRows int
}

// NewDataViewerModel creates a DataViewerModel for the given table.
func NewDataViewerModel(t *db.Table, pageSize, width, height int) DataViewerModel {
	return DataViewerModel{
		tableRef:   t,
		pageSize:   pageSize,
		width:      width,
		height:     height,
		markAnchor: -1,
		totalRows:  -1,
	}
}

// SetHeight updates the available height for the table. The visible-rows
// count is derived from this so the table never overflows the screen.
func (m *DataViewerModel) SetHeight(h int) { m.height = h }

// SetWidth updates the available width for the table and re-clamps the
// horizontal scroll window so the cursor stays visible.
func (m *DataViewerModel) SetWidth(w int) {
	m.width = w
	m.adjustHScroll()
}

// SetLegend sets the inline legend rendered above the table. Empty string
// removes it.
func (m *DataViewerModel) SetLegend(s string) { m.legend = s }

// SetColumnPrefixes sets per-column prefixes (e.g., JOIN aliases). Pass nil
// or a slice of a different length than the resultSet columns to disable.
func (m *DataViewerModel) SetColumnPrefixes(prefixes []string) { m.colPrefixes = prefixes }

// ToggleMark toggles the mark on the row currently under the cursor and
// sets it as the anchor for the next Shift+Space range mark. No-op when
// there's no result set or the cursor is out of range.
func (m *DataViewerModel) ToggleMark() {
	if m.resultSet == nil || len(m.resultSet.Rows) == 0 {
		return
	}
	if m.markedRows == nil {
		m.markedRows = make(map[int]bool)
	}
	if m.markedRows[m.selectedRow] {
		delete(m.markedRows, m.selectedRow)
	} else {
		m.markedRows[m.selectedRow] = true
	}
	m.markAnchor = m.selectedRow
}

// MarkRange marks every row between the current anchor and the cursor
// (inclusive). When no anchor exists, falls back to a plain mark on the
// current row. The anchor is left untouched so the user can extend the
// range further from the same starting point.
func (m *DataViewerModel) MarkRange() {
	if m.resultSet == nil || len(m.resultSet.Rows) == 0 {
		return
	}
	if m.markedRows == nil {
		m.markedRows = make(map[int]bool)
	}
	if m.markAnchor < 0 {
		m.markedRows[m.selectedRow] = true
		m.markAnchor = m.selectedRow
		return
	}
	a, b := m.markAnchor, m.selectedRow
	if a > b {
		a, b = b, a
	}
	if a < 0 {
		a = 0
	}
	if b >= len(m.resultSet.Rows) {
		b = len(m.resultSet.Rows) - 1
	}
	for r := a; r <= b; r++ {
		m.markedRows[r] = true
	}
}

// HasMarks reports whether any rows are currently marked.
func (m DataViewerModel) HasMarks() bool { return len(m.markedRows) > 0 }

// MarkCount returns the number of marked rows.
func (m DataViewerModel) MarkCount() int { return len(m.markedRows) }

// ClearMarks removes all row marks and resets the range anchor.
func (m *DataViewerModel) ClearMarks() {
	m.markedRows = nil
	m.markAnchor = -1
}

// MoveToTop puts the cursor on the first loaded row. Used after fetching
// the next DB page so the user lands on the first new row.
func (m *DataViewerModel) MoveToTop() {
	m.selectedRow = 0
}

// MoveToBottom puts the cursor on the last loaded row. Used after fetching
// the previous DB page so a backward Ctrl+b lands the user at the natural
// continuation point.
func (m *DataViewerModel) MoveToBottom() {
	if m.resultSet != nil && len(m.resultSet.Rows) > 0 {
		m.selectedRow = len(m.resultSet.Rows) - 1
	}
}

// SetDBOffset records the offset of the loaded buffer within the full
// result set. The App calls this when paginating so the status line
// shows the absolute row range.
func (m *DataViewerModel) SetDBOffset(offset int) {
	m.dbOffset = offset
}

// SetTotalRows records the table's full row count so the status line can
// show "loaded N / total T". Pass -1 to clear (e.g., when switching to a
// derived result set).
func (m *DataViewerModel) SetTotalRows(total int) {
	m.totalRows = total
}

// TotalRows returns the recorded total row count, or -1 when unknown.
func (m DataViewerModel) TotalRows() int { return m.totalRows }

// LoadedRowCount returns the number of rows currently in the buffer.
// Used by the App to compute the offset for the next page fetch when the
// buffer has been extended by infinite-scroll appends.
func (m DataViewerModel) LoadedRowCount() int {
	if m.resultSet == nil {
		return 0
	}
	return len(m.resultSet.Rows)
}

// AppendRows extends the current buffer with the rows from rs (which is
// expected to share the same column shape). Cursor and marks stay where
// they were — appending never shifts existing rows. No-op when there is
// no current result set or rs is empty.
func (m *DataViewerModel) AppendRows(rs *db.ResultSet) {
	if rs == nil || m.resultSet == nil {
		return
	}
	m.resultSet.Rows = append(m.resultSet.Rows, rs.Rows...)
}

// SetCursorRow places the cursor at the given absolute row, clamped to
// the loaded range. Used by the App after an append fetch to advance the
// cursor onto the first newly-loaded row.
func (m *DataViewerModel) SetCursorRow(row int) {
	if m.resultSet == nil || len(m.resultSet.Rows) == 0 {
		return
	}
	if row < 0 {
		row = 0
	}
	if row >= len(m.resultSet.Rows) {
		row = len(m.resultSet.Rows) - 1
	}
	m.selectedRow = row
}

// MarkedRows returns the marked rows in ascending order by index. Returns nil
// when no marks are set.
func (m DataViewerModel) MarkedRows() []int {
	if len(m.markedRows) == 0 {
		return nil
	}
	out := make([]int, 0, len(m.markedRows))
	for i := range m.markedRows {
		out = append(out, i)
	}
	// Insertion sort — N is small (page-bounded user marks).
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// ColumnNames returns all column names in the current result set, in order.
// Empty slice when no result set is loaded.
func (m DataViewerModel) ColumnNames() []string {
	if m.resultSet == nil {
		return nil
	}
	names := make([]string, len(m.resultSet.Columns))
	for i, c := range m.resultSet.Columns {
		names[i] = c.Name
	}
	return names
}

// RowValues returns the formatted string values for the row at the given
// absolute index. Returns nil when out of range.
func (m DataViewerModel) RowValues(row int) []string {
	if m.resultSet == nil || row < 0 || row >= len(m.resultSet.Rows) {
		return nil
	}
	r := m.resultSet.Rows[row]
	out := make([]string, len(m.resultSet.Columns))
	for j := range m.resultSet.Columns {
		if j < len(r.Cells) {
			out[j] = formatCellValue(r.Cells[j])
		}
	}
	return out
}

// ResultSetColumnCount returns the number of columns in the current result set.
func (m DataViewerModel) ResultSetColumnCount() int {
	if m.resultSet == nil {
		return 0
	}
	return len(m.resultSet.Columns)
}

// colsPerScreen is the number of columns that fit in the current width with
// a minimum sensible per-column budget. Always ≥ 1.
func (m DataViewerModel) colsPerScreen() int {
	if m.resultSet == nil || len(m.resultSet.Columns) == 0 {
		return 1
	}
	const perCol = 18 // approx min content width per column incl. padding/border
	fit := (m.width - 4) / perCol
	if fit < 1 {
		fit = 1
	}
	if fit > len(m.resultSet.Columns) {
		fit = len(m.resultSet.Columns)
	}
	return fit
}

// adjustHScroll keeps the selected column within the visible window after a
// cursor move or width change. Idempotent.
func (m *DataViewerModel) adjustHScroll() {
	if m.resultSet == nil {
		m.firstVisibleCol = 0
		return
	}
	total := len(m.resultSet.Columns)
	if total == 0 {
		m.firstVisibleCol = 0
		return
	}
	cps := m.colsPerScreen()
	if cps >= total {
		m.firstVisibleCol = 0
		return
	}
	if m.selectedCol < m.firstVisibleCol {
		m.firstVisibleCol = m.selectedCol
	}
	if m.selectedCol >= m.firstVisibleCol+cps {
		m.firstVisibleCol = m.selectedCol - cps + 1
	}
	if m.firstVisibleCol < 0 {
		m.firstVisibleCol = 0
	}
	if m.firstVisibleCol > total-cps {
		m.firstVisibleCol = total - cps
	}
}

// visiblePageRows returns how many data rows fit in the current height,
// budgeting space for the table's borders, header, divider, status line,
// the optional banner above the table, and the optional join legend line.
func (m DataViewerModel) visiblePageRows() int {
	chrome := 6 // top border + header + divider + bottom border + status line + buffer
	if m.tableRef == nil || len(m.tableRef.PKCols) == 0 {
		chrome++ // derived-results or read-only banner
	}
	if m.legend != "" {
		chrome++ // legend line
	}
	n := m.height - chrome
	if n < 1 {
		if m.pageSize > 0 {
			return m.pageSize
		}
		return 1
	}
	return n
}

// SetData replaces the displayed data. The cursor is clamped to the new
// bounds so a refresh after edit/commit doesn't yank the user back to (0, 0).
// Horizontal scroll is reset to the leftmost column.
func (m *DataViewerModel) SetData(rs *db.ResultSet) {
	if rs == nil {
		return
	}
	m.resultSet = rs
	if m.selectedRow >= len(rs.Rows) {
		m.selectedRow = len(rs.Rows) - 1
	}
	if m.selectedRow < 0 {
		m.selectedRow = 0
	}
	if m.selectedCol >= len(rs.Columns) {
		m.selectedCol = len(rs.Columns) - 1
	}
	if m.selectedCol < 0 {
		m.selectedCol = 0
	}
	m.firstVisibleCol = 0
	m.markedRows = nil
	m.markAnchor = -1
	m.adjustHScroll()
}

// Init implements tea.Model.
func (m DataViewerModel) Init() tea.Cmd { return nil }

// Update handles cell-cursor navigation. All non-navigation keys are no-ops
// here and bubble up to the App for action handling.
func (m DataViewerModel) Update(msg tea.Msg) (DataViewerModel, tea.Cmd) {
	if m.resultSet == nil {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	nrows := len(m.resultSet.Rows)
	ncols := len(m.resultSet.Columns)
	if nrows == 0 || ncols == 0 {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.selectedRow > 0 {
			m.selectedRow--
		}
	case "down", "j":
		if m.selectedRow < nrows-1 {
			m.selectedRow++
		} else {
			// At the last loaded row — infinite-scroll: request more rows
			// to be appended to the buffer.
			return m, func() tea.Msg { return WantNextPageAppendMsg{} }
		}
	case "left", "h":
		if m.selectedCol > 0 {
			m.selectedCol--
		}
		m.adjustHScroll()
	case "right", "l":
		if m.selectedCol < ncols-1 {
			m.selectedCol++
		}
		m.adjustHScroll()
	case "g", "home":
		m.selectedRow = 0
	case "G", "end":
		m.selectedRow = nrows - 1
	case "ctrl+f", "pgdown":
		// At the last loaded row, ask the App to fetch the next DB page.
		if m.selectedRow == nrows-1 {
			return m, func() tea.Msg { return WantNextPageMsg{} }
		}
		m.selectedRow += m.visiblePageRows()
		if m.selectedRow >= nrows {
			m.selectedRow = nrows - 1
		}
	case "ctrl+b", "pgup":
		// At the first loaded row, ask the App to fetch the previous DB page.
		if m.selectedRow == 0 {
			return m, func() tea.Msg { return WantPrevPageMsg{} }
		}
		m.selectedRow -= m.visiblePageRows()
		if m.selectedRow < 0 {
			m.selectedRow = 0
		}
	case "0":
		m.selectedCol = 0
		m.adjustHScroll()
	case "$":
		m.selectedCol = ncols - 1
		m.adjustHScroll()
	}
	return m, nil
}

// UpdateCellView handles messages while the cell-view modal is open.
func (m DataViewerModel) UpdateCellView(msg tea.Msg) (DataViewerModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc", "q":
			m.cellViewActive = false
		}
	}
	return m, nil
}

// View renders the data table with cell-cursor highlighting.
func (m DataViewerModel) View() string {
	if m.resultSet == nil {
		if m.tableRef == nil {
			return StyleDim.Render("No table selected — type SQL or open a table")
		}
		return StyleDim.Render("Loading...")
	}

	var sb strings.Builder

	switch {
	case m.tableRef == nil:
		sb.WriteString(StyleBannerYellow.Render("DERIVED RESULTS: edits/deletes are disabled") + "\n")
	case len(m.tableRef.PKCols) == 0:
		sb.WriteString(StyleBannerRed.Render("READ-ONLY: table has no primary key") + "\n")
	}

	if m.legend != "" {
		sb.WriteString(StyleHelp.Render("Tables: "+m.legend) + "\n")
	}

	pageRows := m.visiblePageRows()
	pageStart := (m.selectedRow / pageRows) * pageRows
	pageEnd := pageStart + pageRows
	if pageEnd > len(m.resultSet.Rows) {
		pageEnd = len(m.resultSet.Rows)
	}
	visibleRows := m.resultSet.Rows[pageStart:pageEnd]
	selectedRowInPage := m.selectedRow - pageStart

	totalCols := len(m.resultSet.Columns)
	firstCol := m.firstVisibleCol
	lastCol := firstCol + m.colsPerScreen()
	if lastCol > totalCols {
		lastCol = totalCols
	}

	headers := make([]string, 0, lastCol-firstCol)
	prefixesValid := len(m.colPrefixes) == len(m.resultSet.Columns)
	for i := firstCol; i < lastCol; i++ {
		name := m.resultSet.Columns[i].Name
		if prefixesValid && m.colPrefixes[i] != "" {
			name = m.colPrefixes[i] + ":" + name
		}
		headers = append(headers, name)
	}

	rowsData := make([][]string, len(visibleRows))
	for i, r := range visibleRows {
		cells := make([]string, 0, lastCol-firstCol)
		for j := firstCol; j < lastCol; j++ {
			if j < len(r.Cells) {
				cells = append(cells, formatCellValue(r.Cells[j]))
			} else {
				cells = append(cells, "")
			}
		}
		rowsData[i] = cells
	}

	cellPadding := lipgloss.NewStyle().Padding(0, 1)
	headerStyle := StyleTitle.Padding(0, 1)
	rowHighlight := StyleSelectedRow.Padding(0, 1)
	cellHighlight := StyleSelectedCell.Padding(0, 1)
	markedHighlight := StyleMarkedRow.Padding(0, 1)

	visibleSelectedCol := m.selectedCol - firstCol

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(CtpSurface2)).
		Headers(headers...).
		Rows(rowsData...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			if row == selectedRowInPage && col == visibleSelectedCol {
				return cellHighlight
			}
			if row == selectedRowInPage {
				return rowHighlight
			}
			if m.markedRows[pageStart+row] {
				return markedHighlight
			}
			return cellPadding
		})

	if m.width > 0 {
		t = t.Width(m.width)
	}

	sb.WriteString(t.Render())

	pageNum := pageStart/pageRows + 1
	totalPages := (len(m.resultSet.Rows) + pageRows - 1) / pageRows
	if totalPages == 0 {
		totalPages = 1
	}
	colName := ""
	if m.selectedCol < totalCols {
		colName = m.resultSet.Columns[m.selectedCol].Name
	}
	colWindow := ""
	if firstCol > 0 || lastCol < totalCols {
		hasLeft := "  "
		if firstCol > 0 {
			hasLeft = "‹ "
		}
		hasRight := "  "
		if lastCol < totalCols {
			hasRight = " ›"
		}
		colWindow = fmt.Sprintf(" %s[%d–%d/%d]%s", hasLeft, firstCol+1, lastCol, totalCols, hasRight)
	}
	sb.WriteString("\n")
	marksSuffix := ""
	if n := len(m.markedRows); n > 0 {
		marksSuffix = fmt.Sprintf(" · %d marked", n)
	}
	offsetSuffix := ""
	if m.dbOffset > 0 {
		offsetSuffix = fmt.Sprintf(" · DB offset %d", m.dbOffset)
	}
	loadedSuffix := ""
	if m.totalRows >= 0 {
		loadedSuffix = fmt.Sprintf(" · Loaded %d/%d", len(m.resultSet.Rows), m.totalRows)
	}
	sb.WriteString(StyleHelp.Render(fmt.Sprintf(
		"Row %d/%d · Page %d/%d · Col %d/%d: %s%s%s%s%s",
		m.selectedRow+1, len(m.resultSet.Rows),
		pageNum, totalPages,
		m.selectedCol+1, totalCols, colName, colWindow, loadedSuffix, marksSuffix, offsetSuffix,
	)))

	return sb.String()
}

// Table returns the schema table reference.
func (m DataViewerModel) Table() *db.Table { return m.tableRef }

// SelectedCell returns the cursor position. The first return value is the row
// index (in resultSet), the second is the column index, and the third reports
// whether a valid cell is selected.
func (m DataViewerModel) SelectedCell() (int, int, bool) {
	if m.resultSet == nil || len(m.resultSet.Rows) == 0 || len(m.resultSet.Columns) == 0 {
		return 0, 0, false
	}
	return m.selectedRow, m.selectedCol, true
}

// SelectedRow returns the row under the cursor.
func (m DataViewerModel) SelectedRow() (int, bool) {
	if m.resultSet == nil || len(m.resultSet.Rows) == 0 {
		return 0, false
	}
	return m.selectedRow, true
}

// CellValue returns the string form of the cell at (row, col).
func (m DataViewerModel) CellValue(row, col int) string {
	if m.resultSet == nil || row < 0 || row >= len(m.resultSet.Rows) {
		return ""
	}
	r := m.resultSet.Rows[row]
	if col < 0 || col >= len(r.Cells) {
		return ""
	}
	if r.Cells[col] == nil {
		return ""
	}
	return fmt.Sprintf("%v", r.Cells[col])
}

// ColumnName returns the name of the column at the given result-set index.
func (m DataViewerModel) ColumnName(col int) string {
	if m.resultSet == nil || col < 0 || col >= len(m.resultSet.Columns) {
		return ""
	}
	return m.resultSet.Columns[col].Name
}

// RowPK extracts the primary key map for the given row index.
func (m DataViewerModel) RowPK(rowIdx int) map[string]any {
	pk := map[string]any{}
	if m.resultSet == nil || m.tableRef == nil || rowIdx < 0 || rowIdx >= len(m.resultSet.Rows) {
		return pk
	}
	row := m.resultSet.Rows[rowIdx]
	for _, pkCol := range m.tableRef.PKCols {
		for ci, col := range m.resultSet.Columns {
			if col.Name == pkCol && ci < len(row.Cells) {
				pk[pkCol] = row.Cells[ci]
			}
		}
	}
	return pk
}

// OpenCellView enters cell-view mode for the given cell.
func (m *DataViewerModel) OpenCellView(row, col int) {
	m.cellViewActive = true
	m.cellViewContent = m.CellValue(row, col)
}

// CellViewView renders the cell viewport overlay.
func (m DataViewerModel) CellViewView() string {
	return StyleActiveBorder.
		Width(60).
		Height(20).
		Render(m.cellViewContent)
}

// formatCellValue renders a cell value for display.
func formatCellValue(v any) string {
	if v == nil {
		return "NULL"
	}
	return fmt.Sprintf("%v", v)
}
