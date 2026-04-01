package memory

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	learningsFile    = "learnings.md"
	prdLearningsFile = "prd-learnings.md"
)

// projectMemoryDir returns the path to {projectDir}/.ralph/memory/ (project-specific learnings).
func projectMemoryDir(projectDir string) string {
	return filepath.Join(projectDir, ".ralph", "memory")
}

// globalMemoryDir returns the path to {ralphHome}/memory/ (global PRD learnings).
func globalMemoryDir(ralphHome string) string {
	return filepath.Join(ralphHome, "memory")
}

// ReadLearnings returns the contents of {projectDir}/.ralph/memory/learnings.md.
// Returns empty string (not error) if the file doesn't exist yet.
func ReadLearnings(projectDir string) (string, error) {
	return readFile(filepath.Join(projectMemoryDir(projectDir), learningsFile))
}

// ReadPRDLearnings returns the contents of {ralphHome}/memory/prd-learnings.md.
// Returns empty string (not error) if the file doesn't exist yet.
func ReadPRDLearnings(ralphHome string) (string, error) {
	return readFile(filepath.Join(globalMemoryDir(ralphHome), prdLearningsFile))
}

// AppendLearning appends a LearningEntry to {projectDir}/.ralph/memory/learnings.md.
// Creates the directory if it doesn't exist.
func AppendLearning(projectDir string, entry LearningEntry) error {
	return appendEntry(projectMemoryDir(projectDir), learningsFile, entry)
}

// AppendPRDLearning appends a LearningEntry to {ralphHome}/memory/prd-learnings.md.
// Creates the directory if it doesn't exist.
func AppendPRDLearning(ralphHome string, entry LearningEntry) error {
	return appendEntry(globalMemoryDir(ralphHome), prdLearningsFile, entry)
}

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func appendEntry(dir, filename string, entry LearningEntry) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	formatted := formatEntry(entry)

	f, err := os.OpenFile(filepath.Join(dir, filename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(formatted)
	return err
}

// MemoryFileInfo holds summary info about a memory file.
type MemoryFileInfo struct {
	Name       string
	Path       string // full path to the file
	Exists     bool
	SizeBytes  int64
	EntryCount int
}

// MemoryStats returns summary info about memory files.
// learnings.md is read from {projectDir}/.ralph/memory/ (project-specific).
// prd-learnings.md is read from {ralphHome}/memory/ (global).
func MemoryStats(projectDir, ralphHome string) []MemoryFileInfo {
	type fileSpec struct {
		name string
		dir  string
	}
	specs := []fileSpec{
		{learningsFile, projectMemoryDir(projectDir)},
		{prdLearningsFile, globalMemoryDir(ralphHome)},
	}
	var stats []MemoryFileInfo
	for _, s := range specs {
		path := filepath.Join(s.dir, s.name)
		info := MemoryFileInfo{Name: s.name, Path: path}
		if data, err := os.ReadFile(path); err == nil {
			info.Exists = true
			info.SizeBytes = int64(len(data))
			info.EntryCount = bytes.Count(data, []byte("\n### ")) + countLeadingEntry(data)
		}
		stats = append(stats, info)
	}
	return stats
}

// countLeadingEntry returns 1 if data starts with "### ", else 0.
func countLeadingEntry(data []byte) int {
	if len(data) >= 4 && string(data[:4]) == "### " {
		return 1
	}
	return 0
}

func formatEntry(entry LearningEntry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### %s\n", entry.ID)
	fmt.Fprintf(&b, "- **Run:** %s\n", entry.Run)
	fmt.Fprintf(&b, "- **Stories:** %s\n", strings.Join(entry.Stories, ", "))
	fmt.Fprintf(&b, "- **Confirmed:** %d times\n", entry.Confirmed)
	fmt.Fprintf(&b, "- **Category:** %s\n", entry.Category)
	fmt.Fprintf(&b, "\n%s\n\n", entry.Content)
	return b.String()
}
