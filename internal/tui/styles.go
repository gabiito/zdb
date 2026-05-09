package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Normal text
	StyleNormal = lipgloss.NewStyle()

	// Selected/focused item
	StyleSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	// Error text
	StyleError = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196"))

	// Dimmed / disabled hint
	StyleDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	// Read-only / warning banner (red background)
	StyleBannerRed = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("231")).
			Background(lipgloss.Color("160")).
			Padding(0, 1)

	// Warning banner (yellow background)
	StyleBannerYellow = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("232")).
			Background(lipgloss.Color("214")).
			Padding(0, 1)

	// Status bar
	StyleStatusBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	// Status bar error
	StyleStatusBarError = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Background(lipgloss.Color("236")).
				Padding(0, 1)

	// Spinner / in-flight indicator
	StyleSpinner = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205"))

	// Title bar
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	// Help text
	StyleHelp = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	// Active pane border
	StyleActiveBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("205"))

	// Inactive pane border
	StyleInactiveBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("241"))
)
