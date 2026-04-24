package judge

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ohjann/ralphplusplus/internal/assets"
	"github.com/ohjann/ralphplusplus/internal/costs"
	"github.com/ohjann/ralphplusplus/internal/debuglog"
	rexec "github.com/ohjann/ralphplusplus/internal/exec"
	"github.com/ohjann/ralphplusplus/internal/prd"
)

// DevilsAdvocateResult captures the outcome of an appellate review.
type DevilsAdvocateResult struct {
	// Verdict is "PASS" (override judge) or "FAIL" (uphold judge). Empty on
	// error; callers should check Err.
	Verdict string
	// Reason is the appellate reviewer's one-sentence justification.
	Reason string
	// CriteriaMet and CriteriaFailed mirror the normal judge output.
	CriteriaMet    []string
	CriteriaFailed []string
	Suggestion     string
	// RejectionCount is the number of prior judge rejections that triggered
	// the appellate review.
	RejectionCount int
	TokenUsage     costs.TokenUsage
	// Warning is set when the appellate call errored but we chose to fall
	// through rather than block indefinitely. When non-empty, callers
	// typically treat it as an override PASS with a flag for human review.
	Warning string
	Err     error
}

// RunDevilsAdvocate evaluates whether the judge's objections are grounded in
// the acceptance criteria. It reads the feedback-history file written by the
// regular judge path on each FAIL and feeds the full objection sequence into
// the appellate prompt.
func RunDevilsAdvocate(ctx context.Context, ralphHome, projectDir, prdFile, storyID string, preRevs []DirRev, rejectionCount int) DevilsAdvocateResult {
	p, err := prd.Load(prdFile)
	if err != nil {
		return DevilsAdvocateResult{Warning: fmt.Sprintf("could not load prd.json: %v", err), Err: err, RejectionCount: rejectionCount}
	}
	story := p.FindStory(storyID)
	if story == nil {
		return DevilsAdvocateResult{Warning: fmt.Sprintf("story %s not found in prd.json", storyID), RejectionCount: rejectionCount}
	}

	var criteria []string
	for _, c := range story.AcceptanceCriteria {
		criteria = append(criteria, "- "+c)
	}
	criteriaStr := strings.Join(criteria, "\n")
	if len(criteria) == 0 {
		criteriaStr = "No acceptance criteria specified"
	}

	diff := getDiffs(ctx, preRevs)
	if diff == "" {
		return DevilsAdvocateResult{
			Verdict:        "FAIL",
			Reason:         "No code changes were produced for this story",
			CriteriaFailed: []string{"Implementation produces code changes"},
			Suggestion:     "The worker did not produce any diff. The story needs to be re-attempted.",
			RejectionCount: rejectionCount,
		}
	}

	history := ReadFeedbackHistory(projectDir, storyID)
	if history == "" {
		// Fall back to the single latest-feedback file.
		history = readSingleFeedback(projectDir, storyID)
	}
	if history == "" {
		history = "(no prior feedback recorded — this is unusual; evaluate on the diff + criteria alone)"
	}

	template, err := assets.ReadPrompt("prompts/devils-advocate.md")
	if err != nil {
		return DevilsAdvocateResult{Warning: fmt.Sprintf("could not read devils-advocate.md: %v", err), Err: err, RejectionCount: rejectionCount}
	}

	prompt := string(template)
	prompt = strings.ReplaceAll(prompt, "{{STORY_ID}}", storyID)
	prompt = strings.ReplaceAll(prompt, "{{STORY_TITLE}}", story.Title)
	prompt = strings.ReplaceAll(prompt, "{{STORY_DESCRIPTION}}", story.Description)
	prompt = strings.ReplaceAll(prompt, "{{ACCEPTANCE_CRITERIA}}", criteriaStr)
	prompt = strings.ReplaceAll(prompt, "{{DIFF}}", diff)
	prompt = strings.ReplaceAll(prompt, "{{FEEDBACK_HISTORY}}", history)
	prompt = strings.ReplaceAll(prompt, "{{REJECTION_COUNT}}", fmt.Sprintf("%d", rejectionCount))

	output, tokenUsage, err := rexec.RunClaudeDevilsAdvocate(ctx, prompt)
	if err != nil || output == "" {
		debuglog.Log("devils-advocate: empty output or error for %s: %v", storyID, err)
		return DevilsAdvocateResult{
			Warning:        "Devil's Advocate returned empty output or error",
			Err:            err,
			RejectionCount: rejectionCount,
			TokenUsage:     tokenUsage,
		}
	}

	verdict, parseErr := parseVerdict(output)
	if parseErr != nil {
		return DevilsAdvocateResult{
			Warning:        parseErr.Error(),
			RejectionCount: rejectionCount,
			TokenUsage:     tokenUsage,
		}
	}

	return DevilsAdvocateResult{
		Verdict:        verdict.Verdict,
		Reason:         verdict.Reason,
		CriteriaMet:    verdict.CriteriaMet,
		CriteriaFailed: verdict.CriteriaFailed,
		Suggestion:     verdict.Suggestion,
		RejectionCount: rejectionCount,
		TokenUsage:     tokenUsage,
	}
}

