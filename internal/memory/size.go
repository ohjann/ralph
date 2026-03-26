package memory

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// WarnTokenThreshold is the estimated token count at which a warning is emitted.
	WarnTokenThreshold = 50_000
	// CritTokenThreshold is the estimated token count at which a critical error is emitted.
	CritTokenThreshold = 150_000
)

// SizeResult holds the result of checking memory file sizes.
type SizeResult struct {
	TotalBytes  int64
	TokenEstimate int
}

// Level returns the severity level: "ok", "warn", or "crit".
func (r SizeResult) Level() string {
	switch {
	case r.TokenEstimate >= CritTokenThreshold:
		return "crit"
	case r.TokenEstimate >= WarnTokenThreshold:
		return "warn"
	default:
		return "ok"
	}
}

// WarnMessage returns the appropriate warning message, or empty string if under threshold.
func (r SizeResult) WarnMessage() string {
	switch r.Level() {
	case "crit":
		return fmt.Sprintf("Memory files exceed %d tokens. This may degrade worker quality. Run ralph memory consolidate or ralph memory reset", r.TokenEstimate)
	case "warn":
		return fmt.Sprintf("Memory files are large (%d tokens). Run ralph memory consolidate", r.TokenEstimate)
	default:
		return ""
	}
}

// CheckSize reads all files in the .ralph/memory/ directory and returns
// their combined size as an estimated token count (bytes / 4).
func CheckSize(projectDir string) (SizeResult, error) {
	memDir := filepath.Join(projectDir, ".ralph", "memory")
	entries, err := os.ReadDir(memDir)
	if err != nil {
		if os.IsNotExist(err) {
			return SizeResult{}, nil
		}
		return SizeResult{}, err
	}

	var totalBytes int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		totalBytes += info.Size()
	}

	return SizeResult{
		TotalBytes:    totalBytes,
		TokenEstimate: int(totalBytes / 4),
	}, nil
}
