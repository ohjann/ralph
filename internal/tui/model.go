package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
	"github.com/eoghanhynes/ralph/internal/config"
	"github.com/eoghanhynes/ralph/internal/judge"
	"github.com/eoghanhynes/ralph/internal/prd"
	"github.com/eoghanhynes/ralph/internal/runner"
)

const (
	panelProgress = iota
	panelWorktree
	panelJudge
	panelClaude
	panelCount
)

type Model struct {
	cfg     *config.Config
	version string
	ctx     context.Context
	cancel  context.CancelFunc

	// State
	phase            phase
	iteration        int
	currentStoryID   string
	currentStoryTitle string
	preRevs          map[string]string
	completedStories int
	totalStories     int
	allComplete      bool
	exitCode         int
	startTime        time.Time
	confirmQuit      bool

	// Panel content
	progressContent string
	worktreeContent string
	claudeContent   string
	judgeContent    string

	// Active panel for scrolling
	activePanel int

	// Components
	progressVP  viewport.Model
	worktreeVP  viewport.Model
	judgeVP     viewport.Model
	claudeVP    viewport.Model
	spinner     spinner.Model

	// Terminal size
	width  int
	height int

	// Track if we should auto-scroll
	prevProgressLen int
	prevClaudeLen   int
	prevJudgeLen    int

	// Spring-animated progress bar
	progressSpring harmonica.Spring
	animatedFill   float64 // current animated fill ratio (0.0–1.0)
	fillVelocity   float64 // spring velocity
}

func NewModel(cfg *config.Config, version string) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	return &Model{
		cfg:            cfg,
		version:        version,
		ctx:            ctx,
		cancel:         cancel,
		phase:          phaseInit,
		startTime:      time.Now(),
		spinner:        newSpinner(),
		progressVP:     newProgressViewport(40, 10),
		worktreeVP:     newWorktreeViewport(30, 10),
		judgeVP:        newJudgeViewport(30, 10),
		claudeVP:       newClaudeViewport(80, 20),
		progressSpring: harmonica.NewSpring(harmonica.FPS(30), 6.0, 0.5),
	}
}

func (m *Model) ExitCode() int {
	return m.exitCode
}

