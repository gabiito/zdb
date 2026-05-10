package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// HelpContext identifies which screen or modal is currently active.
// Owned by tui to avoid an import cycle with core.
type HelpContext int

const (
	HelpContextConnPicker HelpContext = iota
	HelpContextSchemaBrowser
	HelpContextDataViewer
	HelpContextSqlPanel
	HelpContextAskPanel
	HelpContextModalCellEdit
	HelpContextModalConfirm
	HelpContextModalCellView
	HelpContextModalStagedView
	HelpContextModalJoinWizard
	HelpContextSqlBarFocused
	HelpContextModalViewsList
	HelpContextModalSaveView
	HelpContextModalJoinChoice
	HelpContextModalAddConnection
	HelpContextModalEditConnection
	HelpContextWelcome
	HelpContextModalPasswordPrompt
	HelpContextSQLEditor
	HelpContextModalAISetup
	HelpContextModalAIDebug
	HelpContextModalAIProfileList
	HelpContextModalAIAnalytics
)

var (
	styleHelpKey = lipgloss.NewStyle().
			Foreground(CtpSapphire).
			Bold(true)
	styleHelpDesc = lipgloss.NewStyle().
			Foreground(CtpOverlay2)
	styleHelpSep = lipgloss.NewStyle().
			Foreground(CtpSurface2)

	// Primary action: visually louder than the regular key/desc styling so
	// the "what you probably want now" cue stands out from the rest of
	// the bar without inflating its width.
	stylePrimaryKey = lipgloss.NewStyle().
			Foreground(CtpPeach).
			Bold(true)
	stylePrimaryDesc = lipgloss.NewStyle().
				Foreground(CtpYellow).
				Bold(true)
)

// helpItem is a (key, description) pair displayed in the help bar.
// primary marks the single action the user most likely wants in the
// current state — rendered with stronger styling.
type helpItem struct {
	keys    string
	desc    string
	primary bool
}

// HelpState carries the dynamic context the App injects into the help bar
// so the displayed bindings reflect what's actually actionable right now.
// All fields default to a sensible zero value — modals and other contexts
// that don't depend on dynamic state can ignore it entirely.
type HelpState struct {
	AIEnabled         bool
	StagedCount       int  // pending edits in the buffer
	MarkCount         int  // marked rows in the data viewer
	AtLastLoadedRow   bool // cursor sits on the last buffered row
	MoreRowsAvailable bool // dbOffset+loaded < totalRows (false when total unknown)
	HasJoinChain      bool // a JOIN chain is materialized in the data viewer
	HasResultSet      bool // the data viewer has data loaded
	SelectedCol       int  // cursor's column index — col > 0 is treated as a "data" column
	TabCount          int  // number of open tabs (incl. Schema); used to surface tab nav keys
}

