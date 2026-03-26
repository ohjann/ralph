package exec

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/eoghanhynes/ralph/internal/costs"
)

const claudeJudgeModel = "claude-sonnet-4-6"

const claudeJudgeSystemPrompt = `You are a senior staff engineer acting as an independent code judge for an automated build system. You have decades of experience shipping production software and reviewing pull requests.

Your role is strictly adversarial-but-fair: you protect the codebase from incomplete work, but you never block solid implementations over stylistic nits. You are pragmatic, not pedantic.

You output ONLY valid JSON — no markdown fences, no commentary, no preamble, no trailing text.`

// runClaudeJudgeOnce executes a single Claude CLI invocation for the judge.
func runClaudeJudgeOnce(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude",
		"-p",
		"--model", claudeJudgeModel,
		"--output-format", "text",
		"--system-prompt", claudeJudgeSystemPrompt,
	)
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return strings.TrimSpace(string(out)), nil
}

// RunClaudeJudge runs Claude with retry and exponential backoff (3 attempts, 2s/4s delays).
// Returns the output text, token usage, and any error.
func RunClaudeJudge(ctx context.Context, prompt string) (string, costs.TokenUsage, error) {
	delays := []time.Duration{2 * time.Second, 4 * time.Second}
	var lastErr error
	var lastOut string

	for attempt := range 3 {
		out, err := runClaudeJudgeOnce(ctx, prompt)
		if err == nil && out != "" {
			usage := estimateClaudeJudgeUsage(prompt, out)
			return out, usage, nil
		}
		lastErr = err
		lastOut = out

		if attempt < 2 {
			select {
			case <-ctx.Done():
				return lastOut, costs.TokenUsage{}, ctx.Err()
			case <-time.After(delays[attempt]):
			}
		}
	}

	if lastErr != nil {
		return lastOut, costs.TokenUsage{}, lastErr
	}
	return lastOut, costs.TokenUsage{}, nil
}

// estimateClaudeJudgeUsage estimates token usage from text lengths (~4 chars per token).
func estimateClaudeJudgeUsage(input, output string) costs.TokenUsage {
	return costs.TokenUsage{
		InputTokens:  estimateTokens(input),
		OutputTokens: estimateTokens(output),
		Model:        claudeJudgeModel,
		Provider:     "claude",
	}
}
