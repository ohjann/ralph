# Ralph Phase 1: Per-Story Work State & Checkpoint/Resume

## Overview

Phase 1 of Ralph v2 adds structured per-story work state persistence and crash-resilient checkpoint/resume capability. Two new packages (`internal/storystate/` and `internal/checkpoint/`) provide the infrastructure for agents to maintain structured context across iterations and for ralph to recover from interruptions. BuildPrompt() now injects PRD context directly into the prompt (eliminating the need for agents to read prd.json), and story state files are synced across jj workspaces in parallel mode. The TUI detects existing checkpoints on startup and offers resume with a y/n prompt.

## Stories Completed

| ID | Title | Status |
|----|-------|--------|
| P1-001 | Create storystate package with types and CRUD | Pass |
| P1-002 | Unit tests for storystate package | Pass |
| P1-003 | Inject PRD context and story state into BuildPrompt | Pass |
| P1-004 | Update ralph-prompt.md for story state and remove prd.json read | Pass |
| P1-005 | Copy story state to worker workspaces | Pass |
| P1-006 | Sync story state back in MergeAndSync | Pass |
| P1-007 | Create checkpoint package with types and CRUD | Pass |
| P1-008 | Unit tests for checkpoint package | Pass |
| P1-009 | Write checkpoint after story events in parallel mode | Pass |
| P1-010 | Write checkpoint after each serial iteration | Pass |
| P1-011 | Delete checkpoint on clean completion | Pass |
| P1-012 | TUI resume prompt on startup | Pass |
| P1-013 | Resume execution from checkpoint | **Fail** |
| P1-014 | Update EVOLUTION_PLAN.md to note phase 1 complete | Pass |

**13/14 stories passed.** P1-013 (resume execution logic) did not pass — see Notes.

## Files Changed

### New Packages
- `internal/storystate/storystate.go` — StoryState struct, Save/Load/LoadPlan/LoadDecisions CRUD
- `internal/storystate/storystate_test.go` — Comprehensive unit tests (round-trip, edge cases, all status values)
- `internal/checkpoint/checkpoint.go` — Checkpoint struct, Save/Load/Delete/ComputePRDHash
- `internal/checkpoint/checkpoint_test.go` — Unit tests (round-trip, hash consistency, populated maps)
- `internal/workspace/workspace_test.go` — Tests for selective story state copying

### Modified Files
- `internal/runner/runner.go` — BuildPrompt() accepts `*prd.PRD`, injects YOUR STORY/PROJECT CONTEXT/OTHER STORIES/story state sections
- `internal/tui/commands.go` — Updated BuildPrompt() call site to pass PRD
- `internal/tui/model.go` — Added phaseResumePrompt, checkpoint detection on startup, resume prompt rendering, checkpoint deletion on clean completion, serial checkpoint writes
- `internal/tui/messages.go` — Added phaseResumePrompt to phase enum
- `internal/coordinator/coordinator.go` — Checkpoint writes after story completion/failure in parallel mode, story state sync in MergeAndSync
- `internal/worker/worker.go` — Updated BuildPrompt() call site to pass PRD
- `internal/workspace/workspace.go` — Selective story state copying (only copies relevant story's state to worker)
- `ralph-prompt.md` — Removed prd.json read instruction, added Story State Management section
- `docs/EVOLUTION_PLAN.md` — Marked Phase 1 as complete

## Configuration

No new configuration, environment variables, or CLI flags were added. Story state files are stored in `.ralph/stories/{story-id}/` and checkpoint in `.ralph/checkpoint.json` — both under the existing `.ralph/` directory which is gitignored.

## Build & Run

```bash
make build          # Builds binary to build/ralph
go build ./...      # Verify compilation
```

## Testing

```bash
go test ./internal/storystate/...   # Storystate package tests
go test ./internal/checkpoint/...   # Checkpoint package tests
go test ./internal/workspace/...    # Workspace selective copy tests
go test ./...                       # All tests
make test-e2e TEST=serial-single    # E2E test (requires jj + claude)
```

## Notes

- **P1-013 did not pass**: Resume execution from checkpoint is the only story that failed. The TUI resume prompt (P1-012) works correctly — it detects checkpoints, validates PRD hash, and renders the prompt. However, the actual resume logic (restoring DAG state, syncing PRD passes, transitioning to correct execution phase) needs review and may require manual verification/fixes.
- **Story state files are agent-written**: The agent itself writes `plan.md`, `decisions.md`, and updates `state.json` during execution. Ralph reads them for context injection. This is by design per the evolution plan.
- **Prompt size optimization**: BuildPrompt() now injects ~200 tokens for a 50-story summary instead of the agent reading the full prd.json (~3K+ tokens). This keeps prompt size roughly constant regardless of PRD size.
- **Checkpoint is best-effort**: Checkpoint writes in the TUI use `_ =` to discard errors since the TUI doesn't have a logger. Checkpoint deletion on clean completion is also best-effort.