// dataViewerBindings builds a context-aware help item list for the data
// viewer. The first item — when one is selected — is marked primary so
// it stands out: this is the "what you probably want now" cue. The
// precedence reflects user intent:
//
//  1. Pending staged edits → save (writes are urgent and easy to lose).
//  2. Active row marks     → copy the marked rows (the user invested
//                             effort in marking; that's why they marked).
//  3. At buffer boundary   → load more (surface the affordance exactly
//                             when scrolling forward stops working).
//  4. Cursor on data col   → copy/view cell (they navigated horizontally
//                             into a specific cell — likely inspecting it).
//  5. Idle / on PK column  → just navigation hints.
//
// Secondary keys are kept short and shared across all states; widely-known
// chrome (top/bottom, page, SQL, join, views, AI, quit) sits at the tail
// so terminal-width truncation eats it last.
func dataViewerBindings(s HelpState) []helpItem {
	if !s.HasResultSet {
		return []helpItem{
			{keys: "Esc", desc: "back"},
			{keys: "Ctrl+c", desc: "quit"},
		}
	}

	var leading []helpItem
	switch {
	case s.StagedCount > 0:
		leading = []helpItem{
			{keys: "s", desc: fmt.Sprintf("save %d staged", s.StagedCount), primary: true},
			{keys: "S", desc: "review"},
			{keys: "D", desc: "discard"},
		}
	case s.MarkCount > 0:
		leading = []helpItem{
			{keys: "Y", desc: fmt.Sprintf("copy %d marked", s.MarkCount), primary: true},
			{keys: "M", desc: "extend range"},
			{keys: "Space", desc: "toggle"},
			{keys: "Esc", desc: "clear marks"},
		}
	case s.AtLastLoadedRow && s.MoreRowsAvailable:
		leading = []helpItem{
			{keys: "↓", desc: "load more", primary: true},
			{keys: "←↑↓→", desc: "cell"},
			{keys: "y", desc: "copy cell"},
		}
	case s.SelectedCol > 0:
		leading = []helpItem{
			{keys: "y", desc: "copy cell", primary: true},
			{keys: "v", desc: "view cell"},
			{keys: "Enter", desc: "edit"},
			{keys: "←↑↓→", desc: "cell"},
		}
	default:
		leading = []helpItem{
			{keys: "←↑↓→", desc: "cell"},
			{keys: "Enter", desc: "edit"},
			{keys: "y/Y", desc: "copy cell/row"},
			{keys: "Space/M", desc: "mark/range"},
		}
	}

	// Common secondary keys, always present, ordered by frequency of use.
	secondary := []helpItem{
		{keys: "g/G", desc: "top/bottom"},
		{keys: "Ctrl+f/b", desc: "page"},
		{keys: "d", desc: "del row"},
		{keys: ":", desc: "SQL"},
		{keys: "Ctrl+e", desc: "SQL editor"},
		{keys: "J", desc: "join"},
		{keys: "V/W", desc: "views"},
	}
	if s.TabCount > 1 {
		secondary = append(secondary,
			helpItem{keys: "Ctrl+←/→", desc: "tab"},
			helpItem{keys: "Ctrl+w", desc: "close tab"},
		)
	}
	if s.AIEnabled {
		secondary = append(secondary, helpItem{keys: "Ctrl+a", desc: "ask AI"})
	}

	// De-dup against the leading items so we don't repeat keys (e.g. ←↑↓→
	// appears in some leading sets and would also try to land in secondary).
	seen := map[string]bool{}
	for _, it := range leading {
		seen[it.keys] = true
	}
	items := append([]helpItem{}, leading...)
	for _, it := range secondary {
		if !seen[it.keys] {
			items = append(items, it)
		}
	}

	// "Esc back" only when no marks (otherwise leading already has
	// "Esc clear marks" which takes precedence).
	if s.MarkCount == 0 {
		items = append(items, helpItem{keys: "Esc", desc: "back"})
	}
	items = append(items, helpItem{keys: "Ctrl+c", desc: "quit"})
	return items
}

