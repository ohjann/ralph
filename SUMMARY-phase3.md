# Ralph Phase 3: Cost Tracking, TUI Analytics & Remote Monitoring

## Overview

Phase 3 adds full cost visibility, run history, push notifications, and remote monitoring to Ralph. Token usage is parsed from Claude and Gemini streaming output, aggregated per-story and per-run, and surfaced in both a new TUI costs tab and the header bar. Run summaries are persisted to `.ralph/run-history.json` and accessible via the `ralph history` subcommand. Push notifications via ntfy.sh fire on key events (story complete/fail/stuck, run done), and a mobile-friendly HTTP status page with Server-Sent Events provides live remote monitoring — designed for access over Tailscale. Cost data is included in checkpoints for accurate resume totals.

## Stories Completed

| ID | Title | Status |
|----|-------|--------|
| P3-001 | Create cost tracking package with types and pricing table | Pass |
| P3-002 | Unit tests for cost tracking package | Pass |
| P3-003 | Parse token usage from Claude streaming JSON output | Pass |
| P3-004 | Track Gemini token usage for judge and autofix | Pass |
| P3-005 | TUI costs context tab with per-story breakdown | Pass |
| P3-006 | Cost summary in TUI header bar | Pass |
| P3-007 | Run history persistence | Pass |
| P3-008 | ralph history subcommand and startup summary | Pass |
| P3-009 | Push notifications via ntfy.sh | Pass |
| P3-010 | Integrate notifications into TUI and coordinator event flow | Pass |
| P3-011 | Remote status page HTTP server with SSE | Pass |
| P3-012 | Wire status page into TUI event flow | Pass |
| P3-013 | Include cost data in checkpoint for resume accuracy | Pass |

**13/13 stories passed.**

## Files Changed

### New Packages
- `internal/costs/costs.go` — TokenUsage, ModelPricing, PricingTable, IterationCost, StoryCosting, RunCosting types; CalculateCost, NewRunCosting, AddIteration, AddJudgeCost, CacheHitRate methods; DefaultPricing for Claude and Gemini models; thread-safe with sync.Mutex
- `internal/costs/costs_test.go` — Unit tests for cost calculations, token accumulation, cache hit rate, multi-story independence
- `internal/costs/history.go` — RunSummary, RunHistory types; LoadHistory, SaveHistory, AppendRun for `.ralph/run-history.json` persistence
- `internal/notify/notify.go` — Notifier struct with ntfy.sh HTTP POST integration; StoryComplete, StoryFailed, StoryStuck, RunComplete, Error helpers; fire-and-forget goroutine pattern
- `internal/statuspage/statuspage.go` — StatusServer with mobile-friendly HTML status page, SSE live updates via `/events`, JSON API at `/api/status`; responsive dark theme; concurrent SSE client fan-out

### Modified Files
- `internal/runner/runner.go` — Stream processor parses `message_start`/`message_delta` usage fields; accumulates TokenUsage across streaming response; extracts model name; returns final TokenUsage; CostUpdateMsg defined and sent after each iteration
- `internal/tui/model.go` — RunCosting, Notifier, StatusServer fields; cost updates in header bar; notification sends on serial story events; status server start/stop; CostUpdateMsg handler; run history summary on startup
- `internal/tui/context_panel.go` — contextCosts mode with per-story cost breakdown, token counts (K/M suffix formatting), cache hit rate percentage
- `internal/tui/messages.go` — CostUpdateMsg type added
- `internal/coordinator/coordinator.go` — SetRunCosting, SetNotifier methods; cost/notification propagation to parallel workers; worker completion notifications with cost data
- `internal/worker/worker.go` — Cost tracking per worker iteration; CostUpdateMsg sent back to TUI
- `internal/config/config.go` — `--notify`, `--ntfy-server`, `--status-port` CLI flags
- `internal/checkpoint/checkpoint.go` — CostData field for checkpoint persistence; snapshot/restore of RunCosting
- `internal/exec/gemini.go` — Parse usageMetadata from Gemini responses; convert to TokenUsage
- `internal/judge/judge.go` — Return TokenUsage from judge invocations
- `cmd/ralph/main.go` — `ralph history` subcommand (last 10 runs, `--all` flag)

## Configuration

### New CLI Flags
| Flag | Description | Default |
|------|-------------|---------|
| `--notify <topic>` | Enable push notifications via ntfy.sh to given topic | disabled |
| `--ntfy-server <url>` | Self-hosted ntfy server URL | `https://ntfy.sh` |
| `--status-port <port>` | Start remote status page HTTP server on given port | disabled |

### No Environment Variables Added
Cost tracking is automatic — no configuration needed. Notifications and status page are opt-in via CLI flags.

## Build & Run

```bash
make build          # Builds binary to build/ralph

# Run with cost tracking (automatic)
./build/ralph

# Run with notifications
./build/ralph --notify my-secret-topic

# Run with remote status page
./build/ralph --status-port 8080

# Run with all Phase 3 features
./build/ralph --notify my-topic --status-port 8080

# View run history
./build/ralph history
./build/ralph history --all
```

## Testing

```bash
go test ./internal/costs/...       # Cost tracking tests
go test ./...                      # All tests
make build                         # Verify compilation
```

## Notes

- **Thread safety**: RunCosting uses sync.Mutex for all mutating methods — safe for concurrent access from parallel workers.
- **Graceful degradation**: Notifications and status page are fully optional. If ntfy.sh is unreachable, errors are logged but execution continues. If the status port is in use, a warning is logged and ralph continues without it.
- **Checkpoint integration**: Cost data persists across resume operations via CostData in checkpoint.json. Older checkpoints without cost data start fresh at zero.
- **Pricing table**: Default pricing covers Claude Opus ($15/$75 per MTok), Sonnet ($3/$15), Haiku ($0.25/$1.25) and Gemini 2.5 Pro ($1.25/$10), Flash ($0.15/$0.60). Easy to update when model prices change.
- **ntfy.sh**: Zero-account, open-source push notification service. Users install the ntfy app on their phone and subscribe to a secret topic string. Self-hosted ntfy instances supported via `--ntfy-server`.
- **Status page**: Designed for Tailscale access — access at `http://<laptop-tailscale-ip>:<port>` from any device on your tailnet. Dark theme, responsive layout, live SSE updates.
