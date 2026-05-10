package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"

	"github.com/gabiito/zdb/internal/db"
)

// OpenTableMsg is emitted when the user selects a table to view. NewTab
// is set to true when the user explicitly requested a fresh data tab
// (Ctrl+Enter) instead of reusing the active data tab.
type OpenTableMsg struct {
	Table  *db.Table
	NewTab bool
}

// tableListItem wraps a TableSummary for the bubbles/list widget.
type tableListItem struct {
	summary db.TableSummary
}

func (t tableListItem) Title() string {
	name := t.summary.Name
	if t.summary.Schema != "" {
		name = t.summary.Schema + "." + name
	}
	return name
}

func (t tableListItem) Description() string {
	pkStr := "no PK"
	if t.summary.HasPK {
		pkStr = "has PK"
	}
	return fmt.Sprintf("%d cols · %s", t.summary.ColCount, pkStr)
}

func (t tableListItem) FilterValue() string { return t.summary.Name }

// SchemaBrowserModel renders the schema browser with two panes:
// left = table list, right = column list for selected table.
type SchemaBrowserModel struct {
	tableList   list.Model
	columns     []db.Column
	selectedTbl *db.Table
	cache       interface{ Table(string) *db.Table }
	width       int
	height      int
	focusLeft   bool
}

// NewSchemaBrowserModel creates a SchemaBrowserModel.
func NewSchemaBrowserModel(
	tables []db.TableSummary,
	cache interface{ Table(string) *db.Table },
	width, height int,
) SchemaBrowserModel {
	items := make([]list.Item, len(tables))
	for i, t := range tables {
		items[i] = tableListItem{summary: t}
	}

	leftWidth := width / 3
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(CtpPink).
		BorderForeground(CtpPink)

	l := list.New(items, delegate, leftWidth, height-4)
	l.Title = "Tables"
	l.Styles.Title = StyleTitle

	sb := SchemaBrowserModel{
		tableList: l,
		cache:     cache,
		width:     width,
		height:    height,
		focusLeft: true,
	}
	// Show the first table's schema immediately so the right pane is populated
	// from the moment the browser opens.
	sb.syncHighlightedTable()
	return sb
}

// syncHighlightedTable populates the right pane with the schema of whichever
// table is currently highlighted in the left list. Idempotent — safe to call
// after every Update.
func (m *SchemaBrowserModel) syncHighlightedTable() {
	item, ok := m.tableList.SelectedItem().(tableListItem)
	if !ok {
		return
	}
	qualName := item.summary.Name
	if item.summary.Schema != "" {
		qualName = item.summary.Schema + "." + item.summary.Name
	}
	if tbl := m.cache.Table(qualName); tbl != nil {
		m.selectedTbl = tbl
		m.columns = tbl.Columns
	}
}

// SetSize updates the available width/height of the browser. Called by the
// App on every render so the panes shrink to fit the SQL bar and banners.
func (m *SchemaBrowserModel) SetSize(w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	m.width = w
	m.height = h
	leftWidth := w / 3
	if leftWidth < 16 {
		leftWidth = 16
	}
	listH := h - 4
	if listH < 1 {
		listH = 1
	}
	m.tableList.SetSize(leftWidth, listH)
}

// Init implements tea.Model.
func (m SchemaBrowserModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m SchemaBrowserModel) Update(msg tea.Msg) (SchemaBrowserModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		// "ctrl+enter" only fires on terminals with the Kitty keyboard
		// protocol (kitty, wezterm, modern alacritty); everywhere else
		// Enter and Ctrl+Enter share the same byte. "ctrl+t" is the
		// portable fallback for "open in a new tab".
		if key == "enter" || key == "ctrl+enter" || key == "ctrl+t" {
			if item, ok := m.tableList.SelectedItem().(tableListItem); ok {
				qualName := item.summary.Name
				if item.summary.Schema != "" {
					qualName = item.summary.Schema + "." + item.summary.Name
				}
				if tbl := m.cache.Table(qualName); tbl != nil {
					newTab := key == "ctrl+enter" || key == "ctrl+t"
					return m, func() tea.Msg {
						return OpenTableMsg{Table: tbl, NewTab: newTab}
					}
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		leftWidth := msg.Width / 3
		m.tableList.SetSize(leftWidth, msg.Height-4)
	}

	var cmd tea.Cmd
	m.tableList, cmd = m.tableList.Update(msg)
	m.syncHighlightedTable()
	return m, cmd
}

// View renders the schema browser.
func (m SchemaBrowserModel) View() string {
	leftWidth := m.width / 3
	rightWidth := m.width - leftWidth - 2

	leftPane := StyleInactiveBorder.
		Width(leftWidth).
		Height(m.height - 4).
		Render(m.tableList.View())

	var rightContent string
	if m.selectedTbl != nil {
		rightContent = StyleTitle.Render("Columns: "+m.selectedTbl.Name) + "\n\n"
		for _, col := range m.columns {
			pk := ""
			if col.IsPK {
				pk = " [PK]"
			}
			nullable := ""
			if col.Nullable {
				nullable = " NULL"
			}
			rightContent += fmt.Sprintf("  %-20s %s%s%s\n",
				col.Name, col.NativeType, pk, nullable)
		}
	} else {
		rightContent = StyleDim.Render("← select a table")
	}

	rightPane := StyleInactiveBorder.
		Width(rightWidth).
		Height(m.height - 4).
		Render(rightContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}
