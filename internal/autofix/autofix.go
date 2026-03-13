package autofix

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/eoghanhynes/ralph/internal/costs"
	rexec "github.com/eoghanhynes/ralph/internal/exec"
	"github.com/eoghanhynes/ralph/internal/prd"
	"github.com/eoghanhynes/ralph/internal/runner"
)

// GenerateFixStory uses Gemini to analyze a stuck pattern and generate a fix story.
// Returns the generated story, token usage from the Gemini call, and any error.
func GenerateFixStory(ctx context.Context, info runner.StuckInfo,
	original prd.UserStory, activityTail string) (*prd.UserStory, costs.TokenUsage, error) {

	prompt := fmt.Sprintf(`You are analyzing a stuck autonomous coding agent. Generate a short fix task.

## Original Story
ID: %s
Title: %s
Description: %s

## Stuck Pattern
Pattern: %s (repeated %d times)
Commands: %s

## Recent Activity (last 50 lines)
%s

## Instructions
Generate a JSON object with these fields:
- "title": short title for the fix task (max 80 chars)
- "description": what needs to be fixed and how
- "acceptanceCriteria": array of 1-3 concrete criteria

Focus on the ROOT CAUSE, not symptoms. If the agent is looping on a command, the fix should address WHY it fails.
Respond with ONLY the JSON object, no markdown fences.`,
		original.ID, original.Title, original.Description,
		info.Pattern, info.Count, strings.Join(info.Commands, ", "),
		activityTail)

	output, tokenUsage, err := rexec.RunGemini(ctx, prompt)
	if err != nil {
		return nil, tokenUsage, fmt.Errorf("gemini fix generation: %w", err)
	}

	// Parse the response - extract JSON
	output = strings.TrimSpace(output)
	output = strings.TrimPrefix(output, "```json")
	output = strings.TrimPrefix(output, "```")
	output = strings.TrimSuffix(output, "```")
	output = strings.TrimSpace(output)

	// Find JSON boundaries
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start < 0 || end < 0 || end <= start {
		return nil, tokenUsage, fmt.Errorf("no JSON found in gemini response")
	}

	type fixResponse struct {
		Title              string   `json:"title"`
		Description        string   `json:"description"`
		AcceptanceCriteria []string `json:"acceptanceCriteria"`
	}

	var resp fixResponse
	if err := json.Unmarshal([]byte(output[start:end+1]), &resp); err != nil {
		return nil, tokenUsage, fmt.Errorf("parsing fix response: %w", err)
	}

	fixStory := &prd.UserStory{
		ID:                 "FIX-" + original.ID,
		Title:              resp.Title,
		Description:        resp.Description,
		AcceptanceCriteria: resp.AcceptanceCriteria,
		Passes:             false,
	}

	return fixStory, tokenUsage, nil
}

// InsertFixStory loads the PRD, inserts the fix story before the given story ID, and saves.
func InsertFixStory(prdPath string, fix *prd.UserStory, beforeID string) error {
	p, err := prd.Load(prdPath)
	if err != nil {
		return err
	}

	if p.HasStory(fix.ID) {
		return nil // already exists
	}

	p.InsertBefore(beforeID, *fix)
	return prd.Save(prdPath, p)
}
