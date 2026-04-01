package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunMetaIncrementAndReset verifies that the run counter increments correctly
// across multiple calls and resets to zero after SaveRunMeta simulates a dream reset.
func TestRunMetaIncrementAndReset(t *testing.T) {
	dir := t.TempDir()

	// Fresh project — counter should start at 0
	meta, err := LoadRunMeta(dir)
	if err != nil {
		t.Fatalf("LoadRunMeta on empty dir: %v", err)
	}
	if meta.RunCount != 0 {
		t.Fatalf("expected initial run count 0, got %d", meta.RunCount)
	}

	// Increment 5 times
	for i := 1; i <= 5; i++ {
		count, err := IncrementRunCount(dir)
		if err != nil {
			t.Fatalf("IncrementRunCount (iteration %d): %v", i, err)
		}
		if count != i {
			t.Errorf("after increment %d: expected count %d, got %d", i, i, count)
		}
	}

	// Verify persisted value
	meta, err = LoadRunMeta(dir)
	if err != nil {
		t.Fatalf("LoadRunMeta after increments: %v", err)
	}
	if meta.RunCount != 5 {
		t.Errorf("expected persisted run count 5, got %d", meta.RunCount)
	}

	// ShouldDream should be true at threshold
	if !ShouldDream(dir, 5) {
		t.Error("ShouldDream should be true when run_count == dreamEveryNRuns")
	}
	if ShouldDream(dir, 6) {
		t.Error("ShouldDream should be false when run_count < dreamEveryNRuns")
	}

	// Simulate dream reset: save with RunCount=0 and a LastDream timestamp
	resetMeta := RunMeta{RunCount: 0, LastDream: "2026-03-26T12:00:00Z"}
	if err := SaveRunMeta(dir, resetMeta); err != nil {
		t.Fatalf("SaveRunMeta (dream reset): %v", err)
	}

	meta, err = LoadRunMeta(dir)
	if err != nil {
		t.Fatalf("LoadRunMeta after reset: %v", err)
	}
	if meta.RunCount != 0 {
		t.Errorf("expected run count 0 after dream reset, got %d", meta.RunCount)
	}
	if meta.LastDream != "2026-03-26T12:00:00Z" {
		t.Errorf("expected LastDream timestamp, got %q", meta.LastDream)
	}

	// Incrementing resumes from 0
	count, err := IncrementRunCount(dir)
	if err != nil {
		t.Fatalf("IncrementRunCount after reset: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1 after reset+increment, got %d", count)
	}
}

// TestMemoryFilesSurviveMultipleRuns simulates multiple runs appending learnings,
// then verifies all entries are present when read back.
func TestMemoryFilesSurviveMultipleRuns(t *testing.T) {
	projectDir := t.TempDir()

	// Simulate 3 runs, each appending learnings
	runs := []struct {
		id       string
		run      string
		stories  []string
		category string
		content  string
	}{
		{"L-run1-001", "prd-alpha (run 1)", []string{"S-001"}, "testing", "Always run unit tests first."},
		{"L-run2-001", "prd-alpha (run 2)", []string{"S-002", "S-003"}, "architecture", "Keep handlers stateless."},
		{"L-run3-001", "prd-beta (run 3)", []string{"S-010"}, "tooling", "Use go vet before committing."},
	}

	for _, r := range runs {
		entry := LearningEntry{
			ID:        r.id,
			Run:       r.run,
			Stories:   r.stories,
			Confirmed: 1,
			Category:  r.category,
			Content:   r.content,
		}
		if err := AppendLearning(projectDir, entry); err != nil {
			t.Fatalf("AppendLearning(%s): %v", r.id, err)
		}
	}

	// Read back and verify all entries survive
	content, err := ReadLearnings(projectDir)
	if err != nil {
		t.Fatalf("ReadLearnings: %v", err)
	}

	for _, r := range runs {
		if !strings.Contains(content, "### "+r.id) {
			t.Errorf("missing entry header %q", r.id)
		}
		if !strings.Contains(content, r.content) {
			t.Errorf("missing content for %s: %q", r.id, r.content)
		}
		if !strings.Contains(content, "- **Run:** "+r.run) {
			t.Errorf("missing run metadata for %s", r.id)
		}
		if !strings.Contains(content, "- **Category:** "+r.category) {
			t.Errorf("missing category for %s", r.id)
		}
	}

	// Verify entry count — learnings are project-specific, PRD learnings are global
	// Pass empty ralphHome since we only care about project learnings here
	stats := MemoryStats(projectDir, "")
	var learningsInfo MemoryFileInfo
	for _, s := range stats {
		if s.Name == "learnings.md" {
			learningsInfo = s
			break
		}
	}
	if !learningsInfo.Exists {
		t.Fatal("learnings.md should exist")
	}
	if learningsInfo.EntryCount != 3 {
		t.Errorf("expected 3 entries, got %d", learningsInfo.EntryCount)
	}
}

