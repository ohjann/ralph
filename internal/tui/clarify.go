package tui

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ohjann/ralphplusplus/internal/debuglog"
	"github.com/ohjann/ralphplusplus/internal/prd"
)

// clarifyPromptTemplate is the prompt sent to Claude (Sonnet) to assess whether
// a task is clear enough to execute or needs clarifying questions.
// It is intentionally kept under 1K tokens.
const clarifyPromptTemplate = `You are a task clarification assistant. Assess whether the following task is clear enough to implement as a software engineering story.

Project: %s

Current stories:
%s

Task:
%s

If the task is unambiguous and actionable, respond with exactly: READY

If the task is ambiguous or missing critical details, respond with up to 3 specific clarifying questions, one per line, prefixed with "Q: ". Do NOT ask unnecessary questions — only ask if the answer would meaningfully change the implementation.

Respond with READY or questions only. No other text.`

// buildStorySummary returns a brief summary of current stories for the clarification prompt.
func buildStorySummary(stories []prd.UserStory) string {
	if len(stories) == 0 {
		return "(none)"
	}
	var sb strings.Builder
	limit := 10 // cap to keep prompt small
	for i, s := range stories {
		if i >= limit {
			fmt.Fprintf(&sb, "... and %d more\n", len(stories)-limit)
			break
		}
		status := "queued"
		if s.Passes {
			status = "done"
		}
		fmt.Fprintf(&sb, "- [%s] %s: %s\n", status, s.ID, s.Title)
	}
	return sb.String()
}

// clarifyTaskCmd invokes Claude with Sonnet model to assess task clarity.
// It returns a clarifyResultMsg with either READY or clarifying questions.
func clarifyTaskCmd(ctx context.Context, projectDir string, projectName string, taskText string, stories []prd.UserStory) tea.Cmd {
	return safeCmd(func() tea.Msg {
		summary := buildStorySummary(stories)
		prompt := fmt.Sprintf(clarifyPromptTemplate, projectName, summary, taskText)

		logDir := filepath.Join(projectDir, ".ralph", "logs")
		logPath := filepath.Join(logDir, "clarify.log")

		// Use claude CLI directly with --model sonnet for speed and plain text output
		cmd := exec.CommandContext(ctx, "claude",
			"--model", "sonnet",
			"-p",
			"--output-format", "text",
		)
		cmd.Dir = projectDir
		cmd.Stdin = strings.NewReader(prompt)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		_ = logPath // reserved for future logging

		err := cmd.Run()
		if err != nil {
			debuglog.Log("clarifyTaskCmd: claude call failed: %v (stderr: %s)", err, stderr.String())
			return clarifyResultMsg{
				TaskText: taskText,
				Err:      fmt.Errorf("clarification call failed: %w", err),
			}
		}

		output := strings.TrimSpace(stdout.String())
		debuglog.Log("clarifyTaskCmd: claude response: %s", output)

		// Parse response
		if strings.HasPrefix(strings.ToUpper(output), "READY") {
			return clarifyResultMsg{
				TaskText: taskText,
				Ready:    true,
			}
		}

		// Extract questions (lines starting with "Q: ")
		var questions []string
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Q: ") || strings.HasPrefix(line, "Q:") {
				q := strings.TrimPrefix(line, "Q: ")
				q = strings.TrimPrefix(q, "Q:")
				q = strings.TrimSpace(q)
				if q != "" {
					questions = append(questions, q)
				}
			}
		}

		// If we couldn't parse questions but got some output, treat as questions
		if len(questions) == 0 && output != "" {
			for _, line := range strings.Split(output, "\n") {
				line = strings.TrimSpace(line)
				if line != "" && len(questions) < 3 {
					questions = append(questions, line)
				}
			}
		}

		// Cap at 3 questions
		if len(questions) > 3 {
			questions = questions[:3]
		}

		return clarifyResultMsg{
			TaskText:  taskText,
			Questions: questions,
		}
	})
}