// bindingsFor returns the ordered list of bindings to display for a given
// context, adapting to the dynamic HelpState — irrelevant actions are
// hidden, counts are folded into descriptions, and contextual hints
// (e.g. "load more" at the buffer boundary) appear only when actionable.
func bindingsFor(ctx HelpContext, s HelpState) []helpItem {
	switch ctx {
	case HelpContextConnPicker:
		return []helpItem{
			{keys: "↑/↓", desc: "navigate"},
			{keys: "Enter", desc: "connect"},
			{keys: "n", desc: "new"},
			{keys: "e", desc: "edit"},
			{keys: "d", desc: "delete"},
			{keys: "Ctrl+c", desc: "quit"},
		}
	case HelpContextWelcome:
		return []helpItem{
			{keys: "n", desc: "add connection"},
			{keys: "q", desc: "quit"},
		}
	case HelpContextSchemaBrowser:
		var items []helpItem
		if s.StagedCount > 0 {
			items = append(items,
				helpItem{keys: "s", desc: fmt.Sprintf("save %d staged", s.StagedCount), primary: true},
				helpItem{keys: "S", desc: "review"},
				helpItem{keys: "D", desc: "discard"},
			)
		}
		items = append(items,
			helpItem{keys: "↑/↓", desc: "navigate"},
			helpItem{keys: "Enter", desc: "open table"},
			helpItem{keys: "Ctrl+t", desc: "in new tab"},
		)
		if s.TabCount > 1 {
			items = append(items, helpItem{keys: "Ctrl+←/→", desc: "switch tab"})
		}
		items = append(items,
			helpItem{keys: ":", desc: "SQL"},
			helpItem{keys: "Ctrl+e", desc: "SQL editor"},
			helpItem{keys: "V", desc: "saved views"},
			helpItem{keys: "Esc", desc: "back"},
			helpItem{keys: "Ctrl+c", desc: "quit"},
		)
		return items
	case HelpContextDataViewer:
		return dataViewerBindings(s)
	case HelpContextSqlPanel:
		return []helpItem{
			{keys: "Enter", desc: "execute"},
			{keys: "Ctrl+Space", desc: "AI suggest"},
			{keys: "Tab", desc: "accept"},
			{keys: "Esc", desc: "back"},
		}
	case HelpContextAskPanel:
		return []helpItem{
			{keys: "Enter", desc: "submit"},
			{keys: "Esc", desc: "back"},
		}
	case HelpContextModalCellEdit:
		return []helpItem{
			{keys: "Enter", desc: "stage"},
			{keys: "Esc", desc: "discard"},
		}
	case HelpContextModalConfirm:
		return []helpItem{
			{keys: "y", desc: "confirm"},
			{keys: "n/Esc", desc: "cancel"},
		}
	case HelpContextModalCellView:
		return []helpItem{
			{keys: "↑/↓", desc: "scroll"},
			{keys: "Esc", desc: "close"},
		}
	case HelpContextModalStagedView:
		return []helpItem{
			{keys: "↑/↓", desc: "scroll"},
			{keys: "s", desc: "save"},
			{keys: "D", desc: "discard"},
			{keys: "Esc", desc: "close"},
		}
	case HelpContextModalJoinWizard:
		return []helpItem{
			{keys: "↑/↓", desc: "navigate"},
			{keys: "Enter", desc: "next/run"},
			{keys: "Space", desc: "toggle"},
			{keys: "a/n", desc: "all/none"},
			{keys: "Backspace", desc: "back"},
			{keys: "Esc", desc: "cancel"},
		}
	case HelpContextSqlBarFocused:
		return []helpItem{
			{keys: "type", desc: "SQL"},
			{keys: "Tab", desc: "autocomplete"},
			{keys: "Enter", desc: "run"},
			{keys: "Esc", desc: "unfocus"},
		}
	case HelpContextModalViewsList:
		return []helpItem{
			{keys: "↑/↓", desc: "navigate"},
			{keys: "Enter", desc: "run"},
			{keys: "D", desc: "delete"},
			{keys: "Esc", desc: "close"},
		}
	case HelpContextModalSaveView:
		return []helpItem{
			{keys: "type", desc: "name"},
			{keys: "Enter", desc: "save"},
			{keys: "Esc", desc: "cancel"},
		}
	case HelpContextModalJoinChoice:
		return []helpItem{
			{keys: "A", desc: "add JOIN"},
			{keys: "R", desc: "replace"},
			{keys: "Esc", desc: "cancel"},
		}
	case HelpContextModalAddConnection:
		return []helpItem{
			{keys: "Tab", desc: "next field"},
			{keys: "Shift+Tab", desc: "prev field"},
			{keys: "Enter", desc: "save"},
			{keys: "Esc", desc: "cancel"},
		}
	case HelpContextModalEditConnection:
		return []helpItem{
			{keys: "Tab", desc: "next field"},
			{keys: "Shift+Tab", desc: "prev field"},
			{keys: "Enter", desc: "save"},
			{keys: "Esc", desc: "cancel"},
		}
	case HelpContextModalPasswordPrompt:
		return []helpItem{
			{keys: "Enter", desc: "connect"},
			{keys: "Esc", desc: "cancel"},
		}
	case HelpContextSQLEditor:
		return []helpItem{
			{keys: "Ctrl+R", desc: "run", primary: true},
			{keys: "Ctrl+L", desc: "format"},
			{keys: "Ctrl+S", desc: "save view"},
			{keys: "Tab", desc: "autocomplete"},
			{keys: "Esc", desc: "back"},
			{keys: "Ctrl+c", desc: "quit"},
		}
	case HelpContextModalAISetup:
		return []helpItem{
			{keys: "Tab", desc: "next field"},
			{keys: "←/→", desc: "preset"},
			{keys: "Enter", desc: "save"},
			{keys: "Esc", desc: "cancel"},
		}
	case HelpContextModalAIDebug:
		return []helpItem{
			{keys: "Enter", desc: "retry with hint", primary: true},
			{keys: "Ctrl+e", desc: "edit SQL"},
			{keys: "Esc", desc: "cancel"},
		}
	case HelpContextModalAIProfileList:
		return []helpItem{
			{keys: "Enter", desc: "activate", primary: true},
			{keys: "a", desc: "add"},
			{keys: "e", desc: "edit"},
			{keys: "d", desc: "delete"},
			{keys: "g", desc: "analytics"},
			{keys: "↑/↓", desc: "nav"},
			{keys: "Esc", desc: "close"},
		}
	case HelpContextModalAIAnalytics:
		return []helpItem{
			{keys: "d", desc: "today"},
			{keys: "w", desc: "7 days"},
			{keys: "m", desc: "30 days"},
			{keys: "a", desc: "all"},
			{keys: "Esc", desc: "close"},
		}
	}
	return nil
}

