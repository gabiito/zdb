package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keyboard bindings for zDB.
type KeyMap struct {
	// Navigation
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding

	// Actions
	Enter    key.Binding
	Escape   key.Binding
	Delete   key.Binding
	Save     key.Binding
	ViewCell key.Binding

	// Panels
	SQLPanel key.Binding
	AskPanel key.Binding

	// AI
	AISuggest    key.Binding
	AskPanelAlt  key.Binding // F2 fallback

	// Confirm
	Confirm key.Binding
	Cancel  key.Binding

	// Tab / suggestions
	Tab key.Binding

	// Quit
	Quit key.Binding
}

// DefaultKeyMap returns the default keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("ctrl+b"),
			key.WithHelp("ctrl+b", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("ctrl+f"),
			key.WithHelp("ctrl+f", "page down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select/edit"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back/cancel"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete row"),
		),
		Save: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "save changes"),
		),
		ViewCell: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "view cell"),
		),
		SQLPanel: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "SQL panel"),
		),
		AskPanel: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("ctrl+a", "AI ask"),
		),
		AskPanelAlt: key.NewBinding(
			key.WithKeys("f2"),
			key.WithHelp("F2", "AI ask (alt)"),
		),
		AISuggest: key.NewBinding(
			key.WithKeys("ctrl+ "),
			key.WithHelp("ctrl+space", "AI suggest"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("n", "esc"),
			key.WithHelp("n/esc", "cancel"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "accept suggestion"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "q"),
			key.WithHelp("ctrl+c/q", "quit"),
		),
	}
}

// Keys is the package-level default key map.
var Keys = DefaultKeyMap()
