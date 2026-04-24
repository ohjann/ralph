// Package testintegrity runs mechanical checks against test files in a diff to
// detect common forms of test cheating: tautological assertions, empty test
// bodies, test files that never import the code under test, and assertion-value
// churn that looks like tests being fit to output.
//
// It is deliberately conservative: Critical and High findings are intended to
// be blockers for an automated judge gate, while Medium and Low findings are
// surfaced as signals for downstream (LLM) review and do not themselves fail
// the gate.
package testintegrity

import (
	"os"
	"path/filepath"
	"strings"
)

// Severity classifies a finding. Critical and High are gate blockers; Medium
// and Low are advisory signals.
type Severity int

const (
	SeverityLow Severity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityCritical:
		return "CRITICAL"
	case SeverityHigh:
		return "HIGH"
	case SeverityMedium:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// Finding describes a single suspicious pattern in a test file.
type Finding struct {
	Severity Severity
	File     string
	Line     int // 1-based line number; 0 if not applicable
	Rule     string
	Message  string
}

// Report aggregates findings from a single Check run.
type Report struct {
	Findings []Finding
}

// HasBlocker returns true if any finding is Critical or High severity.
func (r Report) HasBlocker() bool {
	for _, f := range r.Findings {
		if f.Severity >= SeverityHigh {
			return true
		}
	}
	return false
}

// Blockers returns Critical and High findings only.
func (r Report) Blockers() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Severity >= SeverityHigh {
			out = append(out, f)
		}
	}
	return out
}

// Signals returns Medium and Low findings only.
func (r Report) Signals() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Severity < SeverityHigh {
			out = append(out, f)
		}
	}
	return out
}

// Check parses the given unified diff, identifies touched test files, and runs
// language-specific heuristics over each one. projectDir is the repository root
// used to read current file contents; files that cannot be read (deleted,
// outside the repo) are skipped.
//
// An empty diff returns an empty Report.
func Check(diff, projectDir string) Report {
	return check(diff, projectDir, osFileReader{})
}

// fileReader abstracts file reads so tests can stub them.
type fileReader interface {
	ReadFile(path string) ([]byte, error)
}

type osFileReader struct{}

func (osFileReader) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func check(diff, projectDir string, reader fileReader) Report {
	var report Report
	files := parseDiffFiles(diff)
	seen := make(map[string]struct{})

	for _, fd := range files {
		if fd.deleted {
			continue
		}
		if _, dup := seen[fd.path]; dup {
			continue
		}
		seen[fd.path] = struct{}{}
		if !isTestFile(fd.path) {
			// Still check for assertion mutations even on non-test
			// files? No — non-test files don't contain assertions. Skip.
			continue
		}

		// Read current on-disk content for whole-file heuristics.
		var content string
		if projectDir != "" {
			full := filepath.Join(projectDir, fd.path)
			if data, err := reader.ReadFile(full); err == nil {
				content = string(data)
			}
		}

		switch langOf(fd.path) {
		case langGo:
			report.Findings = append(report.Findings, detectGo(fd.path, content)...)
		case langTS:
			report.Findings = append(report.Findings, detectTS(fd.path, content)...)
		case langPy:
			report.Findings = append(report.Findings, detectPy(fd.path, content)...)
		}

		// Mutation detector runs on diff hunks regardless of language.
		report.Findings = append(report.Findings, detectMutations(fd)...)
	}

	return report
}

// language classifies a file for detector dispatch.
type language int

const (
	langUnknown language = iota
	langGo
	langTS
	langPy
)

func langOf(path string) language {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return langGo
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return langTS
	case ".py":
		return langPy
	}
	return langUnknown
}

// isTestFile returns true if the path looks like a test file by convention.
func isTestFile(path string) bool {
	base := filepath.Base(path)
	lower := strings.ToLower(base)
	dir := strings.ToLower(filepath.ToSlash(filepath.Dir(path)))

	switch langOf(path) {
	case langGo:
		return strings.HasSuffix(base, "_test.go")
	case langTS:
		if strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") {
			return true
		}
		if strings.Contains(dir, "/__tests__/") || strings.HasSuffix(dir, "/__tests__") || strings.HasPrefix(dir, "__tests__") {
			return true
		}
		return false
	case langPy:
		if strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") {
			return true
		}
		for _, seg := range strings.Split(dir, "/") {
			if seg == "tests" || seg == "test" {
				return true
			}
		}
		return false
	}
	return false
}

// FormatReport returns a concise human-readable summary suitable for feedback
// files and progress logs. Blockers first, then signals.
func FormatReport(r Report) string {
	if len(r.Findings) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Test integrity findings:\n")
	blockers := r.Blockers()
	signals := r.Signals()
	if len(blockers) > 0 {
		sb.WriteString("\nBlockers:\n")
		for _, f := range blockers {
			writeFinding(&sb, f)
		}
	}
	if len(signals) > 0 {
		sb.WriteString("\nSignals:\n")
		for _, f := range signals {
			writeFinding(&sb, f)
		}
	}
	return sb.String()
}

func writeFinding(sb *strings.Builder, f Finding) {
	loc := f.File
	if f.Line > 0 {
		loc = f.File + ":" + itoa(f.Line)
	}
	sb.WriteString("  [")
	sb.WriteString(f.Severity.String())
	sb.WriteString("] ")
	sb.WriteString(loc)
	sb.WriteString(" — ")
	sb.WriteString(f.Rule)
	sb.WriteString(": ")
	sb.WriteString(f.Message)
	sb.WriteString("\n")
}

// itoa is a small helper to avoid pulling in strconv where not otherwise needed.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
