package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ohjann/ralphplusplus/internal/runner"
)

// Lens defines a quality review perspective.
type Lens struct {
	Name        string
	Description string
	Prompt      string // injected into the review prompt template
}

// Finding represents a single issue found by a reviewer.
type Finding struct {
	Lens        string `json:"lens"`
	Severity    string `json:"severity"` // "critical", "warning", "info"
	File        string `json:"file"`
	Line        int    `json:"line,omitempty"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

// LensResult is the output of a single lens review.
type LensResult struct {
	Lens        string    `json:"lens"`
	Findings    []Finding `json:"findings"`
	ParseFailed bool      `json:"parse_failed,omitempty"` // true if findings tags could not be parsed
	Err         error     `json:"-"`
}

// Assessment is the merged result of all lens reviews.
type Assessment struct {
	Iteration int          `json:"iteration"`
	Results   []LensResult `json:"results"`
}

// DefaultLenses returns the standard set of review lenses.
func DefaultLenses() []Lens {
	return []Lens{
		{
			Name:        "security",
			Description: "Security review",
			Prompt: `You are a **security reviewer**. Focus exclusively on security concerns:
- Injection vulnerabilities (SQL, command, XSS, path traversal)
- Authentication and authorization issues
- Secrets or credentials in code
- Insecure cryptography or random number generation
- OWASP Top 10 vulnerabilities
- Unsafe deserialization
- Missing input validation at system boundaries

Do NOT comment on code style, efficiency, or architecture. Only security issues.`,
		},
		{
			Name:        "efficiency",
			Description: "Code efficiency review",
			Prompt: `You are a **code efficiency reviewer**. Focus exclusively on performance and efficiency:
- Unnecessary allocations or copies
- N+1 query patterns
- Algorithmic complexity issues (O(n²) where O(n) is possible)
- Missing caching opportunities for expensive operations
- Unnecessary database round-trips
- Memory leaks or resource leaks (unclosed handles, connections)
- Inefficient data structures for the use case

Do NOT comment on security, style, or architecture. Only efficiency issues.`,
		},
		{
			Name:        "dry",
			Description: "DRY-ness and duplication review",
			Prompt: `You are a **DRY (Don't Repeat Yourself) reviewer**. Focus exclusively on code duplication:
- Duplicated logic across the changed files
- Logic that reimplements something that ALREADY EXISTS in the codebase — you MUST explore the existing codebase with Grep and Glob to find existing utilities, helpers, and patterns that the new code could reuse
- Copy-pasted code blocks with minor variations
- Missed opportunities to extract shared helpers (only when 3+ repetitions exist)
- Constants or config values duplicated instead of centralized

IMPORTANT: Don't just review the diff. Actively search the existing codebase for similar patterns using Grep and Glob tools. The goal is to catch cases where the developer reimplemented something that already existed.

Do NOT comment on security, performance, or style. Only duplication issues.`,
		},
		{
			Name:        "error-handling",
			Description: "Error handling and edge cases review",
			Prompt: `You are an **error handling reviewer**. Focus exclusively on error handling and edge cases:
- Unchecked errors (especially in Go: err != nil not checked)
- Missing nil/null checks before dereference
- Panic-prone code (index out of bounds, nil pointer)
- Missing error propagation (swallowed errors)
- Edge cases: empty inputs, zero values, concurrent access, boundary conditions
- Missing cleanup in error paths (defer, finally)
- Race conditions in concurrent code

Do NOT comment on security, style, or architecture. Only error handling and edge cases.`,
		},
		{
			Name:        "testing",
			Description: "Test coverage review",
			Prompt: `You are a **test coverage reviewer**. Focus exclusively on testing gaps:
- New code paths that lack test coverage
- Edge cases not covered by existing tests
- Missing error path tests
- Brittle test assertions (too specific or too vague)
- Integration points that should have integration tests
- Missing boundary value tests

Use Grep and Glob to find existing test files and check if the changed code has corresponding tests.

Do NOT comment on security, style, or architecture. Only testing gaps.`,
		},
	}
}

// RunReview runs a single lens review using Claude Code.
func RunReview(ctx context.Context, projectDir, logDir string, lens Lens, manifest string, iteration int) LensResult {
	prompt := fmt.Sprintf(`%s

## Changed Files

The following files were changed in this work:

%s

## Instructions

1. Use Read, Grep, and Glob tools to examine the changed files and understand the changes
2. For each issue found, output a JSON finding with: lens, severity, file, line (if applicable), description, suggestion
3. After reviewing all changes, output your findings as a JSON array wrapped in <findings> tags:

<findings>
[
  {"lens": "%s", "severity": "critical|warning|info", "file": "path/to/file.go", "line": 42, "description": "what's wrong", "suggestion": "how to fix it"}
]
</findings>

If you find NO issues, output an empty array: <findings>[]</findings>

Be specific. Include file paths and line numbers. Don't flag style preferences — only real issues within your lens.
`, lens.Prompt, manifest, lens.Name)

	logPath := filepath.Join(logDir, fmt.Sprintf("quality-%s-%d.log", lens.Name, iteration))
	result, err := runner.RunClaude(ctx, projectDir, prompt, logPath)
	_ = result
	if err != nil {
		return LensResult{Lens: lens.Name, Err: err}
	}

	// Parse findings from the activity log
	activityPath := strings.TrimSuffix(logPath, ".log") + "-activity.log"
	findings, parsed := parseFindingsFromActivity(activityPath, lens.Name)

	return LensResult{Lens: lens.Name, Findings: findings, ParseFailed: !parsed}
}

// RunReviewsParallel runs multiple lens reviews in parallel, limited to maxWorkers.
func RunReviewsParallel(ctx context.Context, projectDir, logDir string, lenses []Lens, manifest string, iteration int, maxWorkers int) []LensResult {
	results := make([]LensResult, len(lenses))
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, lens := range lenses {
		wg.Add(1)
		go func(idx int, l Lens) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = RunReview(ctx, projectDir, logDir, l, manifest, iteration)
		}(i, lens)
	}

	wg.Wait()
	return results
}

// FilterStaleFindings removes findings that reference files which no longer
// exist on disk. This prevents the fix phase from wasting time on findings
// from files that were deleted or renamed since the review ran.
func FilterStaleFindings(projectDir string, assessment *Assessment) int {
	dropped := 0
	for i := range assessment.Results {
		var kept []Finding
		for _, f := range assessment.Results[i].Findings {
			if f.File == "" {
				kept = append(kept, f)
				continue
			}
			path := filepath.Join(projectDir, f.File)
			if _, err := os.Stat(path); err != nil {
				dropped++
				continue
			}
			kept = append(kept, f)
		}
		assessment.Results[i].Findings = kept
	}
	return dropped
}

// RunFix runs a Claude Code instance to fix identified issues.
func RunFix(ctx context.Context, projectDir, logDir string, assessment Assessment, iteration int) error {
	assessmentJSON, err := json.MarshalIndent(assessment, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling assessment: %w", err)
	}

	prompt := fmt.Sprintf(`You are fixing code quality issues identified by automated reviewers.

## Issues to Address

The following issues were found by quality reviewers. Address them in priority order: critical first, then warning, then info.

%s

## Instructions

1. Read each finding carefully
2. Fix the issues in priority order (critical → warning → info)
3. For each fix, make the minimal change needed — do not refactor surrounding code
4. Run quality checks (typecheck, lint, test) after making changes
5. If a finding is a false positive, skip it
6. Commit your changes with a descriptive message

Focus on real fixes. Don't add unnecessary comments, documentation, or refactoring beyond what's needed to address the findings.
`, string(assessmentJSON))

	logPath := filepath.Join(logDir, fmt.Sprintf("quality-fix-%d.log", iteration))
	result, err := runner.RunClaude(ctx, projectDir, prompt, logPath)
	_ = result
	return err
}

// MergeAssessment combines lens results into an Assessment.
func MergeAssessment(results []LensResult, iteration int) Assessment {
	return Assessment{
		Iteration: iteration,
		Results:   results,
	}
}

// TotalFindings counts all findings across all lenses.
func (a *Assessment) TotalFindings() int {
	total := 0
	for _, r := range a.Results {
		total += len(r.Findings)
	}
	return total
}

// HasParseFailures returns true if any lens failed to parse its findings.
func (a *Assessment) HasParseFailures() bool {
	for _, r := range a.Results {
		if r.ParseFailed {
			return true
		}
	}
	return false
}

// CountBySeverity returns counts by severity level.
func (a *Assessment) CountBySeverity() (critical, warning, info int) {
	for _, r := range a.Results {
		for _, f := range r.Findings {
			switch f.Severity {
			case "critical":
				critical++
			case "warning":
				warning++
			default:
				info++
			}
		}
	}
	return
}

// HasCritical returns true if any critical findings exist.
func (a *Assessment) HasCritical() bool {
	c, _, _ := a.CountBySeverity()
	return c > 0
}

// WriteAssessment writes the assessment to a file.
func WriteAssessment(projectDir string, assessment Assessment) error {
	dir := filepath.Join(projectDir, ".ralph", "quality")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(assessment, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, fmt.Sprintf("assessment-%d.json", assessment.Iteration))
	return os.WriteFile(path, data, 0o644)
}

// FormatSummary returns a human-readable summary of an assessment.
func FormatSummary(a Assessment) string {
	var sb strings.Builder
	critical, warning, info := a.CountBySeverity()

	sb.WriteString(fmt.Sprintf("── Quality Review (iteration %d) ──\n", a.Iteration))
	parseFailures := 0
	for _, r := range a.Results {
		if r.ParseFailed {
			parseFailures++
		}
	}
	if parseFailures > 0 {
		sb.WriteString(fmt.Sprintf("  Findings: %d critical, %d warning, %d info (%d lenses failed to parse)\n", critical, warning, info, parseFailures))
	} else {
		sb.WriteString(fmt.Sprintf("  Findings: %d critical, %d warning, %d info\n", critical, warning, info))
	}

	for _, r := range a.Results {
		if r.Err != nil {
			sb.WriteString(fmt.Sprintf("  %s: ERROR — %v\n", r.Lens, r.Err))
			continue
		}
		if r.ParseFailed {
			sb.WriteString(fmt.Sprintf("  %s: - (parse failed)\n", r.Lens))
			continue
		}
		if len(r.Findings) == 0 {
			sb.WriteString(fmt.Sprintf("  %s: clean\n", r.Lens))
		} else {
			sb.WriteString(fmt.Sprintf("  %s: %d issues\n", r.Lens, len(r.Findings)))
			for _, f := range r.Findings {
				loc := f.File
				if f.Line > 0 {
					loc = fmt.Sprintf("%s:%d", f.File, f.Line)
				}
				sb.WriteString(fmt.Sprintf("    [%s] %s — %s\n", f.Severity, loc, f.Description))
			}
		}
	}

	return sb.String()
}

// GetDiffManifest returns a summary of changed files for the current run.
// It uses story IDs from the PRD to find the earliest story commit and diffs
// from there to the current revision.
func GetDiffManifest(ctx context.Context, projectDir, prdFile string) (string, error) {
	// Derive the story prefix from the PRD (e.g. "P7-" from "P7-001").
	prefix := storyPrefixFromPRD(prdFile)
	if prefix != "" {
		glob := fmt.Sprintf("story %s*", prefix)
		from := fmt.Sprintf("roots(description(glob:\"%s\"))", glob)
		cmd := exec.CommandContext(ctx, "jj", "diff", "--stat", "--from", from, "--to", "@")
		cmd.Dir = projectDir
		out, err := cmd.CombinedOutput()
		if err == nil && len(out) > 0 {
			return strings.TrimSpace(string(out)), nil
		}
	}

	// Fallback: diff working copy (original behaviour)
	cmd := exec.CommandContext(ctx, "jj", "diff", "--stat", "-r", "@")
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err == nil && len(out) > 0 {
		return strings.TrimSpace(string(out)), nil
	}

	cmd = exec.CommandContext(ctx, "git", "diff", "--stat", "HEAD~1")
	cmd.Dir = projectDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not get diff manifest: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// storyPrefixFromPRD extracts the common prefix from story IDs (e.g. "P7-" from "P7-001").
func storyPrefixFromPRD(prdFile string) string {
	data, err := os.ReadFile(prdFile)
	if err != nil {
		return ""
	}
	var raw struct {
		UserStories []struct {
			ID string `json:"id"`
		} `json:"userStories"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || len(raw.UserStories) == 0 {
		return ""
	}
	// Take the first story ID and strip trailing digits to get the prefix.
	// e.g. "P7-001" → "P7-"
	id := raw.UserStories[0].ID
	i := len(id)
	for i > 0 && id[i-1] >= '0' && id[i-1] <= '9' {
		i--
	}
	if i == 0 {
		return ""
	}
	prefix := id[:i]
	// Validate prefix is alphanumeric + dash only to prevent revset injection
	for _, c := range prefix {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
			return ""
		}
	}
	return prefix
}

// parseFindingsFromActivity reads the activity log and extracts findings from <findings> tags.
// Returns the findings and whether parsing succeeded (tags were found and valid JSON extracted).
// A successful parse with no issues returns (nil, true). A failed parse returns (nil, false).
func parseFindingsFromActivity(activityPath, lensName string) ([]Finding, bool) {
	data, err := os.ReadFile(activityPath)
	if err != nil {
		return nil, false
	}
	content := string(data)

	// Extract content between <findings> tags
	start := strings.LastIndex(content, "<findings>")
	if start < 0 {
		return nil, false
	}
	start += len("<findings>")
	end := strings.LastIndex(content, "</findings>")
	if end < 0 || end <= start {
		return nil, false
	}

	jsonStr := strings.TrimSpace(content[start:end])
	if jsonStr == "" || jsonStr == "[]" {
		return nil, true // parsed successfully, no findings
	}

	var findings []Finding
	if err := json.Unmarshal([]byte(jsonStr), &findings); err != nil {
		return nil, false
	}

	// Ensure lens name is set
	for i := range findings {
		findings[i].Lens = lensName
	}

	return findings, true
}
