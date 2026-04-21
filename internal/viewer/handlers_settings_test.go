package viewer_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ohjann/ralphplusplus/internal/viewer"
)

// doPostJSON issues a POST with a JSON body and the test token header.
func doPostJSON(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1"+path, strings.NewReader(body))
	req.Header.Set("X-Ralph-Token", "tok-abc")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// TestSettingsPost_FileFallback exercises the no-daemon path: the viewer
// validates, applies to a fresh Config seeded from disk, and round-trips
// through SaveConfig. The on-disk config.toml should reflect the change.
func TestSettingsPost_FileFallback(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())
	const fp = "feedfacecafe"
	repoRoot := shortTempRoot(t)
	seedRepoWithPath(t, fp, repoRoot)

	_, h := newTestServer(t)
	rr := doPostJSON(t, h, "/api/live/"+fp+"/settings", `{"workers":6,"judge_enabled":false}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
	var resp viewer.SettingsUpdateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Source != "file" {
		t.Errorf("source=%q want file", resp.Source)
	}
	got := map[string]bool{}
	for _, f := range resp.Applied {
		got[f] = true
	}
	if !got["workers"] || !got["judge_enabled"] {
		t.Errorf("applied=%v want workers + judge_enabled", resp.Applied)
	}

	cfgPath := filepath.Join(repoRoot, ".ralph", "config.toml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "workers = 6") {
		t.Errorf("config.toml missing 'workers = 6':\n%s", body)
	}
	if !strings.Contains(body, "judge_enabled = false") {
		t.Errorf("config.toml missing 'judge_enabled = false':\n%s", body)
	}
}

// TestSettingsPost_ValidationFailed rejects bad input before any write
// hits disk or the daemon socket. Response body matches the
// SettingsValidationError shape.
func TestSettingsPost_ValidationFailed(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())
	const fp = "feedfacecafe"
	repoRoot := shortTempRoot(t)
	seedRepoWithPath(t, fp, repoRoot)

	_, h := newTestServer(t)
	rr := doPostJSON(t, h, "/api/live/"+fp+"/settings", `{"workers":0}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%q", rr.Code, rr.Body.String())
	}
	var body viewer.SettingsValidationError
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Error != "validation_failed" {
		t.Errorf("error=%q want validation_failed", body.Error)
	}
	if body.Fields["workers"] != "must be >= 1" {
		t.Errorf("fields.workers=%q want 'must be >= 1'", body.Fields["workers"])
	}

	// Ensure no config.toml was written on the validation failure path.
	if _, err := os.Stat(filepath.Join(repoRoot, ".ralph", "config.toml")); !os.IsNotExist(err) {
		t.Errorf("config.toml created on validation failure: err=%v", err)
	}
}

// TestSettingsPost_DaemonForward verifies that when the daemon socket is
// reachable, the viewer proxies the body verbatim and reports
// source:"daemon".
func TestSettingsPost_DaemonForward(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())
	const fp = "feedfacecafe"
	repoRoot := shortTempRoot(t)
	seedRepoWithPath(t, fp, repoRoot)

	var seenBody []byte
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/settings" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		seenBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","applied":["workers","judge_enabled"]}`))
	})
	_ = startFakeDaemon(t, repoRoot, handler)

	_, h := newTestServer(t)
	rr := doPostJSON(t, h, "/api/live/"+fp+"/settings", `{"workers":6,"judge_enabled":false}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
	var resp viewer.SettingsUpdateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Source != "daemon" {
		t.Errorf("source=%q want daemon", resp.Source)
	}
	got := map[string]bool{}
	for _, f := range resp.Applied {
		got[f] = true
	}
	if !got["workers"] || !got["judge_enabled"] {
		t.Errorf("applied=%v want workers + judge_enabled", resp.Applied)
	}
	if !bytes.Contains(seenBody, []byte(`"workers":6`)) {
		t.Errorf("daemon did not see workers field; body=%q", seenBody)
	}
	// Daemon-forward path must not have written config.toml — the daemon
	// owns that write.
	if _, err := os.Stat(filepath.Join(repoRoot, ".ralph", "config.toml")); !os.IsNotExist(err) {
		t.Errorf("config.toml created on daemon-forward path: err=%v", err)
	}
}

// TestSettingsPost_404UnknownRepo: the file fallback path returns 404
// when the repo fingerprint is unknown.
func TestSettingsPost_404UnknownRepo(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())
	_, h := newTestServer(t)
	rr := doPostJSON(t, h, "/api/live/deadbeef/settings", `{"workers":2}`)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%q", rr.Code, rr.Body.String())
	}
}
