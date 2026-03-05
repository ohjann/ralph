package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func renderHeader(m *Model, width int) string {
	// Line 1: ❖ RALPH  Iter X/Y  |  US-XXX: Title
	iterStr := fmt.Sprintf("Iter %d/%d", m.iteration, m.cfg.MaxIterations)

	storyStr := "Waiting..."
	if m.currentStoryID != "" {
		storyStr = m.currentStoryID
		if m.currentStoryTitle != "" {
			storyStr += ": " + m.currentStoryTitle
		}
		if strings.HasPrefix(m.currentStoryID, "FIX-") {
			storyStr += " " + styleDanger.Render("[AUTO-FIX]")
		}
	}
	if m.phase == phaseIdle {
		storyStr = "Idle mode"
	} else if m.phase == phaseDone {
		if m.allComplete {
			storyStr = "All stories complete!"
		} else {
			storyStr = "Max iterations reached"
		}
	}

	titleIcon := styleTitle.Render("❖")
	line1 := fmt.Sprintf("  %s %s %s  %s  │  %s",
		titleIcon,
		styleTitle.Render("RALPH"),
		styleMuted.Render(m.version),
		iterStr,
		storyStr,
	)

	// Line 2: Stories: ██░░ N/M  |  Elapsed: Xm Ys  |  Judge: ON/OFF  |  Phase
	bar := renderProgressBar(m.animatedFill, 10)
	storiesStr := fmt.Sprintf("Stories: %s %d/%d", bar, m.completedStories, m.totalStories)

	elapsed := time.Since(m.startTime).Truncate(time.Second)
	elapsedStr := fmt.Sprintf("Elapsed: %s", formatDuration(elapsed))

	judgeStr := styleJudgeOff.Render("Judge: OFF")
	if m.cfg.JudgeEnabled {
		judgeStr = styleJudgeOn.Render("Judge: ON")
	}

	phaseStr := renderPhase(m.phase)

	line2 := fmt.Sprintf("  %s  │  %s  │  %s  │  %s", storiesStr, elapsedStr, judgeStr, phaseStr)

	// Decorative separator: ┄┄┄┄┄┄┄┄ ✦ ┄┄┄┄┄┄┄┄
	sep := renderDecorativeSeparator(width)
	return lipgloss.JoinVertical(lipgloss.Left, line1, line2, sep)
}

func renderProgressBar(fillRatio float64, barWidth int) string {
	filled := int(fillRatio * float64(barWidth))
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled
	return styleProgressFilled.Render(strings.Repeat("█", filled)) +
		styleProgressEmpty.Render(strings.Repeat("░", empty))
}

func renderDecorativeSeparator(width int) string {
	accent := styleClaudeSparkle.Render("✦")
	// " ✦ " takes 3 visible characters in the center
	sideWidth := (width - 3) / 2
	if sideWidth < 0 {
		sideWidth = 0
	}
	left := strings.Repeat("┄", sideWidth)
	right := strings.Repeat("┄", width-3-sideWidth)
	return styleHeaderLine.Render(left) + " " + accent + " " + styleHeaderLine.Render(right)
}

func renderPhase(p phase) string {
	switch p {
	case phaseInit:
		return stylePhaseActive.Render("◌ Initializing")
	case phaseIterating:
		return stylePhaseActive.Render("✦ Finding story")
	case phaseClaudeRun:
		return stylePhaseActive.Render("⚡ Claude running")
	case phaseJudgeRun:
		return stylePhaseActive.Render("⚖ Judge reviewing")
	case phaseDone:
		return stylePhaseDone.Render("✓ Complete")
	case phaseIdle:
		return stylePhaseDone.Render("◇ Idle")
	default:
		return ""
	}
}

func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m > 0 {
		return fmt.Sprintf("%dm %02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
