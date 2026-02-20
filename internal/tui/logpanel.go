package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
)

func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = stylePhaseActive
	return s
}

func newClaudeViewport(width, height int) viewport.Model {
	vp := viewport.New(width, height)
	vp.SetContent("")
	return vp
}

func renderClaudePanel(vp viewport.Model, sp spinner.Model, content string, running bool, active bool, width, height int) string {
	title := stylePanelTitle.Render("Claude Activity")
	if running {
		title = fmt.Sprintf("%s %s", title, sp.View())
	}

	style := stylePanelBorder
	if active {
		style = stylePanelBorderActive
	}

	contentW := max(width-4, 0)
	vpH := max(height-3, 0)

	vp.Width = contentW
	vp.Height = vpH
	vp.SetContent(content)

	body := title + "\n" + vp.View()

	// Hard-enforce exact line count to prevent any overflow
	body = clampLines(body, height-2)

	return style.MaxHeight(height).Render(body)
}
