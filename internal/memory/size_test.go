package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckSize_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, ".ralph", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSize(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalBytes != 0 {
		t.Errorf("expected 0 bytes, got %d", result.TotalBytes)
	}
	if result.TokenEstimate != 0 {
		t.Errorf("expected 0 tokens, got %d", result.TokenEstimate)
	}
	if result.Level() != "ok" {
		t.Errorf("expected level ok, got %s", result.Level())
	}
}

func TestCheckSize_NoDir(t *testing.T) {
	dir := t.TempDir()
	result, err := CheckSize(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalBytes != 0 {
		t.Errorf("expected 0 bytes, got %d", result.TotalBytes)
	}
}

func TestCheckSize_KnownSizes(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, ".ralph", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write 1000 bytes
	if err := os.WriteFile(filepath.Join(memDir, "a.md"), make([]byte, 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write 3000 bytes
	if err := os.WriteFile(filepath.Join(memDir, "b.md"), make([]byte, 3000), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSize(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalBytes != 4000 {
		t.Errorf("expected 4000 bytes, got %d", result.TotalBytes)
	}
	if result.TokenEstimate != 1000 {
		t.Errorf("expected 1000 tokens, got %d", result.TokenEstimate)
	}
	if result.Level() != "ok" {
		t.Errorf("expected level ok, got %s", result.Level())
	}
}

func TestCheckSize_WarnThreshold(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, ".ralph", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 50000 tokens = 200000 bytes
	if err := os.WriteFile(filepath.Join(memDir, "big.md"), make([]byte, 200_000), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSize(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level() != "warn" {
		t.Errorf("expected level warn, got %s", result.Level())
	}
	msg := result.WarnMessage()
	if msg == "" {
		t.Error("expected non-empty warning message")
	}
}

func TestCheckSize_CritThreshold(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, ".ralph", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 150000 tokens = 600000 bytes
	if err := os.WriteFile(filepath.Join(memDir, "huge.md"), make([]byte, 600_000), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSize(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level() != "crit" {
		t.Errorf("expected level crit, got %s", result.Level())
	}
	msg := result.WarnMessage()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestSizeResult_WarnMessage_Ok(t *testing.T) {
	r := SizeResult{TokenEstimate: 100}
	if r.WarnMessage() != "" {
		t.Errorf("expected empty message for ok level, got %q", r.WarnMessage())
	}
}

func TestCheckSize_SkipsSubdirectories(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, ".ralph", "memory")
	subDir := filepath.Join(memDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(memDir, "a.md"), make([]byte, 400), 0o644); err != nil {
		t.Fatal(err)
	}
	// File inside subdir should not be counted
	if err := os.WriteFile(filepath.Join(subDir, "b.md"), make([]byte, 800), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSize(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalBytes != 400 {
		t.Errorf("expected 400 bytes (skipping subdir), got %d", result.TotalBytes)
	}
}
