package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/gabiito/zdb/internal/db"
)

// CellEditModel is the cell edit modal using bubbles/textinput.
type CellEditModel struct {
	input   textinput.Model
	rowIdx  int
	colIdx  int
	oldVal  string
	table   *db.Table
	col     db.Column
	pkVals  map[string]any
}

// NewCellEditModel creates a CellEditModel pre-filled with the current value.
func NewCellEditModel(rowIdx, colIdx int, currentValue string) CellEditModel {
	ti := textinput.New()
	ti.SetValue(currentValue)
	ti.Focus()
	ti.CharLimit = 1024
	ti.Width = 50

	return CellEditModel{
		input:  ti,
		rowIdx: rowIdx,
		colIdx: colIdx,
		oldVal: currentValue,
	}
}

// WithTableContext attaches table and PK context (called before mounting the modal).
func (m CellEditModel) WithTableContext(t *db.Table, col db.Column, pk map[string]any) CellEditModel {
	m.table = t
	m.col = col
	m.pkVals = pk
	return m
}

// Init implements tea.Model.
func (m CellEditModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m CellEditModel) Update(msg tea.Msg) (CellEditModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Emit staged change
			return m, func() tea.Msg {
				return StagedChangeMsg{
					Table:  m.table,
					PK:     m.pkVals,
					Col:    m.col,
					OldVal: m.oldVal,
					NewVal: m.input.Value(),
				}
			}
		case "esc":
			return m, func() tea.Msg {
				return DiscardEditMsg{}
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m CellEditModel) View() string {
	return StyleActiveBorder.
		Width(60).
		Render(
			StyleTitle.Render("Edit cell") + "\n\n" +
				m.input.View() + "\n\n" +
				StyleHelp.Render("Enter: save · Esc: cancel"),
		)
}
