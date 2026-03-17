package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: []string{"✦", "✧", "✶", "✷", "✸", "✹", "✺", "✹", "✸", "✷", "✶", "✧"},
		FPS:    time.Second / 10,
	}
	s.Style = styleClaudeSparkle
	return s
}

func newClaudeViewport(width, height int) viewport.Model {
	vp := viewport.New(width, height)
	vp.SetContent("")
	return vp
}

func renderClaudePanel(vp *viewport.Model, sp spinner.Model, content string, running bool, active bool, width, height int, workerTabs ...string) string {
	// Build title with sparkle
	var title string
	if running {
		title = fmt.Sprintf("%s %s", sp.View(), stylePanelTitle.Render("Claude"))
	} else {
		title = fmt.Sprintf("%s %s", styleClaudeSparkle.Render("✻"), stylePanelTitle.Render("Claude"))
	}
	// Append worker tabs if present
	if len(workerTabs) > 0 && workerTabs[0] != "" {
		title += "  " + styleMuted.Render(workerTabs[0])
	}

	// Content area inside ornate border: border uses 2 cols each side + 1 space padding each side = 6
	contentW := max(width-6, 0)
	vpH := max(height-4, 0) // top border(1) + title(1) + bottom border(1) + 1 for spacing

	vp.Width = contentW
	vp.Height = vpH
	vp.SetContent(content)

	body := title + "\n" + vp.View()
	body = clampLines(body, height-2) // content area inside border

	borderColor := colorBorder
	if active {
		borderColor = colorActiveBorder
	}

	return renderOrnateClaudeBox(body, width, height, borderColor)
}

// renderOrnateClaudeBox manually constructs an ornate border around content.
//
//	╭─✦━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━✦─╮
//	│  content                                    │
//	╰─✦━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━✦─╯
func renderOrnateClaudeBox(content string, width, height int, borderColor lipgloss.Color) string {
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	accentStyle := styleClaudeSparkle

	// Inner width = total width - 2 (left border col + right border col)
	innerW := width - 2
	if innerW < 6 {
		innerW = 6
	}

	// Top border: ╭─✦━━━...━━━✦─╮
	// The accent parts: ─✦ on left, ✦─ on right = 4 chars, leaving innerW-4 for ━
	heavyFill := innerW - 4
	if heavyFill < 0 {
		heavyFill = 0
	}
	topBorder := borderStyle.Render("╭") +
		borderStyle.Render("─") + accentStyle.Render("✦") +
		borderStyle.Render(strings.Repeat("━", heavyFill)) +
		accentStyle.Render("✦") + borderStyle.Render("─") +
		borderStyle.Render("╮")

	// Bottom border: ╰─✦━━━...━━━✦─╯
	bottomBorder := borderStyle.Render("╰") +
		borderStyle.Render("─") + accentStyle.Render("✦") +
		borderStyle.Render(strings.Repeat("━", heavyFill)) +
		accentStyle.Render("✦") + borderStyle.Render("─") +
		borderStyle.Render("╯")

	// Content lines: │ content padded │
	// Padding: 1 space each side inside │, so content width = innerW - 2
	contentW := innerW - 2
	if contentW < 0 {
		contentW = 0
	}

	lines := strings.Split(content, "\n")
	// Clamp to fit inside border
	maxLines := height - 2
	if maxLines < 0 {
		maxLines = 0
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	for len(lines) < maxLines {
		lines = append(lines, "")
	}

	var sb strings.Builder
	sb.WriteString(topBorder)
	sb.WriteString("\n")

	for _, line := range lines {
		// Pad or truncate line to contentW
		padded := padRight(line, contentW)
		sb.WriteString(borderStyle.Render("│"))
		sb.WriteString(" ")
		sb.WriteString(padded)
		sb.WriteString(" ")
		sb.WriteString(borderStyle.Render("│"))
		sb.WriteString("\n")
	}

	sb.WriteString(bottomBorder)
	return sb.String()
}

// padRight pads a string with spaces to at least width w (based on visible rune count).
func padRight(s string, w int) string {
	// Use lipgloss width to account for ANSI escape sequences
	visible := lipgloss.Width(s)
	if visible >= w {
		return s
	}
	return s + strings.Repeat(" ", w-visible)
}
