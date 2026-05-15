package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ShortcutCategory groups shortcuts into the tabs shown in the overlay.
type ShortcutCategory int

const (
	CatNavigation ShortcutCategory = iota
	CatConnection
	CatTables
	CatSQL
	CatEditing
	CatViews
	CatAI
	CatGeneral
)

// Shortcut is one row in the catalog. Keys is a list of equivalent key
// labels (`["↑", "k"]` for an "up" entry) — they're joined with " / "
// when rendered.
type Shortcut struct {
	Keys     []string
	Action   string
	Category ShortcutCategory
}

// shortcutsCatalog is the single source of truth for the help overlay.
// Keep entries grouped by category for readability.
var shortcutsCatalog = []Shortcut{
	// Navigation
	{Keys: []string{"↑", "k"}, Action: "Move up", Category: CatNavigation},
	{Keys: []string{"↓", "j"}, Action: "Move down", Category: CatNavigation},
	{Keys: []string{"←", "h"}, Action: "Move cursor left (data viewer)", Category: CatNavigation},
	{Keys: []string{"→", "l"}, Action: "Move cursor right (data viewer)", Category: CatNavigation},
	{Keys: []string{"g"}, Action: "Jump to top of buffer", Category: CatNavigation},
	{Keys: []string{"G"}, Action: "Jump to bottom of buffer", Category: CatNavigation},
	{Keys: []string{"0"}, Action: "First column", Category: CatNavigation},
	{Keys: []string{"$"}, Action: "Last column", Category: CatNavigation},
	{Keys: []string{"Ctrl+f"}, Action: "Page forward (next 50 rows, replace)", Category: CatNavigation},
	{Keys: []string{"Ctrl+b"}, Action: "Page backward (previous 50 rows, replace)", Category: CatNavigation},
	{Keys: []string{"Ctrl+←"}, Action: "Previous tab", Category: CatNavigation},
	{Keys: []string{"Ctrl+→"}, Action: "Next tab", Category: CatNavigation},
	{Keys: []string{"Esc"}, Action: "Back / close current modal", Category: CatNavigation},

	// Connection
	{Keys: []string{"Enter"}, Action: "Connect to selected DB", Category: CatConnection},
	{Keys: []string{"n"}, Action: "New connection", Category: CatConnection},
	{Keys: []string{"e"}, Action: "Edit selected connection", Category: CatConnection},
	{Keys: []string{"d"}, Action: "Delete selected connection", Category: CatConnection},
	{Keys: []string{"Tab", "Shift+Tab"}, Action: "Next / previous form field", Category: CatConnection},

	// Tables
	{Keys: []string{"Enter"}, Action: "Open table in current data tab", Category: CatTables},
	{Keys: []string{"Ctrl+T"}, Action: "Open table in new tab", Category: CatTables},
	{Keys: []string{"Ctrl+W"}, Action: "Close current tab", Category: CatTables},
	{Keys: []string{"J"}, Action: "Open JOIN wizard", Category: CatTables},
	{Keys: []string{"Space"}, Action: "Mark / unmark row · sets range anchor", Category: CatTables},
	{Keys: []string{"M", "Shift+Space"}, Action: "Extend range from anchor to cursor", Category: CatTables},

	// SQL
	{Keys: []string{":"}, Action: "Open raw SQL bar (filter or full statement)", Category: CatSQL},
	{Keys: []string{"Ctrl+E"}, Action: "Open full-screen SQL editor", Category: CatSQL},
	{Keys: []string{"Ctrl+R"}, Action: "Run query (in editor)", Category: CatSQL},
	{Keys: []string{"Ctrl+L"}, Action: "Format SQL (in editor)", Category: CatSQL},
	{Keys: []string{"Tab"}, Action: "Schema-aware autocomplete (in editor)", Category: CatSQL},
	{Keys: []string{"Ctrl+S"}, Action: "Save current SQL as named view", Category: CatSQL},

	// Editing
	{Keys: []string{"Enter"}, Action: "Edit cell under cursor", Category: CatEditing},
	{Keys: []string{"v"}, Action: "View full cell content (modal)", Category: CatEditing},
	{Keys: []string{"s"}, Action: "Save all staged edits (commit transaction)", Category: CatEditing},
	{Keys: []string{"S"}, Action: "Review staged edits", Category: CatEditing},
	{Keys: []string{"D"}, Action: "Discard staged edits", Category: CatEditing},
	{Keys: []string{"d"}, Action: "Delete row (confirm modal)", Category: CatEditing},
	{Keys: []string{"y"}, Action: "Copy current cell to clipboard", Category: CatEditing},
	{Keys: []string{"Y"}, Action: "Copy current row or marked rows as TSV", Category: CatEditing},

	// Views
	{Keys: []string{"V"}, Action: "Open saved views list", Category: CatViews},
	{Keys: []string{"W"}, Action: "Save current SQL as a named view", Category: CatViews},
	{Keys: []string{"C"}, Action: "Copy a view from another connection (in views modal)", Category: CatViews},

	// AI
	{Keys: []string{"Ctrl+A", "F2"}, Action: "Ask AI (natural-language → SQL)", Category: CatAI},
	{Keys: []string{"Ctrl+P"}, Action: "Open AI profiles list", Category: CatAI},
	{Keys: []string{"a"}, Action: "Add new AI profile (in profiles modal)", Category: CatAI},
	{Keys: []string{"e"}, Action: "Edit selected AI profile", Category: CatAI},
	{Keys: []string{"d"}, Action: "Delete selected AI profile", Category: CatAI},
	{Keys: []string{"g"}, Action: "Open AI analytics dashboard", Category: CatAI},
	{Keys: []string{"d", "w", "m", "a"}, Action: "Analytics range: today / 7d / 30d / all", Category: CatAI},

	// General
	{Keys: []string{"F1"}, Action: "Open this shortcuts panel", Category: CatGeneral},
	{Keys: []string{"Ctrl+C"}, Action: "Quit zdb", Category: CatGeneral},
	{Keys: []string{"Esc"}, Action: "Close modal / back to previous view", Category: CatGeneral},
}

