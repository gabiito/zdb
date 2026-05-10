package tui

import "github.com/charmbracelet/lipgloss"

// Catppuccin palette — Latte (light terminals) / Mocha (dark terminals).
// All accents and surfaces flow from this set, so a future theme swap is
// a one-file edit. Catppuccin doesn't map exactly to xterm-256, so we
// pass hex strings; lipgloss uses truecolor when the terminal supports
// it and falls back gracefully otherwise.
var (
	// Accents
	CtpRosewater = lipgloss.AdaptiveColor{Light: "#dc8a78", Dark: "#f5e0dc"}
	CtpFlamingo  = lipgloss.AdaptiveColor{Light: "#dd7878", Dark: "#f2cdcd"}
	CtpPink      = lipgloss.AdaptiveColor{Light: "#ea76cb", Dark: "#f5c2e7"}
	CtpMauve     = lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#cba6f7"}
	CtpRed       = lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"}
	CtpMaroon    = lipgloss.AdaptiveColor{Light: "#e64553", Dark: "#eba0ac"}
	CtpPeach     = lipgloss.AdaptiveColor{Light: "#fe640b", Dark: "#fab387"}
	CtpYellow    = lipgloss.AdaptiveColor{Light: "#df8e1d", Dark: "#f9e2af"}
	CtpGreen     = lipgloss.AdaptiveColor{Light: "#40a02b", Dark: "#a6e3a1"}
	CtpTeal      = lipgloss.AdaptiveColor{Light: "#179299", Dark: "#94e2d5"}
	CtpSky       = lipgloss.AdaptiveColor{Light: "#04a5e5", Dark: "#89dceb"}
	CtpSapphire  = lipgloss.AdaptiveColor{Light: "#209fb5", Dark: "#74c7ec"}
	CtpBlue      = lipgloss.AdaptiveColor{Light: "#1e66f5", Dark: "#89b4fa"}
	CtpLavender  = lipgloss.AdaptiveColor{Light: "#7287fd", Dark: "#b4befe"}

	// Foreground tones (text → progressively dimmer)
	CtpText     = lipgloss.AdaptiveColor{Light: "#4c4f69", Dark: "#cdd6f4"}
	CtpSubtext1 = lipgloss.AdaptiveColor{Light: "#5c5f77", Dark: "#bac2de"}
	CtpSubtext0 = lipgloss.AdaptiveColor{Light: "#6c6f85", Dark: "#a6adc8"}
	CtpOverlay2 = lipgloss.AdaptiveColor{Light: "#7c7f93", Dark: "#9399b2"}
	CtpOverlay1 = lipgloss.AdaptiveColor{Light: "#8c8fa1", Dark: "#7f849c"}
	CtpOverlay0 = lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#6c7086"}

	// Surface tones (subtle backgrounds → progressively lighter)
	CtpSurface2 = lipgloss.AdaptiveColor{Light: "#acb0be", Dark: "#585b70"}
	CtpSurface1 = lipgloss.AdaptiveColor{Light: "#bcc0cc", Dark: "#45475a"}
	CtpSurface0 = lipgloss.AdaptiveColor{Light: "#ccd0da", Dark: "#313244"}

	// Bases — terminal-paired. CtpBase doubles as the "inverse foreground"
	// when used on top of an accent background: Latte Base (#eff1f5) is
	// light enough to read on Latte's darker accents, Mocha Base (#1e1e2e)
	// is dark enough to read on Mocha's lighter accents.
	CtpBase   = lipgloss.AdaptiveColor{Light: "#eff1f5", Dark: "#1e1e2e"}
	CtpMantle = lipgloss.AdaptiveColor{Light: "#e6e9ef", Dark: "#181825"}
	CtpCrust  = lipgloss.AdaptiveColor{Light: "#dce0e8", Dark: "#11111b"}
)

var (
	// Normal text — defaults to terminal foreground.
	StyleNormal = lipgloss.NewStyle()

	// Selected/focused item.
	StyleSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(CtpPink)

	// Error text.
	StyleError = lipgloss.NewStyle().
			Bold(true).
			Foreground(CtpRed)

	// Dimmed / disabled hint.
	StyleDim = lipgloss.NewStyle().
			Foreground(CtpOverlay0)

	// Read-only / warning banner (red background).
	StyleBannerRed = lipgloss.NewStyle().
			Bold(true).
			Foreground(CtpBase).
			Background(CtpRed).
			Padding(0, 1)

	// Warning banner (yellow background).
	StyleBannerYellow = lipgloss.NewStyle().
				Bold(true).
				Foreground(CtpBase).
				Background(CtpYellow).
				Padding(0, 1)

	// Status bar.
	StyleStatusBar = lipgloss.NewStyle().
			Foreground(CtpSubtext0).
			Background(CtpSurface0).
			Padding(0, 1)

	// Status bar error.
	StyleStatusBarError = lipgloss.NewStyle().
				Foreground(CtpRed).
				Background(CtpSurface0).
				Padding(0, 1)

	// Spinner / in-flight indicator.
	StyleSpinner = lipgloss.NewStyle().
			Foreground(CtpPink)

	// Title bar.
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(CtpPink).
			Padding(0, 1)

	// Help text (general descriptions).
	StyleHelp = lipgloss.NewStyle().
			Foreground(CtpOverlay1)

	// Highlighted row in the data viewer. Idiomatic Catppuccin: a Surface
	// tone (neutral, slightly bluer than ambient) holds the row, and the
	// saturated Pink lives only in the cell cursor on top of it.
	StyleSelectedRow = lipgloss.NewStyle().
				Foreground(CtpText).
				Background(lipgloss.AdaptiveColor{
			Light: "#acb0be", // Latte Surface2
			Dark:  "#45475a", // Mocha Surface1
		})

	// Highlighted cell on top of the selected row.
	StyleSelectedCell = lipgloss.NewStyle().
				Bold(true).
				Foreground(CtpBase).
				Background(CtpPink)

	// Marked row in the data viewer (multi-row copy selection set).
	StyleMarkedRow = lipgloss.NewStyle().
			Foreground(CtpBase).
			Background(CtpTeal)

	// Active pane border (saturated accent).
	StyleActiveBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(CtpPink)

	// Inactive pane border.
	StyleInactiveBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(CtpOverlay0)
)
