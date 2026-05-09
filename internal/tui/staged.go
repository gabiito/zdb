package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StagedViewModel renders the staged-changes overlay with internal scrolling
// via bubbles/viewport. Constructed fresh each time the modal is opened so
// the diff reflects current state; the viewport handles ↑/↓/PgUp/PgDn etc.
type StagedViewModel struct {
	viewport viewport.Model
	count    int
	width    int
	height   int
}

// NewStagedViewModel builds a fresh staged-view modal sized for the given
// terminal width/height. width/height are the FULL terminal dimensions; the
// modal sizes its inner box accordingly.
func NewStagedViewModel(diff string, count, width, height int) StagedViewModel {
	boxW, _, contentH := stagedBoxDims(width, height)

	vp := viewport.New(boxW-4, contentH) // -4 for padding(1,2)
	if count == 0 {
		vp.SetContent(StyleDim.Render("(no staged changes)"))
	} else {
		vp.SetContent(diff)
	}

	return StagedViewModel{
		viewport: vp,
		count:    count,
		width:    width,
		height:   height,
	}
}

// stagedBoxDims clamps the modal box dimensions to a sensible range and
// returns the inner content height available for the viewport.
func stagedBoxDims(termW, termH int) (boxW, boxH, contentH int) {
	boxW = termW - 8
	if boxW < 40 {
		boxW = 40
	}
	if boxW > 100 {
		boxW = 100
	}
	boxH = termH - 6
	if boxH < 7 {
		boxH = 7
	}
	if boxH > 30 {
		boxH = 30
	}
	// Reserve: title (1) + spacer (1) + viewport content + spacer (1) + hint (1)
	// + top/bottom border (2) + top/bottom padding (2) = 8 lines of chrome.
	contentH = boxH - 8
	if contentH < 1 {
		contentH = 1
	}
	return boxW, boxH, contentH
}

// Update routes scroll keys to the inner viewport. Action keys (s/D/Esc) are
// handled at the App level and never reach this method.
func (m StagedViewModel) Update(msg tea.Msg) (StagedViewModel, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the bordered modal box.
func (m StagedViewModel) View() string {
	title := fmt.Sprintf("Staged changes (%d)", m.count)

	pct := ""
	if m.viewport.TotalLineCount() > m.viewport.Height {
		pct = fmt.Sprintf(" — %.0f%%", m.viewport.ScrollPercent()*100)
	}
	hint := StyleHelp.Render("↑/↓ scroll" + pct + " · s save · D discard · Esc close")

	boxW, _, _ := stagedBoxDims(m.width, m.height)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Padding(1, 2).
		Width(boxW).
		Render(
			StyleTitle.Render(title) + "\n\n" +
				m.viewport.View() + "\n\n" +
				hint,
		)
}
