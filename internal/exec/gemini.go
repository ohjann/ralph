package exec

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"github.com/eoghanhynes/ralph/internal/costs"
)

// GeminiResult holds the output text and token usage from a Gemini invocation.
type GeminiResult struct {
	Output     string
	TokenUsage costs.TokenUsage
}

// geminiModel is the default model name used for Gemini CLI invocations.
const geminiModel = "gemini-2.5-pro"

// usageMetadata matches the Gemini API response usage metadata structure.
type usageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// geminiJSONResponse represents a Gemini API JSON response containing usage metadata.
type geminiJSONResponse struct {
	UsageMetadata *usageMetadata `json:"usageMetadata"`
}

// runGeminiOnce executes a single gemini invocation.
func runGeminiOnce(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "gemini", "-m", geminiModel, "-p", prompt, "-o", "text")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return strings.TrimSpace(string(out)), nil
}

// RunGemini runs gemini with retry and exponential backoff (3 attempts, 2s/4s delays).
// Returns the output text, token usage, and any error.
func RunGemini(ctx context.Context, prompt string) (string, costs.TokenUsage, error) {
	delays := []time.Duration{2 * time.Second, 4 * time.Second}
	var lastErr error
	var lastOut string

	for attempt := range 3 {
		out, err := runGeminiOnce(ctx, prompt)
		if err == nil && out != "" {
			usage := parseOrEstimateUsage(prompt, out)
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

// GeminiAvailable checks whether the gemini CLI is on PATH.
func GeminiAvailable() bool {
	_, err := exec.LookPath("gemini")
	return err == nil
}

// parseOrEstimateUsage tries to extract usageMetadata from Gemini output JSON.
// Falls back to estimation from text length if metadata is not present.
func parseOrEstimateUsage(prompt, output string) costs.TokenUsage {
	// Try to parse usage metadata from the output (if Gemini returned JSON with it)
	if usage, ok := parseUsageMetadata(output); ok {
		return costs.TokenUsage{
			InputTokens:  usage.PromptTokenCount,
			OutputTokens: usage.CandidatesTokenCount,
			Model:        geminiModel,
			Provider:     "gemini",
		}
	}

	// Fallback: estimate tokens from text length (~4 chars per token)
	return EstimateGeminiUsage(prompt, output)
}

// parseUsageMetadata attempts to extract usageMetadata from a JSON response.
func parseUsageMetadata(output string) (*usageMetadata, bool) {
	// Look for usageMetadata in the output
	if !strings.Contains(output, "usageMetadata") {
		return nil, false
	}

	// Try direct JSON parse
	var resp geminiJSONResponse
	if err := json.Unmarshal([]byte(output), &resp); err == nil && resp.UsageMetadata != nil {
		return resp.UsageMetadata, true
	}

	// Try extracting JSON block
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(output[start:end+1]), &resp); err == nil && resp.UsageMetadata != nil {
			return resp.UsageMetadata, true
		}
	}

	return nil, false
}

// EstimateGeminiUsage estimates token usage from text lengths.
// Uses ~4 characters per token as a rough approximation.
func EstimateGeminiUsage(input, output string) costs.TokenUsage {
	return costs.TokenUsage{
		InputTokens:  estimateTokens(input),
		OutputTokens: estimateTokens(output),
		Model:        geminiModel,
		Provider:     "gemini",
	}
}

// estimateTokens estimates token count from text length (~4 chars per token).
func estimateTokens(text string) int {
	n := len(text) / 4
	if n == 0 && len(text) > 0 {
		n = 1
	}
	return n
}
