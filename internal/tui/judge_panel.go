package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

func newJudgeViewport(width, height int) viewport.Model {
	vp := viewport.New(width, height)
	vp.SetContent("")
	return vp
}

func renderJudgePanel(vp viewport.Model, content string, active bool, width, height int) string {
	icon := lipgloss.NewStyle().Foreground(colorPrimary).Render("⚖")
	title := fmt.Sprintf("%s %s", icon, stylePanelTitle.Render("Judge"))

	style := styleSoftBorder
	if active {
		style = styleSoftBorderActive
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
