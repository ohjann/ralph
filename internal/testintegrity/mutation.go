package testintegrity

import (
	"regexp"
	"strings"
)

// assertion-value mutations are same-slot hunk pairs (one removed, one added
// line at the same position within a hunk) where both lines are assertion
// calls and the only thing that changed is the expected value. That pattern —
// repeated — suggests the implementer is "fitting" tests to the code rather
// than writing tests for specified behavior.
//
// This is intentionally a Low-severity signal: a single mutation is normal
// during iterative development; two or more across the same file is worth
// flagging for LLM review.

var (
	// Patterns that match the left-hand expected-value position of common
	// assertion styles. Capture group 1 is the "stem" (everything up to the
	// expected value) so we can compare stems across the removed/added pair.
	mutGoEqualStem    = regexp.MustCompile(`^(\s*(?:assert|require)\.Equal\s*\(\s*t\s*,\s*[^,]+,\s*)`)
	mutGoNotEqStem    = regexp.MustCompile(`^(\s*if\s+.+\s*!=\s*)`)
	mutTSToBeStem     = regexp.MustCompile(`^(\s*expect\s*\([^)]+\)\s*\.toBe\s*\(\s*)`)
	mutTSToEqualStem  = regexp.MustCompile(`^(\s*expect\s*\([^)]+\)\s*\.toEqual\s*\(\s*)`)
	mutPyAssertEqStem = regexp.MustCompile(`^(\s*assert\s+[^=!<>]+==\s*)`)
	mutPyAssertEqFn   = regexp.MustCompile(`^(\s*self\.assertEqual\s*\([^,]+,\s*)`)

	mutationStems = []*regexp.Regexp{
		mutGoEqualStem,
		mutGoNotEqStem,
		mutTSToBeStem,
		mutTSToEqualStem,
		mutPyAssertEqStem,
		mutPyAssertEqFn,
	}
)

// detectMutations counts same-slot assertion-value changes within the diff
// hunks for one file and emits a Low-severity signal when two or more are
// found.
func detectMutations(fd fileDiff) []Finding {
	if !isTestFile(fd.path) {
		return nil
	}
	count := 0
	for _, pair := range fd.hunkPairs {
		if pair.removed == "" || pair.added == "" {
			continue
		}
		if isAssertionMutation(pair.removed, pair.added) {
			count++
		}
	}
	if count < 2 {
		return nil
	}
	return []Finding{{
		Severity: SeverityLow,
		File:     fd.path,
		Line:     0,
		Rule:     "assertion-value-churn",
		Message:  "multiple assertion expected-values changed in this diff; verify tests were not fit to the implementation",
	}}
}

// isAssertionMutation returns true when `removed` and `added` share an
// assertion-call stem and differ only in the expected-value suffix.
func isAssertionMutation(removed, added string) bool {
	for _, re := range mutationStems {
		rm := re.FindStringSubmatch(removed)
		am := re.FindStringSubmatch(added)
		if rm == nil || am == nil {
			continue
		}
		if rm[1] != am[1] {
			continue
		}
		// Stems match. Require that the suffix differs but is non-empty
		// on both sides — rules out pure deletions or whitespace churn.
		rSuf := strings.TrimSpace(removed[len(rm[1]):])
		aSuf := strings.TrimSpace(added[len(am[1]):])
		if rSuf == "" || aSuf == "" || rSuf == aSuf {
			continue
		}
		return true
	}
	return false
}