// shortcutsTabs defines the tab order shown at the top of the overlay.
// The first entry (Todas) is the "all" view — it filters nothing.
var shortcutsTabs = []struct {
	label string
	cat   ShortcutCategory // ignored when isAll is true
	isAll bool
}{
	{label: "All", isAll: true},
	{label: "Nav", cat: CatNavigation},
	{label: "Conn", cat: CatConnection},
	{label: "Tables", cat: CatTables},
	{label: "SQL", cat: CatSQL},
	{label: "Edit", cat: CatEditing},
	{label: "Views", cat: CatViews},
	{label: "AI", cat: CatAI},
	{label: "General", cat: CatGeneral},
}

// categoryLabel returns the full human label for a category — used as
// the section header when rendering the "All" tab. Section headers stay
// descriptive even though tab labels are abbreviated to fit the bar.
func categoryLabel(c ShortcutCategory) string {
	switch c {
	case CatNavigation:
		return "Navigation"
	case CatConnection:
		return "Connection"
	case CatTables:
		return "Tables"
	case CatSQL:
		return "SQL"
	case CatEditing:
		return "Editing"
	case CatViews:
		return "Views"
	case CatAI:
		return "AI"
	case CatGeneral:
		return "General"
	}
	return ""
}

// Chrome cost of the overlay box: outer border (2), padding (2), title
// row, blank, tabs row, blank, footer row, blank above footer — total
// nine rows around the scrollable viewport.
const shortcutsChromeHeight = 9

// Inner padding eaten by the box border + horizontal padding.
const shortcutsChromeWidth = 6

// ShortcutsModel is the bubbletea model for the help overlay.
type ShortcutsModel struct {
	active   int
	viewport viewport.Model
	width    int
	height   int
}

// NewShortcutsModel returns a fresh overlay sized to the viewport.
func NewShortcutsModel(width, height int) ShortcutsModel {
	m := ShortcutsModel{width: width, height: height}
	vw, vh := m.viewportSize()
	m.viewport = viewport.New(vw, vh)
	m.viewport.SetContent(renderShortcutBody(0))
	return m
}

// SetSize updates the viewport. Re-render uses the new dimensions.
func (m *ShortcutsModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	vw, vh := m.viewportSize()
	m.viewport.Width = vw
	m.viewport.Height = vh
	m.viewport.SetContent(renderShortcutBody(m.active))
}

// viewportSize returns the inner viewport dimensions, accounting for the
// surrounding chrome (border, padding, title, tabs, footer).
func (m ShortcutsModel) viewportSize() (int, int) {
	panelW := m.width - 8
	if panelW > 88 {
		panelW = 88
	}
	if panelW < 40 {
		panelW = 40
	}
	panelH := m.height - 4
	if panelH < 12 {
		panelH = 12
	}
	vw := panelW - shortcutsChromeWidth
	if vw < 20 {
		vw = 20
	}
	vh := panelH - shortcutsChromeHeight
	if vh < 5 {
		vh = 5
	}
	return vw, vh
}

