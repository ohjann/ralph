package testintegrity

import (
	"regexp"
	"strings"
)

// TypeScript / JavaScript detectors.

var (
	// Relative import: import ... from './foo' or "../bar". Absolute imports
	// like 'react' don't point at the code under test.
	tsRelativeImport = regexp.MustCompile(`(?m)^\s*import\s[^;]*\sfrom\s+['"](\.\.?/[^'"]+)['"]`)
	// Dynamic import('...') with relative path.
	tsDynamicRelImport = regexp.MustCompile(`require\s*\(\s*['"](\.\.?/[^'"]+)['"]`)
	// expect(true).toBe(true), expect(1).toBe(1), expect("x").toBe("x").
	tsTautoToBe = regexp.MustCompile(`\bexpect\s*\(\s*([^)]+?)\s*\)\s*\.toBe\s*\(\s*([^)]+?)\s*\)`)
	// expect(X).toEqual(X) same-sided.
	tsTautoToEqual = regexp.MustCompile(`\bexpect\s*\(\s*([^)]+?)\s*\)\s*\.toEqual\s*\(\s*([^)]+?)\s*\)`)
	// Skipped tests.
	tsSkip = regexp.MustCompile(`\b(?:it|test|describe)\.skip\b|\b(?:xit|xtest|xdescribe)\b`)
	// Test-case opener: it('name', () => { OR test('name', () => { OR test('name', function () {
	tsTestOpener = regexp.MustCompile(`\b(?:it|test)\s*\(\s*['"\x60][^'"\x60]*['"\x60]\s*,\s*(?:async\s+)?(?:\([^)]*\)|function\s*\([^)]*\))\s*=>?\s*\{`)
	// Any assertion-like call inside a TS test body.
	tsAssertionCall = regexp.MustCompile(`\bexpect\s*\(|\bassert\.\w+\(|\bchai\.\w+\(|\bshould\.`)
)

func detectTS(path, content string) []Finding {
	if content == "" {
		return nil
	}
	var findings []Finding
	lines := strings.Split(content, "\n")

	// Imports scan: require at least one relative import OR explicit use of
	// a project-root alias. If we see only node_modules imports, flag CRITICAL.
	if !hasTSSourceImport(content) {
		findings = append(findings, Finding{
			Severity: SeverityCritical,
			File:     path,
			Line:     0,
			Rule:     "no-source-import",
			Message:  "test file has no relative import; it cannot be exercising code under test",
		})
	}

	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
			continue
		}

		if m := tsTautoToBe.FindStringSubmatch(line); m != nil {
			if isTautologicalPair(m[1], m[2]) {
				findings = append(findings, Finding{
					Severity: SeverityCritical,
					File:     path,
					Line:     ln,
					Rule:     "tautological-assertion",
					Message:  "expect(X).toBe(X) with identical literal operands cannot fail",
				})
			}
		}
		if m := tsTautoToEqual.FindStringSubmatch(line); m != nil {
			if isTautologicalPair(m[1], m[2]) {
				findings = append(findings, Finding{
					Severity: SeverityCritical,
					File:     path,
					Line:     ln,
					Rule:     "tautological-assertion",
					Message:  "expect(X).toEqual(X) with identical literal operands cannot fail",
				})
			}
		}
		if tsSkip.MatchString(line) {
			findings = append(findings, Finding{
				Severity: SeverityLow,
				File:     path,
				Line:     ln,
				Rule:     "skipped-test",
				Message:  "skipped test leaves the case unexecuted",
			})
		}
	}

	findings = append(findings, detectTSEmptyBodies(path, lines)...)

	return findings
}

// hasTSSourceImport returns true if the file imports something via a relative
// path. Pure node_modules imports do not count.
func hasTSSourceImport(content string) bool {
	if tsRelativeImport.MatchString(content) {
		return true
	}
	if tsDynamicRelImport.MatchString(content) {
		return true
	}
	return false
}

func detectTSEmptyBodies(path string, lines []string) []Finding {
	var findings []Finding
	i := 0
	for i < len(lines) {
		if !tsTestOpener.MatchString(lines[i]) {
			i++
			continue
		}
		startLn := i + 1
		// Find the opening brace on this line and scan to matching close.
		depth := strings.Count(lines[i], "{") - strings.Count(lines[i], "}")
		body := []string{lines[i]}
		j := i + 1
		for j < len(lines) && depth > 0 {
			body = append(body, lines[j])
			depth += strings.Count(lines[j], "{")
			depth -= strings.Count(lines[j], "}")
			j++
		}
		if !hasTSAssertion(body) {
			findings = append(findings, Finding{
				Severity: SeverityHigh,
				File:     path,
				Line:     startLn,
				Rule:     "empty-test-body",
				Message:  "test has no expect/assert call",
			})
		}
		i = j
	}
	return findings
}

func hasTSAssertion(body []string) bool {
	for _, l := range body {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "*") {
			continue
		}
		if tsAssertionCall.MatchString(l) {
			return true
		}
	}
	return false
}