// RenderHelpBar produces a single-line help bar for the given context,
// truncated to fit width. Items are dropped from the right when the
// rendered width exceeds the available space, never mid-word. The
// dynamic HelpState lets the bar surface only the actions that apply
// to the current state of the data viewer / schema browser.
//
// The width budget reserves 1 column for the leading space and 1 more
// as a safety margin against terminals that render some Unicode glyphs
// (box drawing, arrow chars) wider than lipgloss measures them. Even
// after item-level truncation, the final line is hard-clamped via
// MaxWidth so a misjudgment can never produce an overflow that wraps.
func RenderHelpBar(ctx HelpContext, width int, state HelpState) string {
	items := bindingsFor(ctx, state)
	if len(items) == 0 || width <= 0 {
		return ""
	}

	const leadingSpace = 1
	const safetyMargin = 1
	budget := width - leadingSpace - safetyMargin
	if budget <= 0 {
		return ""
	}

	sep := styleHelpSep.Render(" · ")
	sepWidth := lipgloss.Width(sep)

	var rendered []string
	used := 0
	for i, it := range items {
		keyStyle, descStyle := styleHelpKey, styleHelpDesc
		if it.primary {
			keyStyle, descStyle = stylePrimaryKey, stylePrimaryDesc
		}
		piece := keyStyle.Render(it.keys) + " " + descStyle.Render(it.desc)
		w := lipgloss.Width(piece)
		extra := w
		if i > 0 {
			extra += sepWidth
		}
		if used+extra > budget {
			break
		}
		rendered = append(rendered, piece)
		used += extra
	}

	if len(rendered) == 0 {
		return ""
	}

	line := " " + strings.Join(rendered, sep)
	// Hard clamp: even if our width calculations were off (wide-char
	// terminals, unexpected styling), MaxWidth guarantees the line fits.
	return lipgloss.NewStyle().MaxWidth(width).Render(line)
}
