package exec

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/ohjann/ralphplusplus/internal/costs"
)

const claudeDevilsAdvocateModel = "claude-sonnet-4-6"

const claudeDevilsAdvocateSystemPrompt = `You are an appellate reviewer for an automated build system. A primary judge has rejected this work multiple times and the implementer maintains it is complete.

Your role is narrow and specific: you do NOT re-evaluate the code. You evaluate whether the judge's repeated objections are grounded in the acceptance criteria, or whether they have become pedantic.

You are skeptical of both sides: the judge may be moving goalposts, the implementer may be genuinely incomplete. Decide based on evidence tied to the acceptance criteria — not on who you sympathize with.

You output ONLY valid JSON — no markdown fences, no commentary, no preamble, no trailing text.`

// runClaudeDevilsAdvocateOnce executes a single Claude CLI invocation for the
// appellate reviewer.
func runClaudeDevilsAdvocateOnce(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude",
		"-p",
		"--model", claudeDevilsAdvocateModel,
		"--output-format", "text",
		"--system-prompt", claudeDevilsAdvocateSystemPrompt,
	)
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return strings.TrimSpace(string(out)), nil
}

// RunClaudeDevilsAdvocate runs the appellate reviewer with retry/backoff,
// paralleling RunClaudeJudge. Returns the output text, token usage, and any
// error.
func RunClaudeDevilsAdvocate(ctx context.Context, prompt string) (string, costs.TokenUsage, error) {
	delays := []time.Duration{2 * time.Second, 4 * time.Second}
	var lastErr error
	var lastOut string

	for attempt := range 3 {
		out, err := runClaudeDevilsAdvocateOnce(ctx, prompt)
		if err == nil && out != "" {
			usage := estimateClaudeDevilsAdvocateUsage(prompt, out)
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

func estimateClaudeDevilsAdvocateUsage(input, output string) costs.TokenUsage {
	return costs.TokenUsage{
		InputTokens:  estimateTokens(input),
		OutputTokens: estimateTokens(output),
		Model:        claudeDevilsAdvocateModel,
		Provider:     "claude",
	}
}