// TestFullMemoryLifecycle is an end-to-end integration test: append entries,
// read them back, verify sizes, and check run counter.
func TestFullMemoryLifecycle(t *testing.T) {
	projectDir := t.TempDir()
	ralphHome := t.TempDir()

	// Step 1: Append project-specific learning and global PRD learning
	learning := LearningEntry{
		ID:        "lifecycle-L1",
		Run:       "lifecycle-run-01",
		Stories:   []string{"LC-001"},
		Confirmed: 1,
		Category:  "testing",
		Content:   "Integration tests catch what unit tests miss.",
	}
	prdLearning := LearningEntry{
		ID:        "lifecycle-PL1",
		Run:       "lifecycle-run-01",
		Stories:   []string{"LC-002"},
		Confirmed: 2,
		Category:  "sizing",
		Content:   "Stories with >5 subtasks should be split.",
	}

	if err := AppendLearning(projectDir, learning); err != nil {
		t.Fatalf("AppendLearning: %v", err)
	}
	if err := AppendPRDLearning(ralphHome, prdLearning); err != nil {
		t.Fatalf("AppendPRDLearning: %v", err)
	}

	// Step 2: Read back and verify round-trip
	learnings, err := ReadLearnings(projectDir)
	if err != nil {
		t.Fatalf("ReadLearnings: %v", err)
	}
	if !strings.Contains(learnings, "### lifecycle-L1") {
		t.Error("learnings should contain lifecycle-L1")
	}
	if !strings.Contains(learnings, "Integration tests catch what unit tests miss.") {
		t.Error("learnings content missing")
	}

	prdContent, err := ReadPRDLearnings(ralphHome)
	if err != nil {
		t.Fatalf("ReadPRDLearnings: %v", err)
	}
	if !strings.Contains(prdContent, "### lifecycle-PL1") {
		t.Error("prd-learnings should contain lifecycle-PL1")
	}

	// Step 3: CheckSize should report non-zero for both memory locations
	result, err := CheckSize(projectDir, ralphHome)
	if err != nil {
		t.Fatalf("CheckSize: %v", err)
	}
	if result.TotalBytes == 0 {
		t.Error("expected non-zero total bytes after writing memory")
	}
	if result.Level() != "ok" {
		t.Errorf("small memory should be 'ok', got %q", result.Level())
	}

	// Step 4: MemoryStats should report correct counts
	stats := MemoryStats(projectDir, ralphHome)
	if len(stats) != 2 {
		t.Fatalf("expected 2 memory file stats, got %d", len(stats))
	}
	for _, s := range stats {
		if !s.Exists {
			t.Errorf("memory file %s should exist", s.Name)
		}
		if s.EntryCount != 1 {
			t.Errorf("memory file %s: expected 1 entry, got %d", s.Name, s.EntryCount)
		}
		if s.SizeBytes == 0 {
			t.Errorf("memory file %s: expected non-zero size", s.Name)
		}
	}

	// Verify learnings.md is in project dir, prd-learnings.md is in ralphHome
	if !strings.Contains(stats[0].Path, ".ralph") {
		t.Errorf("learnings.md path should be under .ralph, got %s", stats[0].Path)
	}
	if strings.Contains(stats[1].Path, ".ralph") {
		t.Errorf("prd-learnings.md path should NOT be under .ralph, got %s", stats[1].Path)
	}

	// Step 5: Run counter lifecycle
	for i := 1; i <= 3; i++ {
		count, err := IncrementRunCount(projectDir)
		if err != nil {
			t.Fatalf("IncrementRunCount: %v", err)
		}
		if count != i {
			t.Errorf("run counter: expected %d, got %d", i, count)
		}
	}
	if !ShouldDream(projectDir, 3) {
		t.Error("ShouldDream should be true at threshold 3")
	}
}

