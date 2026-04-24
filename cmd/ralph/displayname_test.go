package main

import "testing"

func TestExtractSlug(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"add-tailscale-integration\n", "add-tailscale-integration"},
		{"  Fix-Login-Race  ", "fix-login-race"},
		{"`migrate-postgres`.", "migrate-postgres"},
		{"'wire-up-auth'", "wire-up-auth"},
		{"add-feature\nexplanation text", "add-feature"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := extractSlug(tc.in); got != tc.want {
			t.Errorf("extractSlug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTrimFirstLine(t *testing.T) {
	if got := trimFirstLine("first line\nsecond", 100); got != "first line" {
		t.Errorf("got %q", got)
	}
	if got := trimFirstLine("abcdefghij", 4); got != "abcd" {
		t.Errorf("got %q", got)
	}
	if got := trimFirstLine("   ", 10); got != "" {
		t.Errorf("got %q", got)
	}
}