// readSingleFeedback returns the contents of the most recent judge-feedback
// file, used as a fallback when the history file does not yet exist (e.g.
// pre-existing rejection counter from before the history feature shipped).
func readSingleFeedback(projectDir, storyID string) string {
	path := fmt.Sprintf("%s/.ralph/judge-feedback-%s.md", projectDir, storyID)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// AppendDevilsAdvocateOverride writes a note to progress.md indicating the
// appellate reviewer overrode the judge and passed the story.
func AppendDevilsAdvocateOverride(progressFile, storyID string, r DevilsAdvocateResult) {
	f, err := os.OpenFile(progressFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "\n## [DA] %s PASS after %d rejections — appellate override\n\n**Reason:** %s\n\n---\n",
		storyID, r.RejectionCount, r.Reason)
}

// AppendDevilsAdvocateConcur writes a note to progress.md indicating the
// appellate reviewer upheld the judge. The story is halted for human review.
func AppendDevilsAdvocateConcur(progressFile, storyID string, r DevilsAdvocateResult) {
	f, err := os.OpenFile(progressFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	failed := strings.Join(r.CriteriaFailed, ", ")
	fmt.Fprintf(f, "\n## [DA] %s FAIL after %d rejections — appellate upheld judge [HUMAN REVIEW NEEDED]\n\n**Reason:** %s\n\n**Failed criteria:** %s\n\n**Suggestion:** %s\n\n---\n",
		storyID, r.RejectionCount, r.Reason, failed, r.Suggestion)
}

// AppendDevilsAdvocateWarning writes a fallthrough note when the appellate
// call errored. The behaviour matches the legacy auto-pass so we do not
// regress availability when the appellate side is unhealthy.
func AppendDevilsAdvocateWarning(progressFile, storyID string, r DevilsAdvocateResult) {
	f, err := os.OpenFile(progressFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "\n## [DA] %s auto-passed after %d rejections — appellate error: %s [HUMAN REVIEW NEEDED]\n---\n",
		storyID, r.RejectionCount, r.Warning)
}

// FormatDevilsAdvocateResult renders a short TUI-friendly summary.
func FormatDevilsAdvocateResult(storyID string, r DevilsAdvocateResult) string {
	if r.Warning != "" {
		return fmt.Sprintf("── DA: %s ── ERROR (%s) — falling through [HUMAN REVIEW NEEDED] ──\n", storyID, r.Warning)
	}
	if r.Verdict == "PASS" {
		return fmt.Sprintf("── DA: %s ── OVERRIDE (judge was pedantic) ──\n  Reason: %s\n", storyID, r.Reason)
	}
	if r.Verdict == "FAIL" {
		return fmt.Sprintf("── DA: %s ── UPHELD (judge was correct) [HUMAN REVIEW NEEDED] ──\n  Reason: %s\n", storyID, r.Reason)
	}
	return fmt.Sprintf("── DA: %s ── unknown verdict %q ──\n", storyID, r.Verdict)
}