func (m *Model) Init() tea.Cmd {
	if m.cfg.IdleMode {
		m.phase = phaseIdle
		return tea.Batch(
			m.spinner.Tick,
			fastTickCmd(),
			tickCmd(),
		)
	}
	return tea.Batch(
		archiveCmd(m.cfg),
		m.spinner.Tick,
		fastTickCmd(),
		tickCmd(),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Recompute viewport dimensions so SetContent wraps at the correct width
		available := m.height - 4 // header(3) + footer(1)
		if available < 10 {
			available = 10
		}
		topHeight := available * 35 / 100
		if topHeight < 5 {
			topHeight = 5
		}
		claudeHeight := available - topHeight

		progressWidth := m.width * 40 / 100
		worktreeWidth := m.width * 30 / 100
		judgeWidth := m.width - progressWidth - worktreeWidth

		m.progressVP.Width = progressWidth - 4 // border(2) + padding(2)
		m.progressVP.Height = topHeight - 3    // border(2) + title(1)
		m.worktreeVP.Width = worktreeWidth - 4
		m.worktreeVP.Height = topHeight - 3
		m.judgeVP.Width = judgeWidth - 4
		m.judgeVP.Height = topHeight - 3
		m.claudeVP.Width = m.width - 4
		m.claudeVP.Height = claudeHeight - 3

		return m, nil

	case tea.KeyMsg:
		switch {
		case msg.String() == "ctrl+c":
			m.cancel()
			return m, tea.Quit
		case msg.String() == "q":
			if m.confirmQuit || m.phase == phaseDone || m.phase == phaseIdle {
				m.cancel()
				return m, tea.Quit
			}
			m.confirmQuit = true
			return m, nil
		case msg.String() == "tab":
			m.activePanel = (m.activePanel + 1) % panelCount
			return m, nil
		case msg.String() == "j" || msg.String() == "down":
			switch m.activePanel {
			case panelProgress:
				m.progressVP.LineDown(1)
			case panelWorktree:
				m.worktreeVP.LineDown(1)
			case panelJudge:
				m.judgeVP.LineDown(1)
			case panelClaude:
				m.claudeVP.LineDown(1)
			}
			return m, nil
		case msg.String() == "k" || msg.String() == "up":
			switch m.activePanel {
			case panelProgress:
				m.progressVP.LineUp(1)
			case panelWorktree:
				m.worktreeVP.LineUp(1)
			case panelJudge:
				m.judgeVP.LineUp(1)
			case panelClaude:
				m.claudeVP.LineUp(1)
			}
			return m, nil
		case msg.String() == "pgdown":
			switch m.activePanel {
			case panelProgress:
				m.progressVP.ViewDown()
			case panelWorktree:
				m.worktreeVP.ViewDown()
			case panelJudge:
				m.judgeVP.ViewDown()
			case panelClaude:
				m.claudeVP.ViewDown()
			}
			return m, nil
		case msg.String() == "pgup":
			switch m.activePanel {
			case panelProgress:
				m.progressVP.ViewUp()
			case panelWorktree:
				m.worktreeVP.ViewUp()
			case panelJudge:
				m.judgeVP.ViewUp()
			case panelClaude:
				m.claudeVP.ViewUp()
			}
			return m, nil
		default:
			m.confirmQuit = false
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	// --- Fast tick: poll activity + progress ---
	case fastTickMsg:
		cmds = append(cmds, fastTickCmd())
		cmds = append(cmds, pollProgressCmd(m.cfg.ProgressFile))

		// Update spring-animated progress bar
		target := 0.0
		if m.totalStories > 0 {
			target = float64(m.completedStories) / float64(m.totalStories)
		}
		m.animatedFill, m.fillVelocity = m.progressSpring.Update(
			m.animatedFill, m.fillVelocity, target,
		)
		if m.phase == phaseClaudeRun || m.phase == phaseJudgeRun {
			activityPath := runner.ActivityFilePath(m.cfg.LogDir, m.iteration)
			cmds = append(cmds, pollActivityCmd(activityPath))
		}

	// --- Slow tick: poll worktree + prd ---
	case tickMsg:
		cmds = append(cmds, tickCmd())
		cmds = append(cmds, pollWorktreeCmd(m.ctx, m.cfg.ProjectDir))
		cmds = append(cmds, reloadPRDCmd(m.cfg.PRDFile))

	// --- Data updates ---
	case progressContentMsg:
		m.progressContent = msg.Content
		newLen := len(msg.Content)
		m.progressVP.SetContent(msg.Content)
		// Auto-scroll if new content
		if newLen > m.prevProgressLen {
			m.progressVP.GotoBottom()
		}
		m.prevProgressLen = newLen

	case worktreeMsg:
		m.worktreeContent = msg.Content

	case claudeActivityMsg:
		m.claudeContent = msg.Content
		newLen := len(msg.Content)
		m.claudeVP.SetContent(msg.Content)
		if newLen > m.prevClaudeLen {
			m.claudeVP.GotoBottom()
		}
		m.prevClaudeLen = newLen

	case prdReloadedMsg:
		m.completedStories = msg.CompletedCount
		m.totalStories = msg.TotalCount

	// --- Phase transitions ---
	case archiveDoneMsg:
		m.phase = phaseIterating
		cmds = append(cmds, findNextStoryCmd(m.cfg.PRDFile))

	case nextStoryMsg:
		if msg.AllDone {
			m.phase = phaseDone
			m.allComplete = true
			m.exitCode = 0
			return m, nil
		}
		m.iteration++
		if m.iteration > m.cfg.MaxIterations {
			m.phase = phaseDone
			m.allComplete = false
			m.exitCode = 1
			return m, nil
		}
		m.currentStoryID = msg.StoryID
		m.currentStoryTitle = msg.StoryTitle
		m.phase = phaseClaudeRun
		m.claudeContent = ""
		m.prevClaudeLen = 0

		// Capture revision for judge diff baseline
		if m.cfg.JudgeEnabled {
			dirs := []string{m.cfg.ProjectDir}
			if p, err := prd.Load(m.cfg.PRDFile); err == nil {
				for _, r := range p.Repos {
					if filepath.IsAbs(r) {
						dirs = append(dirs, r)
					} else {
						dirs = append(dirs, filepath.Join(m.cfg.ProjectDir, r))
					}
				}
			}
			m.preRevs = captureRevsCmd(m.ctx, dirs)
		}

		cmds = append(cmds, runClaudeCmd(m.ctx, m.cfg, msg.StoryID, m.iteration))

	case claudeDoneMsg:
		if msg.Err != nil {
			// Context cancelled = user quit
			if m.ctx.Err() != nil {
				return m, tea.Quit
			}
			// Show Claude error in activity panel
			m.claudeContent += fmt.Sprintf("\n── Claude Error ──\n%s\n", msg.Err)
			m.claudeVP.SetContent(m.claudeContent)
			m.claudeVP.GotoBottom()
			m.prevClaudeLen = len(m.claudeContent)
		}

		if msg.CompleteSignal {
			m.phase = phaseDone
			m.allComplete = true
			m.exitCode = 0
			return m, nil
		}

		// Judge check
		if m.cfg.JudgeEnabled && m.currentStoryID != "" {
			// Check if story now passes
			cmds = append(cmds, m.handleJudgeCheck())
			if len(cmds) > 0 {
				return m, tea.Batch(cmds...)
			}
		}

		// No judge or story didn't pass yet — next iteration
		m.phase = phaseIterating
		cmds = append(cmds, findNextStoryCmd(m.cfg.PRDFile))

	case judgeDoneMsg:
		// Show judge result in the judge panel
		m.judgeContent += judge.FormatResult(m.currentStoryID, msg.Result)
		newJudgeLen := len(m.judgeContent)
		m.judgeVP.SetContent(m.judgeContent)
		if newJudgeLen > m.prevJudgeLen {
			m.judgeVP.GotoBottom()
		}
		m.prevJudgeLen = newJudgeLen

		// Persist judge result to progress.txt
		judge.AppendJudgeResult(m.cfg.ProgressFile, m.currentStoryID, msg.Result)

		if msg.Result.Passed {
			judge.ClearRejectionCount(m.cfg.ProjectDir, m.currentStoryID)
		} else {
			judge.IncrementRejectionCount(m.cfg.ProjectDir, m.currentStoryID)
		}
		// Either way, move to next iteration
		m.phase = phaseIterating
		cmds = append(cmds, findNextStoryCmd(m.cfg.PRDFile))
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleJudgeCheck() tea.Cmd {
	// Reload PRD to check if story passes
	p, err := prd.Load(m.cfg.PRDFile)
	if err != nil {
		return nil
	}
	story := p.FindStory(m.currentStoryID)
	if story == nil || !story.Passes {
		return nil
	}

	// Story claims to pass — run judge
	rejections := judge.GetRejectionCount(m.cfg.ProjectDir, m.currentStoryID)
	if rejections >= m.cfg.JudgeMaxRejections {
		// Auto-pass
		judge.AppendAutoPass(m.cfg.ProgressFile, m.currentStoryID, rejections)
		judge.ClearRejectionCount(m.cfg.ProjectDir, m.currentStoryID)
		m.judgeContent += fmt.Sprintf("\n── Judge: %s ── AUTO-PASS after %d rejections [HUMAN REVIEW NEEDED] ──\n", m.currentStoryID, rejections)
		m.judgeVP.SetContent(m.judgeContent)
		m.judgeVP.GotoBottom()
		m.prevJudgeLen = len(m.judgeContent)
		return nil
	}

	m.phase = phaseJudgeRun
	m.judgeContent += "\n── Judge reviewing " + m.currentStoryID + "... ──\n"
	m.judgeVP.SetContent(m.judgeContent)
	m.judgeVP.GotoBottom()
	m.prevJudgeLen = len(m.judgeContent)
	return runJudgeCmd(m.ctx, m.cfg, m.currentStoryID, m.preRevs)
}

func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Layout: header(3) + top panels + claude activity + footer(1)
	// Reserve exact line counts for fixed elements
	headerHeight := 3
	footerHeight := 1
	available := m.height - headerHeight - footerHeight
	if available < 10 {
		available = 10
	}

	// Split: 35% top panels, 65% claude activity
	topHeight := available * 35 / 100
	if topHeight < 5 {
		topHeight = 5
	}
	claudeHeight := available - topHeight

	progressWidth := m.width * 40 / 100
	worktreeWidth := m.width * 30 / 100
	judgeWidth := m.width - progressWidth - worktreeWidth

	// Render sections
	header := renderHeader(m, m.width)

	progressPanel := renderProgressPanel(
		m.progressVP,
		m.activePanel == panelProgress,
		progressWidth,
		topHeight,
	)

	worktreePanel := renderWorktreePanel(
		m.worktreeVP,
		m.worktreeContent,
		m.activePanel == panelWorktree,
		worktreeWidth,
		topHeight,
	)

	judgePanel := renderJudgePanel(
		m.judgeVP,
		m.judgeContent,
		m.activePanel == panelJudge,
		judgeWidth,
		topHeight,
	)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, progressPanel, worktreePanel, judgePanel)

	claudeRunning := m.phase == phaseClaudeRun || m.phase == phaseJudgeRun
	claudePanel := renderClaudePanel(
		m.claudeVP,
		m.spinner,
		m.claudeContent,
		claudeRunning,
		m.activePanel == panelClaude,
		m.width,
		claudeHeight,
	)

	footer := renderFooter(m.width, m.confirmQuit, m.phase == phaseDone, m.phase == phaseIdle)

	output := lipgloss.JoinVertical(lipgloss.Left,
		header,
		topRow,
		claudePanel,
		footer,
	)

	// Clamp to exactly terminal height to prevent scrolling/jitter
	lines := strings.Split(output, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	return strings.Join(lines, "\n")
}

// clampLines truncates or pads a string to exactly n lines.
func clampLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	for len(lines) < n {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func renderFooter(width int, confirmQuit bool, done bool, idle bool) string {
	if confirmQuit {
		return "  " + styleQuitConfirm.Render("Press q again to quit, any other key to cancel")
	}
	help := styleKey.Render("q") + styleFooter.Render(": quit  ") +
		styleKey.Render("tab") + styleFooter.Render(": switch panel  ") +
		styleKey.Render("j/k") + styleFooter.Render(": scroll")
	if idle {
		return "  " + styleMuted.Render("Idle — ") + help
	}
	if done {
		return "  " + styleSuccess.Render("Run complete — ") + help
	}
	return "  " + help
}
