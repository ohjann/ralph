package testintegrity

import (
	"regexp"
	"strings"
)

// Python detectors.

var (
	// assert True / assert 1 == 1 / assert "x" == "x".
	pyAssertLiteralTrue = regexp.MustCompile(`^\s*assert\s+True\s*(?:,.*)?$`)
	pyAssertLiteralEq   = regexp.MustCompile(`^\s*assert\s+(.+?)\s*==\s*(.+?)\s*(?:,.*)?$`)
	// self.assertTrue(True), self.assertEqual(X, X).
	pyAssertTrueTrue  = regexp.MustCompile(`\bself\.assertTrue\s*\(\s*True\s*(?:,[^)]*)?\)`)
	pyAssertFalseFalse = regexp.MustCompile(`\bself\.assertFalse\s*\(\s*False\s*(?:,[^)]*)?\)`)
	pyAssertEqualSame = regexp.MustCompile(`\bself\.assertEqual\s*\(\s*([^,]+?)\s*,\s*([^,)]+?)\s*(?:,[^)]*)?\)`)
	// Skipped tests: @pytest.mark.skip, @unittest.skip, self.skipTest(..).
	pySkip = regexp.MustCompile(`@(?:pytest\.mark\.skip|unittest\.skip)\b|\bself\.skipTest\s*\(`)
	// Test function opener: def test_whatever(self):
	pyTestFunc = regexp.MustCompile(`^\s*(async\s+)?def\s+(test_\w+)\s*\([^)]*\)\s*(?:->[^:]+)?:\s*$`)
	// Any assertion-like call in a Python test body.
	pyAssertionCall = regexp.MustCompile(`\bassert\s|\bself\.assert\w+\s*\(|\bpytest\.raises\b|\bwith\s+pytest\.raises`)
	// Imports.
	pyFromImport = regexp.MustCompile(`^\s*from\s+([A-Za-z_][\w.]*)\s+import\s`)
	pyImport     = regexp.MustCompile(`^\s*import\s+([A-Za-z_][\w.]*)`)
)

func detectPy(path, content string) []Finding {
	if content == "" {
		return nil
	}
	var findings []Finding
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if pyAssertLiteralTrue.MatchString(line) {
			findings = append(findings, Finding{
				Severity: SeverityCritical,
				File:     path,
				Line:     ln,
				Rule:     "tautological-assertion",
				Message:  "assert True cannot fail",
			})
		}
		if m := pyAssertLiteralEq.FindStringSubmatch(line); m != nil {
			if isTautologicalPair(m[1], m[2]) {
				findings = append(findings, Finding{
					Severity: SeverityCritical,
					File:     path,
					Line:     ln,
					Rule:     "tautological-assertion",
					Message:  "assert with identical literal operands cannot fail",
				})
			}
		}
		if pyAssertTrueTrue.MatchString(line) {
			findings = append(findings, Finding{
				Severity: SeverityCritical,
				File:     path,
				Line:     ln,
				Rule:     "tautological-assertion",
				Message:  "self.assertTrue(True) cannot fail",
			})
		}
		if pyAssertFalseFalse.MatchString(line) {
			findings = append(findings, Finding{
				Severity: SeverityCritical,
				File:     path,
				Line:     ln,
				Rule:     "tautological-assertion",
				Message:  "self.assertFalse(False) cannot fail",
			})
		}
		if m := pyAssertEqualSame.FindStringSubmatch(line); m != nil {
			if isTautologicalPair(m[1], m[2]) {
				findings = append(findings, Finding{
					Severity: SeverityCritical,
					File:     path,
					Line:     ln,
					Rule:     "tautological-assertion",
					Message:  "self.assertEqual with identical literal operands cannot fail",
				})
			}
		}
		if pySkip.MatchString(line) {
			findings = append(findings, Finding{
				Severity: SeverityLow,
				File:     path,
				Line:     ln,
				Rule:     "skipped-test",
				Message:  "skipped test leaves the case unexecuted",
			})
		}
	}

	findings = append(findings, detectPyEmptyBodies(path, lines)...)

	return findings
}

// detectPyEmptyBodies scans def test_*(): bodies. A body is "empty" when the
// only non-blank, non-comment statements are pass, docstrings, or t.Log-like
// prints.
func detectPyEmptyBodies(path string, lines []string) []Finding {
	var findings []Finding
	i := 0
	for i < len(lines) {
		m := pyTestFunc.FindStringSubmatch(lines[i])
		if m == nil {
			i++
			continue
		}
		name := m[2]
		startLn := i + 1
		// Determine indent of function body by scanning for the first
		// non-blank line after the def and reading its indent.
		j := i + 1
		bodyIndent := -1
		for j < len(lines) {
			if strings.TrimSpace(lines[j]) == "" {
				j++
				continue
			}
			bodyIndent = leadingSpaces(lines[j])
			break
		}
		if bodyIndent < 0 {
			break
		}
		// Walk body lines until we hit a line with indent <= header indent
		// and non-blank — that's the end of the function.
		headerIndent := leadingSpaces(lines[i])
		body := []string{}
		k := j
		for k < len(lines) {
			line := lines[k]
			if strings.TrimSpace(line) == "" {
				body = append(body, line)
				k++
				continue
			}
			if leadingSpaces(line) <= headerIndent {
				break
			}
			body = append(body, line)
			k++
		}
		if !hasPyAssertion(body) {
			findings = append(findings, Finding{
				Severity: SeverityHigh,
				File:     path,
				Line:     startLn,
				Rule:     "empty-test-body",
				Message:  "test " + name + " has no assertion",
			})
		}
		i = k
	}
	return findings
}

func hasPyAssertion(body []string) bool {
	for _, l := range body {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, `"""`) || strings.HasPrefix(t, "'''") {
			continue
		}
		if pyAssertionCall.MatchString(l) {
			return true
		}
	}
	return false
}

func leadingSpaces(s string) int {
	n := 0
	for n < len(s) && (s[n] == ' ' || s[n] == '\t') {
		n++
	}
	return n
}