// TestCheckSizeWithAppendedEntries verifies CheckSize reports correct sizes
// after appending entries through the normal API.
func TestCheckSizeWithAppendedEntries(t *testing.T) {
	projectDir := t.TempDir()

	// Create the structure CheckSize expects: projectDir/.ralph/memory/
	memDir := filepath.Join(projectDir, ".ralph", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write known content directly
	content := strings.Repeat("x", 200_000) // 200KB = 50,000 tokens → warn threshold
	if err := os.WriteFile(filepath.Join(memDir, "learnings.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSize(projectDir, "")
	if err != nil {
		t.Fatalf("CheckSize: %v", err)
	}
	if result.Level() != "warn" {
		t.Errorf("expected warn level for 200KB, got %q (tokens=%d)", result.Level(), result.TokenEstimate)
	}

	// Add more to exceed crit threshold
	critContent := strings.Repeat("y", 400_000) // total 600KB = 150,000 tokens → crit
	if err := os.WriteFile(filepath.Join(memDir, "prd-learnings.md"), []byte(critContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err = CheckSize(projectDir, "")
	if err != nil {
		t.Fatalf("CheckSize: %v", err)
	}
	if result.Level() != "crit" {
		t.Errorf("expected crit level for 600KB total, got %q (tokens=%d)", result.Level(), result.TokenEstimate)
	}
	if result.WarnMessage() == "" {
		t.Error("expected non-empty warning message at crit level")
	}
}

// TestAppendLearningFormatIsReadable verifies that entries written by AppendLearning
// produce well-formed markdown that can be parsed back by counting headers.
func TestAppendLearningFormatIsReadable(t *testing.T) {
	projectDir := t.TempDir()

	entries := []LearningEntry{
		{ID: "fmt-001", Run: "run-01", Stories: []string{"S-001"}, Confirmed: 1, Category: "testing", Content: "First entry."},
		{ID: "fmt-002", Run: "run-02", Stories: []string{"S-002", "S-003"}, Confirmed: 3, Category: "arch", Content: "Second entry with multiple stories."},
		{ID: "fmt-003", Run: "run-03", Stories: []string{"S-004"}, Confirmed: 0, Category: "tooling", Content: "Third entry."},
	}

	for _, e := range entries {
		if err := AppendLearning(projectDir, e); err != nil {
			t.Fatalf("AppendLearning(%s): %v", e.ID, err)
		}
	}

	content, err := ReadLearnings(projectDir)
	if err != nil {
		t.Fatalf("ReadLearnings: %v", err)
	}

	// Each entry should have exactly one "### ID" header
	for _, e := range entries {
		count := strings.Count(content, "### "+e.ID)
		if count != 1 {
			t.Errorf("expected exactly 1 header for %s, found %d", e.ID, count)
		}
	}

	// Verify structured fields are parseable
	lines := strings.Split(content, "\n")
	runCount := 0
	storiesCount := 0
	confirmedCount := 0
	categoryCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "- **Run:**") {
			runCount++
		}
		if strings.HasPrefix(line, "- **Stories:**") {
			storiesCount++
		}
		if strings.HasPrefix(line, "- **Confirmed:**") {
			confirmedCount++
		}
		if strings.HasPrefix(line, "- **Category:**") {
			categoryCount++
		}
	}

	if runCount != 3 {
		t.Errorf("expected 3 Run fields, got %d", runCount)
	}
	if storiesCount != 3 {
		t.Errorf("expected 3 Stories fields, got %d", storiesCount)
	}
	if confirmedCount != 3 {
		t.Errorf("expected 3 Confirmed fields, got %d", confirmedCount)
	}
	if categoryCount != 3 {
		t.Errorf("expected 3 Category fields, got %d", categoryCount)
	}

	// Multi-story entry should have comma-separated stories
	if !strings.Contains(content, "S-002, S-003") {
		t.Error("expected comma-separated stories for fmt-002")
	}
}
