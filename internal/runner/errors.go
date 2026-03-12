package runner

import (
	"strings"
)

// UsageLimitError indicates the Claude CLI failed due to a usage/rate limit.
type UsageLimitError struct {
	Stderr string
}

func (e *UsageLimitError) Error() string {
	return "claude usage limit: " + e.Stderr
}

var usageLimitPatterns = []string{
	"rate limit",
	"usage limit",
	"token limit",
	"too many requests",
	"overloaded",
	"429",
}

// IsUsageLimitError checks whether stderr output indicates a usage limit error.
func IsUsageLimitError(stderr string) bool {
	lower := strings.ToLower(stderr)
	for _, pat := range usageLimitPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}
