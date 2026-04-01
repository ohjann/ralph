package quality

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeAssessment(t *testing.T) {
	results := []LensResult{
		{Lens: "security", Findings: []Finding{{Severity: "critical"}}},
		{Lens: "efficiency"},
	}
	a := MergeAssessment(results, 1)
	if a.Iteration != 1 {
		t.Errorf("expected iteration 1, got %d", a.Iteration)
	}
	if len(a.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(a.Results))
	}
}

func TestTotalFindings(t *testing.T) {
	a := Assessment{Results: []LensResult{
		{Findings: []Finding{{}, {}}},
		{Findings: []Finding{{}}},
		{Findings: nil},
	}}
	if got := a.TotalFindings(); got != 3 {
		t.Errorf("TotalFindings() = %d, want 3", got)
	}
}

func TestHasParseFailures(t *testing.T) {
	a := Assessment{Results: []LensResult{
		{ParseFailed: false},
		{ParseFailed: true},
	}}
	if !a.HasParseFailures() {
		t.Error("expected HasParseFailures() = true")
	}

	a2 := Assessment{Results: []LensResult{{ParseFailed: false}}}
	if a2.HasParseFailures() {
		t.Error("expected HasParseFailures() = false")
	}
}

func TestCountBySeverity(t *testing.T) {
	a := Assessment{Results: []LensResult{
		{Findings: []Finding{
			{Severity: "critical"},
			{Severity: "warning"},
			{Severity: "info"},
			{Severity: "unknown"}, // should map to info
			{Severity: "critical"},
		}},
	}}
	c, w, i := a.CountBySeverity()
	if c != 2 || w != 1 || i != 2 {
		t.Errorf("CountBySeverity() = (%d, %d, %d), want (2, 1, 2)", c, w, i)
	}
}

func TestHasCritical(t *testing.T) {
	a := Assessment{Results: []LensResult{
		{Findings: []Finding{{Severity: "critical"}}},
	}}
	if !a.HasCritical() {
		t.Error("expected HasCritical() = true")
	}

	a2 := Assessment{Results: []LensResult{
		{Findings: []Finding{{Severity: "warning"}}},
	}}
	if a2.HasCritical() {
		t.Error("expected HasCritical() = false")
	}
}

func TestWriteAssessment(t *testing.T) {
	dir := t.TempDir()
	a := Assessment{Iteration: 1, Results: []LensResult{
		{Lens: "test", Findings: []Finding{{Severity: "info", Description: "test finding"}}},
	}}
	if err := WriteAssessment(dir, a); err != nil {
		t.Fatalf("WriteAssessment: %v", err)
	}

	path := filepath.Join(dir, ".ralph", "quality", "assessment-1.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected assessment file to exist: %v", err)
	}
}

func TestFormatSummary(t *testing.T) {
	a := Assessment{
		Iteration: 1,
		Results: []LensResult{
			{Lens: "security", Findings: []Finding{
				{Severity: "critical", File: "test.go", Line: 10, Description: "bad thing"},
			}},
			{Lens: "efficiency", Findings: nil},
			{Lens: "dry", ParseFailed: true},
			{Lens: "broken", Err: os.ErrNotExist},
		},
	}
	summary := FormatSummary(a)
	if summary == "" {
		t.Error("FormatSummary returned empty string")
	}
}

