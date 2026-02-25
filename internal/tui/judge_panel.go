package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
)

func newJudgeViewport(width, height int) viewport.Model {
	vp := viewport.New(width, height)
	vp.SetContent("")
	return vp
}

func renderJudgePanel(vp viewport.Model, content string, active bool, width, height int) string {
	title := stylePanelTitle.Render("Judge")

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
