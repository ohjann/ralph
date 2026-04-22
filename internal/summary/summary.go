// Package summary generates the post-run SUMMARY.md by asking Claude to
// describe the completed work from the PRD and progress log.
package summary

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ohjann/ralphplusplus/internal/config"
	"github.com/ohjann/ralphplusplus/internal/memory"
	"github.com/ohjann/ralphplusplus/internal/runner"
)

// Generate removes any stale SUMMARY.md in the project, asks Claude to
// produce a fresh one from the PRD and progress log, and returns its
// contents. The file is written by Claude via the Write tool; Generate
// reads it back from disk so the caller can forward the content to the
// UI without touching the filesystem.
//
// The call records an iteration under StoryID "_summary" Role "summary"
// when cfg.HistoryRun is active, matching the pre-port TUI behavior.
func Generate(ctx context.Context, cfg *config.Config) (string, error) {
	return GenerateWith(ctx, cfg, defaultRunClaude(cfg))
}

// GenerateWith is Generate with the Claude invocation injected. Callers
// supply a RunClaudeFunc so tests can record prompts / short-circuit the
// Claude call, and so alternative attributions (e.g. utility role) can
// be chosen by the caller.
func GenerateWith(ctx context.Context, cfg *config.Config, runClaude memory.RunClaudeFunc) (string, error) {
	summaryPath := filepath.Join(cfg.ProjectDir, "SUMMARY.md")
	_ = os.Remove(summaryPath)

	prdData, _ := os.ReadFile(cfg.PRDFile)
	progressData, _ := os.ReadFile(cfg.ProgressFile)

	prompt := fmt.Sprintf(`You have just completed implementing all stories in a project. Generate a comprehensive summary of everything that was done.

Write this summary to a file called SUMMARY.md in the current working directory using the Write tool.

The summary should include:
1. **Overview** - What was built/changed (one paragraph)
2. **Stories Completed** - Brief summary of each story and what it involved
3. **Files Changed** - Key files that were added or modified (explore the recent changes)
4. **Configuration** - Any new configuration, environment variables, or setup needed
5. **Build & Run** - How to build and run the project (check for Makefile, package.json, etc.)
6. **Testing** - How to run tests, any new test files added
7. **Notes** - Any caveats, known issues, or things that need human review

Be concise but thorough. Focus on actionable information the developer needs to know.

## PRD (what was planned)
%s

## Progress Log
%s
`, string(prdData), string(progressData))

	logPath := filepath.Join(cfg.LogDir, "summary.log")
	err := runClaude(ctx, cfg.ProjectDir, prompt, logPath)

	content, _ := os.ReadFile(summaryPath)
	return string(content), err
}

func defaultRunClaude(cfg *config.Config) memory.RunClaudeFunc {
	return func(ctx context.Context, projectDir, prompt, logFilePath string) error {
		_, err := runner.RunClaudeForIteration(ctx, cfg, projectDir, prompt, logFilePath, runner.IterationOpts{
			StoryID: "_summary",
			Role:    "summary",
			Iter:    1,
		})
		return err
	}
}