func TestParseFindingsFromActivity(t *testing.T) {
	t.Run("valid findings", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "activity.log")
		content := `some log output
<findings>
[{"lens":"test","severity":"warning","file":"a.go","line":1,"description":"issue","suggestion":"fix it"}]
</findings>
more output`
		os.WriteFile(path, []byte(content), 0o644)
		findings, ok := parseFindingsFromActivity(path, "test")
		if !ok {
			t.Error("expected parse success")
		}
		if len(findings) != 1 {
			t.Errorf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("empty findings", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "activity.log")
		content := `<findings>[]</findings>`
		os.WriteFile(path, []byte(content), 0o644)
		findings, ok := parseFindingsFromActivity(path, "test")
		if !ok {
			t.Error("expected parse success")
		}
		if findings != nil {
			t.Errorf("expected nil findings, got %v", findings)
		}
	})

	t.Run("missing tags", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "activity.log")
		os.WriteFile(path, []byte("no tags here"), 0o644)
		_, ok := parseFindingsFromActivity(path, "test")
		if ok {
			t.Error("expected parse failure for missing tags")
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "activity.log")
		content := `<findings>not json</findings>`
		os.WriteFile(path, []byte(content), 0o644)
		_, ok := parseFindingsFromActivity(path, "test")
		if ok {
			t.Error("expected parse failure for malformed JSON")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, ok := parseFindingsFromActivity("/nonexistent", "test")
		if ok {
			t.Error("expected parse failure for nonexistent file")
		}
	})
}

func TestFilterStaleFindings(t *testing.T) {
	dir := t.TempDir()

	// Create one file that exists
	existingFile := filepath.Join(dir, "exists.go")
	os.WriteFile(existingFile, []byte("package main"), 0o644)

	a := Assessment{
		Results: []LensResult{
			{
				Lens: "security",
				Findings: []Finding{
					{File: "exists.go", Severity: "warning", Description: "real issue"},
					{File: "deleted.go", Severity: "critical", Description: "stale issue"},
					{File: "", Severity: "info", Description: "no file ref"},
				},
			},
			{
				Lens: "testing",
				Findings: []Finding{
					{File: "also-deleted.go", Severity: "warning", Description: "another stale"},
				},
			},
		},
	}

	dropped := FilterStaleFindings(dir, &a)
	if dropped != 2 {
		t.Errorf("FilterStaleFindings dropped %d, want 2", dropped)
	}
	if len(a.Results[0].Findings) != 2 {
		t.Errorf("security lens has %d findings, want 2", len(a.Results[0].Findings))
	}
	if a.Results[0].Findings[0].File != "exists.go" {
		t.Errorf("first finding file = %q, want exists.go", a.Results[0].Findings[0].File)
	}
	if a.Results[0].Findings[1].File != "" {
		t.Errorf("second finding file = %q, want empty", a.Results[0].Findings[1].File)
	}
	if len(a.Results[1].Findings) != 0 {
		t.Errorf("testing lens has %d findings, want 0", len(a.Results[1].Findings))
	}
}

func TestStoryPrefixFromPRD(t *testing.T) {
	t.Run("valid PRD", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "prd.json")
		os.WriteFile(path, []byte(`{"userStories":[{"id":"P7-001"},{"id":"P7-002"}]}`), 0o644)
		got := storyPrefixFromPRD(path)
		if got != "P7-" {
			t.Errorf("storyPrefixFromPRD = %q, want %q", got, "P7-")
		}
	})

	t.Run("all-digit ID", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "prd.json")
		os.WriteFile(path, []byte(`{"userStories":[{"id":"12345"}]}`), 0o644)
		got := storyPrefixFromPRD(path)
		if got != "" {
			t.Errorf("storyPrefixFromPRD = %q, want empty", got)
		}
	})

	t.Run("empty stories", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "prd.json")
		os.WriteFile(path, []byte(`{"userStories":[]}`), 0o644)
		got := storyPrefixFromPRD(path)
		if got != "" {
			t.Errorf("storyPrefixFromPRD = %q, want empty", got)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		got := storyPrefixFromPRD("/nonexistent/prd.json")
		if got != "" {
			t.Errorf("storyPrefixFromPRD = %q, want empty", got)
		}
	})

	t.Run("invalid prefix chars rejected", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "prd.json")
		os.WriteFile(path, []byte(`{"userStories":[{"id":"P7;DROP-001"}]}`), 0o644)
		got := storyPrefixFromPRD(path)
		if got != "" {
			t.Errorf("storyPrefixFromPRD = %q, want empty for invalid chars", got)
		}
	})
}
