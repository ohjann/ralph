package runner

import (
	"context"
	"fmt"

	"github.com/ohjann/ralphplusplus/internal/config"
	"github.com/ohjann/ralphplusplus/internal/roles"
)

// IterationOpts is the role+iteration descriptor passed to RunClaudeForIteration.
// It carries the per-invocation metadata that the history layer needs in order
// to name the turn files and stamp the manifest, alongside the usual RunClaude
// knobs (model, resume, system-append, tool restrictions).
type IterationOpts struct {
	StoryID         string
	Role            roles.Role
	Iter            int
	Model           string
	ResumeSessionID string
	ForkSession     bool
	SystemAppend    string
	// Disallowed is an optional override. When empty, RunClaude derives the
	// disallowed-tools list from the role default. When non-empty, this slice
	// replaces it.
	Disallowed []string
}

// RunClaudeForIteration wraps RunClaude with history capture. It opens a
// history.IterationWriter for (StoryID, Role, Iter) when cfg.HistoryRun is
// non-nil and both StoryID and Role are set, threads it into RunClaude via
// RunClaudeOpts.History, and lets RunClaude's deferred Finish record the
// session ID, token usage, and any error.
//
// When no run is active, or the call has no natural story/role to attribute,
// the wrapper is a transparent pass-through.
func RunClaudeForIteration(ctx context.Context, cfg *config.Config, projectDir, prompt, logPath string, it IterationOpts) (*RunClaudeResult, error) {
	opts := RunClaudeOpts{
		Iteration:       it.Iter,
		StoryID:         it.StoryID,
		Role:            it.Role,
		Model:           it.Model,
		ResumeSessionID: it.ResumeSessionID,
		ForkSession:     it.ForkSession,
		SystemAppend:    it.SystemAppend,
	}

	if cfg != nil && cfg.HistoryRun != nil && it.StoryID != "" && it.Role != "" {
		iw, err := cfg.HistoryRun.StartIteration(it.StoryID, string(it.Role), it.Iter, it.Model)
		if err != nil {
			return nil, fmt.Errorf("history: start iteration %s/%s#%d: %w", it.StoryID, it.Role, it.Iter, err)
		}
		opts.History = iw
	}

	return RunClaude(ctx, projectDir, prompt, logPath, opts)
}
