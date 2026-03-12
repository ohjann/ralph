package runner

import "testing"

func TestIsUsageLimitError(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   bool
	}{
		{"rate limit", "Error: rate limit exceeded", true},
		{"usage limit", "You have exceeded your usage limit", true},
		{"429", "HTTP 429: too many requests", true},
		{"overloaded", "The API is overloaded", true},
		{"token limit", "token limit reached", true},
		{"normal error", "Error: file not found", false},
		{"empty", "", false},
		{"exit status only", "exit status 1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUsageLimitError(tt.stderr); got != tt.want {
				t.Errorf("IsUsageLimitError(%q) = %v, want %v", tt.stderr, got, tt.want)
			}
		})
	}
}

func TestUsageLimitErrorMessage(t *testing.T) {
	err := &UsageLimitError{Stderr: "rate limit exceeded"}
	want := "claude usage limit: rate limit exceeded"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}
