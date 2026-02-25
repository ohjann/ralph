package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
)

func newProgressViewport(width, height int) viewport.Model {
	vp := viewport.New(width, height)
	vp.SetContent("")
	return vp
}

func renderProgressPanel(vp viewport.Model, active bool, width, height int) string {
	icon := styleSuccess.Render("◈")
	title := fmt.Sprintf("%s %s", icon, stylePanelTitle.Render("Progress"))

	style := styleSoftBorder
	if active {
		style = styleSoftBorderActive
	}

	// Content area: inside border (2) + padding (2 horizontal, 0 vertical)
	contentW := max(width-4, 0)
	vpH := max(height-3, 0) // border (2) + title (1)

	vp.Width = contentW
	vp.Height = vpH

	body := title + "\n" + vp.View()

	// Hard-enforce exact line count to prevent any overflow
	body = clampLines(body, height-2) // content area inside border

	return style.MaxHeight(height).Render(body)
}