// Init implements tea.Model.
func (m ShortcutsModel) Init() tea.Cmd { return nil }

// Update implements tea.Model. Returns the new model and a flag that
// tells the caller to dismiss the overlay (true when Esc / F1 / q were
// pressed).
func (m ShortcutsModel) Update(msg tea.Msg) (ShortcutsModel, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "f1", "q":
			return m, true
		case "right", "tab":
			m.active = (m.active + 1) % len(shortcutsTabs)
			m.viewport.SetContent(renderShortcutBody(m.active))
			m.viewport.GotoTop()
		case "left", "shift+tab":
			m.active = (m.active - 1 + len(shortcutsTabs)) % len(shortcutsTabs)
			m.viewport.SetContent(renderShortcutBody(m.active))
			m.viewport.GotoTop()
		default:
			// Pass scroll keys (↑/↓, j/k, PgUp/PgDn, Ctrl+u/d, …) to the
			// viewport.
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			_ = cmd
		}
	}
	return m, false
}

// View implements tea.Model.
func (m ShortcutsModel) View() string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(CtpPink).
		Padding(1, 2)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(CtpPink).
		Render("Keyboard shortcuts")

	tabs := renderShortcutTabs(m.active)
	footer := lipgloss.NewStyle().
		Foreground(CtpOverlay1).
		Render(scrollFooter(m.viewport))

	content := strings.Join([]string{title, "", tabs, "", m.viewport.View(), "", footer}, "\n")

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box.Render(content),
	)
}

func scrollFooter(vp viewport.Model) string {
	base := "←/→ tabs  ·  ↑/↓ scroll  ·  Esc close"
	if vp.TotalLineCount() > vp.Height {
		return base + "  ·  " + scrollIndicator(vp)
	}
	return base
}

func scrollIndicator(vp viewport.Model) string {
	pct := int(vp.ScrollPercent() * 100)
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	switch {
	case pct == 0:
		return "top"
	case pct == 100:
		return "bot"
	default:
		return "scroll"
	}
}

func renderShortcutTabs(active int) string {
	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(CtpBase).
		Background(CtpPink).
		Padding(0, 1)
	idleStyle := lipgloss.NewStyle().
		Foreground(CtpOverlay1).
		Padding(0, 1)
	sep := lipgloss.NewStyle().
		Foreground(CtpSurface2).
		Render("·")

	parts := make([]string, 0, len(shortcutsTabs)*2-1)
	for i, t := range shortcutsTabs {
		if i > 0 {
			parts = append(parts, sep)
		}
		if i == active {
			parts = append(parts, activeStyle.Render(t.label))
		} else {
			parts = append(parts, idleStyle.Render(t.label))
		}
	}
	return strings.Join(parts, " ")
}

func renderShortcutBody(active int) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(CtpSapphire).
		Bold(true)
	actionStyle := lipgloss.NewStyle().
		Foreground(CtpText)
	headerStyle := lipgloss.NewStyle().
		Foreground(CtpMauve).
		Bold(true)

	tab := shortcutsTabs[active]
	var lines []string

	if tab.isAll {
		first := true
		for _, c := range []ShortcutCategory{
			CatNavigation, CatConnection, CatTables, CatSQL,
			CatEditing, CatViews, CatAI, CatGeneral,
		} {
			if !first {
				lines = append(lines, "")
			}
			first = false
			lines = append(lines, headerStyle.Render(categoryLabel(c)))
			for _, s := range shortcutsCatalog {
				if s.Category != c {
					continue
				}
				lines = append(lines, formatShortcutRow(s, keyStyle, actionStyle))
			}
		}
	} else {
		for _, s := range shortcutsCatalog {
			if s.Category != tab.cat {
				continue
			}
			lines = append(lines, formatShortcutRow(s, keyStyle, actionStyle))
		}
	}

	return strings.Join(lines, "\n")
}

func formatShortcutRow(s Shortcut, keyStyle, actionStyle lipgloss.Style) string {
	keys := strings.Join(s.Keys, " / ")
	const keyCol = 16
	keysRendered := keyStyle.Render(keys)
	keyWidth := lipgloss.Width(keysRendered)
	padding := ""
	if keyWidth < keyCol {
		padding = strings.Repeat(" ", keyCol-keyWidth)
	}
	return "  " + keysRendered + padding + actionStyle.Render(s.Action)
}
