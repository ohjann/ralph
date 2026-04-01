package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// stuckPreventionScript is the bash hook script that tracks recent tool calls
// and denies repeated ones. It receives JSON on stdin with tool_name and
// tool_input fields. Exit code 0 = allow, exit code 2 = deny.
const stuckPreventionScript = `#!/usr/bin/env bash
# Ralph stuck-prevention hook for Claude Code PreToolUse.
# Tracks recent tool calls in a history file and denies repeated ones.
set -euo pipefail

HOOK_DIR="${RALPH_HOOK_DIR:-}"
if [ -z "$HOOK_DIR" ]; then
  exit 0
fi

HISTORY_FILE="$HOOK_DIR/tool-history.txt"
THRESHOLD="${RALPH_STUCK_THRESHOLD:-3}"

# Read JSON from stdin
INPUT=$(cat)

# Extract tool_name — try jq first, fall back to pure bash
if command -v jq &>/dev/null; then
  TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty' 2>/dev/null || true)
  TOOL_INPUT=$(echo "$INPUT" | jq -r '.tool_input // empty' 2>/dev/null || true)
else
  # Pure bash fallback: extract tool_name from JSON
  TOOL_NAME=$(echo "$INPUT" | grep -o '"tool_name"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"tool_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
  TOOL_INPUT=$(echo "$INPUT" | grep -o '"tool_input"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"tool_input"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
fi

if [ -z "$TOOL_NAME" ]; then
  exit 0
fi

# Build a key: tool_name:truncated_input (first 120 chars)
TRUNCATED="${TOOL_INPUT:0:120}"
KEY="${TOOL_NAME}:${TRUNCATED}"

# Ensure history file exists
touch "$HISTORY_FILE"

# Append this call
echo "$KEY" >> "$HISTORY_FILE"

# Count occurrences of this exact key
COUNT=$(grep -cF "$KEY" "$HISTORY_FILE" 2>/dev/null || echo 0)

if [ "$COUNT" -ge "$THRESHOLD" ]; then
  echo "BLOCKED: This exact tool call (${TOOL_NAME}) has been attempted ${COUNT} times. You are in a stuck loop. Try a fundamentally different approach: use a different tool, edit a different file, break the problem into smaller steps, or reconsider your strategy entirely."
  exit 2
fi

exit 0
`

// claudeSettingsForHook returns the .claude/settings.json content that
// registers the stuck-prevention hook as a PreToolUse command.
func claudeSettingsForHook(hookPath string) map[string]interface{} {
	return map[string]interface{}{
		"hooks": map[string]interface{}{
			"preToolUse": []interface{}{
				map[string]interface{}{
					"type":    "command",
					"command": hookPath,
				},
			},
		},
	}
}

// DeployStuckPreventionHook writes the stuck-prevention hook script to the
// workspace and configures .claude/settings.json to register it. Returns the
// path to the hook state directory (for RALPH_HOOK_DIR env var).
func DeployStuckPreventionHook(wsDir string, isFixStory bool) (hookDir string, err error) {
	// Create hook directory
	hookPath := filepath.Join(wsDir, ".ralph", "hooks", "stuck-prevention.sh")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return "", fmt.Errorf("creating hook dir: %w", err)
	}

	// Write hook script
	if err := os.WriteFile(hookPath, []byte(stuckPreventionScript), 0o755); err != nil {
		return "", fmt.Errorf("writing hook script: %w", err)
	}

	// Create/update .claude/settings.json
	claudeDir := filepath.Join(wsDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return "", fmt.Errorf("creating .claude dir: %w", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")

	// Load existing settings if present, merge hook config
	settings := make(map[string]interface{})
	if data, readErr := os.ReadFile(settingsPath); readErr == nil {
		_ = json.Unmarshal(data, &settings)
	}

	// Set hooks.preToolUse
	hookSettings := claudeSettingsForHook(hookPath)
	settings["hooks"] = hookSettings["hooks"]

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling settings: %w", err)
	}
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		return "", fmt.Errorf("writing settings.json: %w", err)
	}

	// Hook state directory (where history file lives)
	stateDir := filepath.Join(wsDir, ".ralph", "hooks")
	return stateDir, nil
}

// ResetToolHistory removes the tool history file so each RunClaude invocation
// starts with a clean slate.
func ResetToolHistory(hookDir string) {
	if hookDir == "" {
		return
	}
	historyFile := filepath.Join(hookDir, "tool-history.txt")
	_ = os.Remove(historyFile)
}

// hookEnvVars returns environment variables to set on the Claude process for
// the stuck-prevention hook. Returns nil if hookDir is empty.
func hookEnvVars(hookDir string, isFixStory bool) []string {
	if hookDir == "" {
		return nil
	}
	threshold := "3"
	if isFixStory {
		threshold = "5"
	}
	return []string{
		"RALPH_HOOK_DIR=" + hookDir,
		"RALPH_STUCK_THRESHOLD=" + threshold,
	}
}

// SetupHookEnv adds the hook-related environment variables to cmd.Env,
// preserving any existing environment. hookDir is the path returned by
// DeployStuckPreventionHook.
func SetupHookEnv(existingEnv []string, hookDir string, storyID string) []string {
	if hookDir == "" {
		return existingEnv
	}
	isFixStory := strings.HasPrefix(storyID, "FIX-")
	vars := hookEnvVars(hookDir, isFixStory)
	if existingEnv == nil {
		existingEnv = os.Environ()
	}
	return append(existingEnv, vars...)
}
