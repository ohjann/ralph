package history

import "testing"

func TestIsValidDisplayName(t *testing.T) {
	ok := []string{
		"add-tailscale-integration",
		"fix-login-race",
		"wire-auth",
		"x1",
		"run-42",
	}
	for _, s := range ok {
		if !IsValidDisplayName(s) {
			t.Errorf("IsValidDisplayName(%q) = false, want true", s)
		}
	}
	bad := []string{
		"",
		"a",                  // too short
		"-leading",
		"trailing-",
		"double--dash",
		"Upper-Case",
		"has space",
		"has_underscore",
		"has.dot",
		"", // duplicated above but explicit
		"way-too-long-slug-that-keeps-going-past-forty-chars-for-sure",
	}
	for _, s := range bad {
		if IsValidDisplayName(s) {
			t.Errorf("IsValidDisplayName(%q) = true, want false", s)
		}
	}
}

func TestNormaliseDisplayName(t *testing.T) {
	if got := normaliseDisplayName("  Add-Feature \n"); got != "add-feature" {
		t.Errorf("got %q", got)
	}
}
