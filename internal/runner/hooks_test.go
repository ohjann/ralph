package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployStuckPreventionHook(t *testing.T) {
	tmpDir := t.TempDir()

	hookDir, err := DeployStuckPreventionHook(tmpDir, false)
	if err != nil {
		t.Fatalf("DeployStuckPreventionHook: %v", err)
	}

	// Verify hook script exists and is executable
	hookPath := filepath.Join(tmpDir, ".ralph", "hooks", "stuck-prevention.sh")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("hook script not found: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("hook script is not executable: %v", info.Mode())
	}

	// Verify .claude/settings.json exists and has correct content
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not found: %v", err)
	}
	settingsStr := string(data)
	if !strings.Contains(settingsStr, "preToolUse") {
		t.Errorf("settings.json missing preToolUse hook config")
	}
	if !strings.Contains(settingsStr, "stuck-prevention.sh") {
		t.Errorf("settings.json missing hook path")
	}

	// Verify hookDir is correct
	expectedHookDir := filepath.Join(tmpDir, ".ralph", "hooks")
	if hookDir != expectedHookDir {
		t.Errorf("hookDir = %q, want %q", hookDir, expectedHookDir)
	}
}

func TestResetToolHistory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a history file
	historyFile := filepath.Join(tmpDir, "tool-history.txt")
	if err := os.WriteFile(historyFile, []byte("Edit:foo.go\nEdit:foo.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ResetToolHistory(tmpDir)

	if _, err := os.Stat(historyFile); !os.IsNotExist(err) {
		t.Errorf("history file should be removed after reset")
	}
}

func TestSetupHookEnv(t *testing.T) {
	env := SetupHookEnv(nil, "/tmp/hooks", "P7-006")
	found := false
	for _, v := range env {
		if v == "RALPH_HOOK_DIR=/tmp/hooks" {
			found = true
		}
	}
	if !found {
		t.Errorf("RALPH_HOOK_DIR not found in env")
	}

	// FIX- story should get threshold 5
	env = SetupHookEnv([]string{}, "/tmp/hooks", "FIX-001")
	foundThreshold := false
	for _, v := range env {
		if v == "RALPH_STUCK_THRESHOLD=5" {
			foundThreshold = true
		}
	}
	if !foundThreshold {
		t.Errorf("RALPH_STUCK_THRESHOLD=5 not found for FIX- story")
	}

	// Regular story should get threshold 3
	env = SetupHookEnv([]string{}, "/tmp/hooks", "P7-006")
	foundThreshold = false
	for _, v := range env {
		if v == "RALPH_STUCK_THRESHOLD=3" {
			foundThreshold = true
		}
	}
	if !foundThreshold {
		t.Errorf("RALPH_STUCK_THRESHOLD=3 not found for regular story")
	}
}

func TestDeployPreservesExistingSettings(t *testing.T) {
	tmpDir := t.TempDir()

	// Write existing settings
	claudeDir := filepath.Join(tmpDir, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"existingKey": "value"}`), 0o644)

	_, err := DeployStuckPreventionHook(tmpDir, false)
	if err != nil {
		t.Fatalf("DeployStuckPreventionHook: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	settingsStr := string(data)
	if !strings.Contains(settingsStr, "existingKey") {
		t.Errorf("existing settings key was lost")
	}
	if !strings.Contains(settingsStr, "preToolUse") {
		t.Errorf("hook config not added")
	}
}
