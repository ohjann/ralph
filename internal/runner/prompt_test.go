package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eoghanhynes/ralph/internal/prd"
	"github.com/eoghanhynes/ralph/internal/storystate"
)

func TestBuildStoryStateContextIncludesHint(t *testing.T) {
	dir := t.TempDir()
	storyID := "TEST-001"

	// Create story state so buildStoryStateContext returns content
	state := storystate.StoryState{
		StoryID:        storyID,
		Status:         storystate.StatusInProgress,
		IterationCount: 2,
		LastUpdated:    time.Now(),
	}
	if err := storystate.Save(dir, state); err != nil {
		t.Fatalf("Save state: %v", err)
	}

	// Save a hint
	hintText := "Use the existing fetchUser helper"
	if err := storystate.SaveHint(dir, storyID, hintText); err != nil {
		t.Fatalf("SaveHint: %v", err)
	}

	// Build context — should include hint
	ctx := buildStoryStateContext(dir, storyID)
	if !strings.Contains(ctx, "### User Hint") {
		t.Error("expected '### User Hint' section in context")
	}
	if !strings.Contains(ctx, hintText) {
		t.Errorf("expected hint text %q in context, got:\n%s", hintText, ctx)
	}

	// Hint should be consumed (cleared after read)
	hint, _ := storystate.LoadHint(dir, storyID)
	if hint != "" {
		t.Errorf("hint should be cleared after buildStoryStateContext, got %q", hint)
	}
}

func TestBuildStoryStateContextNoHint(t *testing.T) {
	dir := t.TempDir()
	storyID := "TEST-002"

	state := storystate.StoryState{
		StoryID:        storyID,
		Status:         storystate.StatusInProgress,
		IterationCount: 1,
		LastUpdated:    time.Now(),
	}
	if err := storystate.Save(dir, state); err != nil {
		t.Fatalf("Save state: %v", err)
	}

	ctx := buildStoryStateContext(dir, storyID)
	if strings.Contains(ctx, "User Hint") {
		t.Error("should not contain User Hint section when no hint exists")
	}
}

func TestBuildStoryStateContextEmptyStateReturnsEmpty(t *testing.T) {
	dir := t.TempDir()

	// Even if hint exists, no state means empty return (early exit on IterationCount == 0)
	storyDir := filepath.Join(dir, ".ralph", "stories", "TEST-003")
	_ = os.MkdirAll(storyDir, 0o755)
	_ = os.WriteFile(filepath.Join(storyDir, "hint.md"), []byte("some hint"), 0o644)

	ctx := buildStoryStateContext(dir, "TEST-003")
	if ctx != "" {
		t.Errorf("expected empty context for story with no state, got %q", ctx)
	}
}

func TestBuildPromptWithPRDIncludesHint(t *testing.T) {
	dir := t.TempDir()
	ralphHome := t.TempDir()
	storyID := "TEST-004"

	// Create minimal ralph-prompt.md
	if err := os.WriteFile(filepath.Join(ralphHome, "ralph-prompt.md"), []byte("base prompt"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create story state
	state := storystate.StoryState{
		StoryID:        storyID,
		Status:         storystate.StatusInProgress,
		IterationCount: 1,
		LastUpdated:    time.Now(),
	}
	if err := storystate.Save(dir, state); err != nil {
		t.Fatal(err)
	}

	// Save hint
	hintText := "try a different approach"
	if err := storystate.SaveHint(dir, storyID, hintText); err != nil {
		t.Fatal(err)
	}

	// Build with a PRD so buildStoryStateContext is called
	p := &prd.PRD{
		Project:    "test",
		BranchName: "test-branch",
		UserStories: []prd.UserStory{
			{ID: storyID, Title: "Test story", Priority: 1},
		},
	}

	prompt, _, err := BuildPrompt(ralphHome, dir, storyID, p)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if !strings.Contains(prompt, "### User Hint") {
		t.Error("BuildPrompt output should contain '### User Hint' section")
	}
	if !strings.Contains(prompt, hintText) {
		t.Errorf("BuildPrompt output should contain hint text %q", hintText)
	}

	// Hint consumed
	hint, _ := storystate.LoadHint(dir, storyID)
	if hint != "" {
		t.Errorf("hint should be consumed after BuildPrompt, got %q", hint)
	}
}

func TestBuildPromptNilPRDNoCrash(t *testing.T) {
	ralphHome := t.TempDir()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(ralphHome, "ralph-prompt.md"), []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should not crash with nil PRD
	prompt, _, err := BuildPrompt(ralphHome, dir, "ANY-001", nil)
	if err != nil {
		t.Fatalf("BuildPrompt with nil PRD: %v", err)
	}
	if !strings.Contains(prompt, "base") {
		t.Error("prompt should contain base content")
	}
}
