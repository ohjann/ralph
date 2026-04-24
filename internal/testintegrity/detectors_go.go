package testintegrity

import (
	"regexp"
	"strings"
)

// Go detectors.
//
// Approach: regex over file contents. Full go/ast parsing would be more precise
// but adds complexity and startup cost; the patterns below match common forms
// of test cheating and leave subtle cases to downstream LLM review.

var (
	// assert.Equal(t, X, X) or require.Equal(t, X, X) with literal same on both sides.
	goAssertEqualSameLit = regexp.MustCompile(`\b(?:assert|require)\.Equal\(\s*t\s*,\s*([^,]+?)\s*,\s*([^,)]+?)\s*(?:,[^)]*)?\)`)
	// assert.True(t, true) or require.True(t, true)
	goAssertTrueLiteral = regexp.MustCompile(`\b(?:assert|require)\.True\(\s*t\s*,\s*true\s*(?:,[^)]*)?\)`)
	// assert.False(t, false)
	goAssertFalseLiteral = regexp.MustCompile(`\b(?:assert|require)\.False\(\s*t\s*,\s*false\s*(?:,[^)]*)?\)`)
	// Skipped tests.
	goSkip = regexp.MustCompile(`\bt\.(?:Skip|SkipNow|Skipf)\b`)
	// Test function header: func TestSomething(t *testing.T) {
	goTestFuncHeader = regexp.MustCompile(`^func\s+(Test[A-Z_]\w*)\s*\(\s*t\s*\*testing\.T\s*\)\s*\{`)
	// Assertion-like calls inside Go tests (used to decide "empty" tests).
	goAssertionCall = regexp.MustCompile(`\b(?:assert|require)\.\w+\(|\bt\.(?:Error|Errorf|Fatal|Fatalf|Fail|FailNow)\b|\bif\s+.*!=.*\{`)
)

func detectGo(path, content string) []Finding {
	if content == "" {
		return nil
	}
	var findings []Finding
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		if m := goAssertEqualSameLit.FindStringSubmatch(line); m != nil {
			if isTautologicalPair(m[1], m[2]) {
				findings = append(findings, Finding{
					Severity: SeverityCritical,
					File:     path,
					Line:     ln,
					Rule:     "tautological-assertion",
					Message:  "assert.Equal with identical literal operands cannot fail",
				})
			}
		}
		if goAssertTrueLiteral.MatchString(line) {
			findings = append(findings, Finding{
				Severity: SeverityCritical,
				File:     path,
				Line:     ln,
				Rule:     "tautological-assertion",
				Message:  "assert.True(t, true) cannot fail",
			})
		}
		if goAssertFalseLiteral.MatchString(line) {
			findings = append(findings, Finding{
				Severity: SeverityCritical,
				File:     path,
				Line:     ln,
				Rule:     "tautological-assertion",
				Message:  "assert.False(t, false) cannot fail",
			})
		}
		if goSkip.MatchString(line) {
			findings = append(findings, Finding{
				Severity: SeverityLow,
				File:     path,
				Line:     ln,
				Rule:     "skipped-test",
				Message:  "t.Skip leaves the test unexecuted",
			})
		}
	}

	// Empty-body test function detector. Scan function headers and inspect
	// their bodies for any assertion-like call.
	findings = append(findings, detectGoEmptyBodies(path, lines)...)

	return findings
}

func detectGoEmptyBodies(path string, lines []string) []Finding {
	var findings []Finding
	i := 0
	for i < len(lines) {
		m := goTestFuncHeader.FindStringSubmatch(lines[i])
		if m == nil {
			i++
			continue
		}
		name := m[1]
		startLn := i + 1
		// Scan forward to the matching closing brace at depth 0.
		depth := 1
		body := make([]string, 0, 16)
		j := i + 1
		for j < len(lines) {
			body = append(body, lines[j])
			depth += strings.Count(lines[j], "{")
			depth -= strings.Count(lines[j], "}")
			if depth <= 0 {
				break
			}
			j++
		}
		if !hasGoAssertion(body) {
			findings = append(findings, Finding{
				Severity: SeverityHigh,
				File:     path,
				Line:     startLn,
				Rule:     "empty-test-body",
				Message:  "test " + name + " has no assertion or failure call",
			})
		}
		i = j + 1
	}
	return findings
}

func hasGoAssertion(body []string) bool {
	for _, l := range body {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "//") {
			continue
		}
		if goAssertionCall.MatchString(l) {
			return true
		}
	}
	return false
}

// isTautologicalPair reports whether two argument strings are the same literal
// (both string literals with identical content, or both numeric literals with
// identical text after trimming).
func isTautologicalPair(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	// Must be literal (not an identifier) — refuse if either looks like an
	// expression with a symbol reference.
	if !isLiteral(a) || !isLiteral(b) {
		return false
	}
	return a == b
}

// isLiteral returns true for quoted strings, numeric literals, and the
// keywords true/false/nil. Anything else we treat as "could be a variable."
func isLiteral(s string) bool {
	if s == "true" || s == "false" || s == "nil" {
		return true
	}
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'' || s[0] == '`') && s[len(s)-1] == s[0] {
		return true
	}
	// Numeric: only digits, leading -, ., or underscore.
	allNum := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9') && c != '-' && c != '.' && c != '_' && c != 'x' && c != 'b' && c != 'o' {
			allNum = false
			break
		}
	}
	return allNum
}
