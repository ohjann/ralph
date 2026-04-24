package testintegrity

import (
	"strings"
)

// fileDiff describes the per-file slice of a unified diff.
type fileDiff struct {
	path    string
	deleted bool
	// addedLines are the lines that appear with a leading `+` (excluding the
	// `+++` file header), with the leading marker stripped. Context lines
	// (lines beginning with ` `) are not included here.
	addedLines []string
	// removedLines are the lines that appear with a leading `-` (excluding
	// the `---` file header), with the leading marker stripped. Used by the
	// mutation detector to find same-position assertion-value changes.
	removedLines []string
	// hunkPairs preserves sequential -line/+line pairs within a hunk so the
	// mutation detector can look for same-slot substitutions. A pair with an
	// empty removed slot represents a pure addition, and vice versa.
	hunkPairs []hunkPair
}

type hunkPair struct {
	removed string // may be "" for pure additions
	added   string // may be "" for pure deletions
}

// parseDiffFiles consumes a unified diff and returns one fileDiff per file
// encountered. It tolerates both `diff --git` headers (git/jj) and bare
// `--- a/... / +++ b/...` headers. A file that appears only in a Repo: header
// block (multi-repo concatenation) is handled transparently because the
// per-file markers still appear.
func parseDiffFiles(diff string) []fileDiff {
	if diff == "" {
		return nil
	}
	var files []fileDiff
	var cur *fileDiff
	var pending []hunkPair // unflushed removed lines awaiting +matches

	flushPending := func() {
		if cur == nil {
			return
		}
		for _, p := range pending {
			cur.hunkPairs = append(cur.hunkPairs, p)
		}
		pending = pending[:0]
	}

	closeFile := func() {
		if cur == nil {
			return
		}
		flushPending()
		files = append(files, *cur)
		cur = nil
	}

	lines := strings.Split(diff, "\n")
	inHunk := false
	for _, line := range lines {
		// New file boundary via git-style header.
		if strings.HasPrefix(line, "diff --git ") {
			closeFile()
			cur = &fileDiff{path: extractGitDiffPath(line)}
			inHunk = false
			continue
		}
		// `--- a/path` — indicates either start of new file (if no git
		// header above, e.g. plain unified diff) or deletion marker.
		if strings.HasPrefix(line, "--- ") {
			// If we have no current file yet, open one.
			if cur == nil {
				cur = &fileDiff{}
			}
			if strings.HasPrefix(line, "--- /dev/null") {
				// Stays false; added file
			}
			inHunk = false
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			if cur == nil {
				cur = &fileDiff{}
			}
			if strings.HasPrefix(line, "+++ /dev/null") {
				cur.deleted = true
			}
			if cur.path == "" {
				cur.path = extractPlusPath(line)
			}
			inHunk = false
			continue
		}
		if strings.HasPrefix(line, "@@") {
			flushPending()
			inHunk = true
			continue
		}
		if !inHunk || cur == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "+"):
			added := line[1:]
			// Pair with the next pending removed line, if any.
			if len(pending) > 0 {
				// Find first pending entry without an added match.
				matched := false
				for i := range pending {
					if pending[i].added == "" {
						pending[i].added = added
						matched = true
						break
					}
				}
				if !matched {
					pending = append(pending, hunkPair{added: added})
				}
			} else {
				pending = append(pending, hunkPair{added: added})
			}
			cur.addedLines = append(cur.addedLines, added)
		case strings.HasPrefix(line, "-"):
			removed := line[1:]
			pending = append(pending, hunkPair{removed: removed})
			cur.removedLines = append(cur.removedLines, removed)
		default:
			// Context or blank — break pending pairing so adjacent
			// removed/added across context lines don't pair up.
			flushPending()
		}
	}
	closeFile()
	return files
}

// extractGitDiffPath pulls the b/<path> half of a `diff --git a/x b/y` line.
// Falls back to empty string if parsing fails.
func extractGitDiffPath(line string) string {
	// `diff --git a/foo/bar.go b/foo/bar.go`
	parts := strings.Fields(line)
	for i := range parts {
		if strings.HasPrefix(parts[i], "b/") && i > 0 {
			return strings.TrimPrefix(parts[i], "b/")
		}
	}
	return ""
}

// extractPlusPath pulls the path from a `+++ b/foo/bar.go` header.
func extractPlusPath(line string) string {
	rest := strings.TrimPrefix(line, "+++ ")
	// Strip tab-delimited metadata (timestamps).
	if idx := strings.IndexByte(rest, '\t'); idx >= 0 {
		rest = rest[:idx]
	}
	rest = strings.TrimSpace(rest)
	rest = strings.TrimPrefix(rest, "b/")
	return rest
}
