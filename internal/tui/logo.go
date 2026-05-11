package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// LogoMinWidth is the minimum total terminal width required before the
// connection picker shows the right-hand logo panel.
const LogoMinWidth = 100

// LogoPanelWidth is the fixed width the logo panel occupies inside the
// horizontal split. The list claims everything else.
const LogoPanelWidth = 40

// ANSI-shadow wordmark, rendered in the bright accent.
var zdbWordmark = []string{
	" ███████╗ ██████╗  ██████╗ ",
	" ╚══███╔╝ ██╔══██╗ ██╔══██╗",
	"   ███╔╝  ██║  ██║ ██████╔╝",
	"  ███╔╝   ██║  ██║ ██╔══██╗",
	" ███████╗ ██████╔╝ ██████╔╝",
	" ╚══════╝ ╚═════╝  ╚═════╝ ",
}

// RenderLogo draws the zDB wordmark centered inside the given box.
func RenderLogo(width, height int) string {
	fg := lipgloss.NewStyle().Foreground(CtpPink).Bold(true)
	tag := lipgloss.NewStyle().Foreground(CtpSubtext0)

	innerWidth := lipgloss.Width(zdbWordmark[0])

	lines := []string{fg.Width(innerWidth).Render(" >_"), ""}
	for _, w := range zdbWordmark {
		lines = append(lines, fg.Render(w))
	}
	lines = append(lines, "", tag.Width(innerWidth).Render("   terminal database viewer"))

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(strings.Join(lines, "\n"))
}
