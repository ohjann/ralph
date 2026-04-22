package notify

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/ohjann/ralphplusplus/internal/debuglog"
	"github.com/ohjann/ralphplusplus/internal/userdata"
)

// Notifier sends push notifications via ntfy.sh and/or local terminal notifications.
type Notifier struct {
	topic     string
	serverURL string
	terminal  bool   // send macOS/terminal notifications
	disabled  bool   // when true, all notifications are suppressed
	repoFP    string // when set, ntfy notifications include a Click header to the viewer
}

// NewNotifier creates a Notifier. If serverURL is empty, defaults to "https://ntfy.sh".
func NewNotifier(topic string, serverURL string) *Notifier {
	if serverURL == "" {
		serverURL = "https://ntfy.sh"
	}
	return &Notifier{
		topic:     topic,
		serverURL: strings.TrimRight(serverURL, "/"),
		terminal:  true, // always enabled
	}
}

// SetDisabled enables or disables all notifications at runtime.
func (n *Notifier) SetDisabled(disabled bool) {
	if n != nil {
		n.disabled = disabled
	}
}

// IsDisabled returns whether notifications are currently suppressed.
func (n *Notifier) IsDisabled() bool {
	return n != nil && n.disabled
}

// SetTopic updates the ntfy topic at runtime.
func (n *Notifier) SetTopic(topic string) {
	if n != nil {
		n.topic = topic
	}
}

// SetRepoFP records the daemon's repo fingerprint so each ntfy push can carry
// a Click header that opens the viewer at /repos/<fp>. Without this, push
// notifications still fire but tapping them does nothing on mobile. The base
// URL itself is read from <userdata>/viewer-url at send time, so a viewer
// started after the daemon will Just Work on the next notification.
func (n *Notifier) SetRepoFP(fp string) {
	if n != nil {
		n.repoFP = fp
	}
}

// clickURL returns the deep-link to embed in the next ntfy push, or "" when
// no link should be attached (no fp configured, no viewer hint file present,
// hint malformed). Falling back to "" keeps notifications best-effort —
// missing the deep-link is strictly better than failing the send.
func (n *Notifier) clickURL() string {
	if n == nil || n.repoFP == "" {
		return ""
	}
	base := userdata.ReadViewerURL()
	if base == "" {
		return ""
	}
	return strings.TrimRight(base, "/") + "/repos/" + n.repoFP
}

// Topic returns the current ntfy topic.
func (n *Notifier) Topic() string {
	if n == nil {
		return ""
	}
	return n.topic
}

// Notify sends a push notification. Priority levels: 1=min, 3=default, 5=urgent.
// The send is non-blocking (fire-and-forget goroutine) and logs errors rather than failing.
func (n *Notifier) Notify(ctx context.Context, title string, message string, priority int) error {
	if n == nil || n.disabled {
		return nil
	}

	// Terminal/OS notification (always fires)
	if n.terminal {
		go terminalNotify(title, message)
	}

	// ntfy.sh push notification (only if topic configured)
	if n.topic != "" {
		go func() {
			url := fmt.Sprintf("%s/%s", n.serverURL, n.topic)
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(message))
			if err != nil {
				debuglog.Log("notify: failed to create request: %v", err)
				return
			}
			req.Header.Set("Title", title)
			req.Header.Set("Priority", strconv.Itoa(priority))
			req.Header.Set("Tags", "robot")
			if click := n.clickURL(); click != "" {
				req.Header.Set("Click", click)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				debuglog.Log("notify: failed to send notification: %v", err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode >= 400 {
				debuglog.Log("notify: server returned %d for %q", resp.StatusCode, title)
			}
		}()
	}

	return nil
}

// terminalNotify sends a local OS notification and terminal bell.
func terminalNotify(title, message string) {
	// Terminal bell (works in most terminals)
	fmt.Print("\a")

	if runtime.GOOS == "darwin" {
		// macOS: use osascript for native notification center
		// Sanitize to printable ASCII to prevent AppleScript injection
		script := fmt.Sprintf(`display notification %q with title "Ralph" subtitle %q`,
			sanitizeForNotification(message), sanitizeForNotification(title))
		_ = exec.Command("osascript", "-e", script).Run()
	}
}

var nonPrintableASCII = regexp.MustCompile(`[^\x20-\x7E]`)

// sanitizeForNotification strips non-printable-ASCII characters from notification text.
func sanitizeForNotification(s string) string {
	return nonPrintableASCII.ReplaceAllString(s, "")
}

// Helper methods for common notification events.

// StoryComplete sends a notification for a completed story.
func (n *Notifier) StoryComplete(ctx context.Context, storyID, title string) {
	n.Notify(ctx, "Story Complete", fmt.Sprintf("%s: %s", storyID, title), 3)
}

// StoryFailed sends a notification for a failed story.
func (n *Notifier) StoryFailed(ctx context.Context, storyID string, err string) {
	n.Notify(ctx, "Story Failed", fmt.Sprintf("%s: %s", storyID, err), 4)
}

// StoryStuck sends a notification for a stuck story.
func (n *Notifier) StoryStuck(ctx context.Context, storyID, reason string) {
	n.Notify(ctx, "Story Stuck", fmt.Sprintf("%s: %s", storyID, reason), 4)
}

// RunComplete sends a notification when the entire run finishes.
func (n *Notifier) RunComplete(ctx context.Context, completed, total int, cost float64) {
	n.Notify(ctx, "Run Complete", fmt.Sprintf("%d/%d stories, $%.2f", completed, total, cost), 5)
}

// Error sends a notification for an unexpected crash/error.
func (n *Notifier) Error(ctx context.Context, err string) {
	n.Notify(ctx, "Ralph Error", err, 5)
}
