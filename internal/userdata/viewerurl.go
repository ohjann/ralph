package userdata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ViewerURLPath returns <Dir>/viewer-url. The file is the viewer's hint to
// other Ralph processes (notifier, spawner) about where the UI is currently
// reachable — when present, it contains a single line with the best URL
// (tailnet hostname when --tailscale is on, otherwise loopback with token).
// Mode 0600 because a loopback URL still embeds the token.
func ViewerURLPath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "viewer-url"), nil
}

// WriteViewerURL writes u to the viewer-url hint file at 0600.
func WriteViewerURL(u string) error {
	path, err := ViewerURLPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure dir: %w", err)
	}
	return os.WriteFile(path, []byte(u+"\n"), 0o600)
}

// ReadViewerURL returns the hint file's trimmed contents, or "" if absent.
// Callers treat a missing / empty file as "no URL hint available" and skip
// deep-linking in notifications rather than erroring out.
func ReadViewerURL() string {
	path, err := ViewerURLPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// RemoveViewerURL deletes the hint file. Safe to call when the file is
// missing. Called on viewer shutdown so stale URLs don't linger for the
// notifier to deep-link into.
func RemoveViewerURL() error {
	path, err := ViewerURLPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
