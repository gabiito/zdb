package tui

import (
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
)

var (
	styleHelpKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)
	styleHelpDesc = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))
	styleHelpSep = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))
)

// helpItem is a (key, description) pair displayed in the help bar.
type helpItem struct {
	keys string
	desc string
}

// bindingsFor returns the ordered list of bindings to display for a given context.
// aiEnabled is reserved for future use (e.g., dimming AI hints when disabled);
// for now the same bar is shown regardless so the user knows the keys exist.
func bindingsFor(ctx HelpContext, aiEnabled bool) []helpItem {
	_ = aiEnabled

	switch ctx {
	case HelpContextConnPicker:
		return []helpItem{
			{keys: "↑/↓", desc: "navigate"},
			{keys: "Enter", desc: "connect"},
			{keys: "n", desc: "new connection"},
			{keys: "Ctrl+c", desc: "quit"},
		}
	case HelpContextSchemaBrowser:
		return []helpItem{
			{keys: "↑/↓", desc: "navigate"},
			{keys: "Enter", desc: "open table"},
			{keys: "s", desc: "save"},
			{keys: "S", desc: "review staged"},
			{keys: "D", desc: "discard staged"},
			{keys: ":", desc: "SQL"},
			{keys: "V", desc: "saved views"},
			{keys: "Esc", desc: "back"},
			{keys: "Ctrl+c", desc: "quit"},
		}
	case HelpContextDataViewer:
		return []helpItem{
			{keys: "←↑↓→", desc: "cell"},
			{keys: "g/G", desc: "top/bottom"},
			{keys: "Ctrl+f/b", desc: "page"},
			{keys: "Enter", desc: "edit"},
			{keys: "v", desc: "view"},
			{keys: "s", desc: "save"},
			{keys: "S", desc: "review staged"},
			{keys: "D", desc: "discard staged"},
			{keys: "d", desc: "delete"},
			{keys: ":", desc: "SQL"},
			{keys: "J", desc: "join"},
			{keys: "V", desc: "saved views"},
			{keys: "W", desc: "save view"},
			{keys: "Ctrl+a", desc: "ask AI"},
			{keys: "Esc", desc: "back"},
			{keys: "Ctrl+c", desc: "quit"},
		}
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
	}
	return nil
}

// RenderHelpBar produces a single-line help bar for the given context, truncated
// to fit width. Items are dropped from the right when the rendered width exceeds
// the available space, never mid-word.
func RenderHelpBar(ctx HelpContext, width int, aiEnabled bool) string {
	items := bindingsFor(ctx, aiEnabled)
	if len(items) == 0 || width <= 0 {
		return ""
	}

	sep := styleHelpSep.Render(" · ")
	sepWidth := lipgloss.Width(sep)

	var rendered []string
	used := 0
	for i, it := range items {
		piece := styleHelpKey.Render(it.keys) + " " + styleHelpDesc.Render(it.desc)
		w := lipgloss.Width(piece)
		extra := w
		if i > 0 {
			extra += sepWidth
		}
		if used+extra > width {
			break
		}
		rendered = append(rendered, piece)
		used += extra
	}

	if len(rendered) == 0 {
		return ""
	}

	line := strings.Join(rendered, sep)
	// Pad with a leading space so it doesn't kiss the left edge.
	return " " + line
}
