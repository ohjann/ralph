# Phase 8: Daemon/Client Architecture

## Overview

Decoupled the TUI from long-running work (workers, merges, coordination) so that TUI crashes or freezes do not kill workers. Ralph now auto-forks a background daemon and the TUI connects as a disposable client over Unix socket IPC.

## What Changed

- **14 files changed**, **+3,160 / -186 lines** across 10 commits (8 stories + 2 fixes)

### New Files

| File | Purpose |
|------|---------|
| `internal/daemon/protocol.go` | Shared IPC types: `DaemonStateEvent`, `WorkerLogEvent`, `MergeResultEvent`, `StuckAlertEvent`, command requests |
| `internal/daemon/daemon.go` | Daemon core: owns coordinator, runs event loop (scheduling, merges, checkpoints, broadcasts), manages lifecycle via PID file and signal handling |
| `internal/daemon/api.go` | HTTP API over Unix socket (`/events` SSE, `/api/state`, `/api/quit`, `/api/pause`, `/api/resume`, `/api/hint`, `/api/task`, etc.) |
| `internal/daemon/client.go` | Client library: connects via Unix socket, subscribes to SSE, sends commands, handles reconnection |
| `internal/daemon/daemon_test.go` | Integration tests: connect/disconnect/reconnect, graceful quit, stale detection, hint delivery |

### Modified Files

| File | Change |
|------|--------|
| `cmd/ralph/main.go` | Auto-fork logic: detect running daemon, fork if absent, attach TUI; `--daemon` and `--kill` flags |
| `internal/coordinator/coordinator.go` | Exposed `UpdateCh()` accessor, removed BubbleTea coupling (`ListenCmd()`), serialised jj ops via `jjMu` |
| `internal/tui/model.go` | Replaced `*coordinator.Coordinator` with `*daemon.DaemonClient`; SSE stream drives the BubbleTea event loop |
| `internal/tui/commands.go` | All coordinator method calls replaced with HTTP POST commands via `DaemonClient` |
| `internal/tui/messages.go` | New `tea.Msg` types wrapping daemon events |
| `internal/tui/header.go` | Reads state from daemon client instead of coordinator |
| `internal/tui/stories_panel.go` | Reads story state from daemon client |
| `internal/worker/worker.go` | Minor interface adjustments for daemon compatibility |
| `internal/config/config.go` | New config fields for daemon socket path and PID file |

## Architecture

```
┌─────────────┐         Unix Socket          ┌──────────────────┐
│   TUI       │◄──── SSE /events ────────────│                  │
│  (client)   │───── POST /api/* ───────────►│     Daemon       │
│             │                               │                  │
│ Disposable  │    ┌──────────────────────────│  Coordinator     │
│ Can crash/  │    │                          │  Workers         │
│ reconnect   │    │  .ralph/daemon.sock      │  Merges          │
└─────────────┘    │  .ralph/daemon.pid       │  Checkpoints     │
                   │  .ralph/daemon.log       │                  │
                   └──────────────────────────└──────────────────┘
```

**Default flow:** `ralph` checks for a live daemon via `.ralph/daemon.sock` → if absent, forks `ralph --daemon` as a detached background process → waits for socket → attaches TUI as SSE client. If the TUI dies, the daemon and all workers continue. Running `ralph` again reconnects to the existing session.

## Stories

| ID | Title | Status |
|----|-------|--------|
| P8-001 | Shared protocol types for daemon IPC | Done |
| P8-002 | Coordinator cleanup: expose update channel, remove tea coupling | Done |
| P8-003 | Daemon core with coordination event loop and lifecycle | Done |
| P8-004 | Daemon HTTP API over Unix socket | Done |
| P8-005 | Client library for TUI-to-daemon communication | Done |
| P8-006 | TUI refactor: replace coordinator with daemon client | Done |
| P8-007 | Daemon lifecycle in main.go: auto-fork, attach, and --kill | Done |
| P8-008 | Integration testing: TUI disconnect/reconnect and daemon resilience | Done |

## Commits

```
09289390 story P8-008: Integration testing: TUI disconnect/reconnect and daemon resilience
81d3966c story P8-007: Daemon lifecycle in main.go: auto-fork, attach, and --kill
6c09f156 story P8-006: TUI refactor: replace coordinator with daemon client
b36f64a5 fix small tui bug
a24444fd Serialise all jj operations via jjMu to prevent concurrent sibling operations
13cc152c story P8-005: Client library for TUI-to-daemon communication
5eedb07d story P8-004: Daemon HTTP API over Unix socket
dcf77942 story P8-003: Daemon core with coordination event loop and lifecycle
f64ac5c2 story P8-002: Coordinator cleanup: expose update channel, remove tea coupling
bfcabb43 story P8-001: Shared protocol types for daemon IPC
```
