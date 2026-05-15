package tui

import (
	"runtime/debug"
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

// RenderLogo draws the zDB wordmark centered inside the given box, with a
// tagline and the running version below.
func RenderLogo(width, height int) string {
	fg := lipgloss.NewStyle().Foreground(CtpPink).Bold(true)
	tag := lipgloss.NewStyle().Foreground(CtpSubtext0)
	ver := lipgloss.NewStyle().Foreground(CtpOverlay1)

	innerWidth := lipgloss.Width(zdbWordmark[0])

	lines := []string{fg.Width(innerWidth).Render(" >_"), ""}
	for _, w := range zdbWordmark {
		lines = append(lines, fg.Render(w))
	}
	lines = append(lines,
		"",
		tag.Width(innerWidth).Align(lipgloss.Center).Render("terminal database viewer"),
		ver.Width(innerWidth).Align(lipgloss.Center).Render(versionTagForLogo()),
	)

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(strings.Join(lines, "\n"))
}

// versionTagForLogo returns the short version string shown under the logo.
// For tagged builds it returns the semver tag (e.g. "v0.2.0"); for local
// dev builds it returns "dev". A modified worktree appends "*" so the user
// can tell at a glance the binary doesn't match a clean tag.
func versionTagForLogo() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return "dev"
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.modified" && s.Value == "true" {
			v += "*"
			break
		}
	}
	return v
}
