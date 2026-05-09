package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/evertras/bubble-table/table"

	"github.com/gabiito/db-viewer/internal/db"
)

// DataViewerModel wraps bubble-table and provides paginated row viewing.
type DataViewerModel struct {
	tableModel   table.Model
	tableRef     *db.Table
	resultSet    *db.ResultSet
	pageSize     int
	width        int
	height       int

	// Cell view state
	cellViewContent string
	cellViewActive  bool

	// Current row/col selection (for cell view and edit)
	selectedRow int
	selectedCol int
}

// NewDataViewerModel creates a DataViewerModel for the given table.
func NewDataViewerModel(t *db.Table, pageSize, width, height int) DataViewerModel {
	return DataViewerModel{
		tableRef: t,
		pageSize: pageSize,
		width:    width,
		height:   height,
	}
}

// SetData updates the displayed data from a ResultSet.
func (m *DataViewerModel) SetData(rs *db.ResultSet) {
	if rs == nil {
		return
	}
	m.resultSet = rs

	// Build bubble-table columns
	cols := make([]table.Column, len(rs.Columns))
	for i, c := range rs.Columns {
		width := 16
		if len(c.Name) > width {
			width = len(c.Name) + 2
		}
		cols[i] = table.NewColumn(c.Name, c.Name, width)
	}

	// Build rows
	rows := make([]table.Row, len(rs.Rows))
	for i, r := range rs.Rows {
		rowData := table.RowData{}
		for j, cell := range r.Cells {
			if j < len(rs.Columns) {
				colName := rs.Columns[j].Name
				if cell == nil {
					rowData[colName] = "NULL"
				} else {
					rowData[colName] = fmt.Sprintf("%v", cell)
				}
			}
		}
		rows[i] = table.NewRow(rowData)
	}

	t := table.New(cols).
		WithRows(rows).
		WithPageSize(m.pageSize).
		Focused(true).
		WithBaseStyle(StyleNormal)

	m.tableModel = t
}

// Init implements tea.Model.
func (m DataViewerModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m DataViewerModel) Update(msg tea.Msg) (DataViewerModel, tea.Cmd) {
	var cmd tea.Cmd
	m.tableModel, cmd = m.tableModel.Update(msg)
	return m, cmd
}

// UpdateCellView handles messages when in cell-view mode.
func (m DataViewerModel) UpdateCellView(msg tea.Msg) (DataViewerModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc", "q":
			m.cellViewActive = false
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m DataViewerModel) View() string {
	if m.tableRef == nil {
		return "No table selected"
	}

	var sb strings.Builder

	// PK-less banner
	if len(m.tableRef.PKCols) == 0 {
		sb.WriteString(StyleBannerRed.Render("READ-ONLY: table has no primary key") + "\n")
	}

	if m.resultSet == nil {
		sb.WriteString(StyleDim.Render("Loading..."))
	} else {
		sb.WriteString(m.tableModel.View())
		// Row counter in status area
		total := len(m.resultSet.Rows)
		sb.WriteString("\n")
		sb.WriteString(StyleHelp.Render(fmt.Sprintf("Row %d / %d", m.tableModel.GetHighlightedRowIndex()+1, total)))
	}

	return sb.String()
}

// Table returns the schema table reference.
func (m DataViewerModel) Table() *db.Table { return m.tableRef }

// SelectedCell returns the row index, column index, and true if a cell is selected.
func (m DataViewerModel) SelectedCell() (int, int, bool) {
	if m.resultSet == nil {
		return 0, 0, false
	}
	row := m.tableModel.GetHighlightedRowIndex()
	return row, m.selectedCol, row >= 0 && row < len(m.resultSet.Rows)
}

// SelectedRow returns the row index and true if a row is selected.
func (m DataViewerModel) SelectedRow() (int, bool) {
	if m.resultSet == nil {
		return 0, false
	}
	row := m.tableModel.GetHighlightedRowIndex()
	return row, row >= 0 && row < len(m.resultSet.Rows)
}

// CellValue returns the string representation of a cell value.
func (m DataViewerModel) CellValue(row, col int) string {
	if m.resultSet == nil || row >= len(m.resultSet.Rows) {
		return ""
	}
	r := m.resultSet.Rows[row]
	if col >= len(r.Cells) {
		return ""
	}
	if r.Cells[col] == nil {
		return ""
	}
	return fmt.Sprintf("%v", r.Cells[col])
}

// RowPK extracts the primary key map for the given row index.
func (m DataViewerModel) RowPK(rowIdx int) map[string]any {
	pk := map[string]any{}
	if m.resultSet == nil || m.tableRef == nil || rowIdx >= len(m.resultSet.Rows) {
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

// CellViewView renders the cell viewport.
func (m DataViewerModel) CellViewView() string {
	return StyleActiveBorder.
		Width(60).
		Height(20).
		Render(m.cellViewContent)
}
