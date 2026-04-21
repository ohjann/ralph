package viewer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/ohjann/ralphplusplus/internal/config"
	"github.com/ohjann/ralphplusplus/internal/costs"
	"github.com/ohjann/ralphplusplus/internal/history"
)

// handleSettings serves GET /api/live/:fp/settings. When the repo's daemon
// socket is reachable, it forwards GET /api/state and returns the body under
// {source:"daemon", state:<raw>}. When the socket is missing or refuses, it
// falls back to <RepoMeta.Path>/.ralph/config.toml and returns {source:"file",
// config:{...}}. The "source" field is the SPA's signal for showing a
// "Daemon offline" banner. Cache-Control is always no-store.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	fp := r.PathValue("fp")
	w.Header().Set("Cache-Control", "no-store")

	sock, err := s.resolveSock(r.Context(), fp)
	if err == nil {
		// Daemon reachable — proxy /api/state, capture the body, wrap.
		req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, "http://daemon/api/state", nil)
		if reqErr == nil {
			resp, dialErr := unixRoundTrip(sock, req)
			if dialErr == nil {
				defer resp.Body.Close()
				body, readErr := io.ReadAll(resp.Body)
				if readErr == nil && resp.StatusCode == http.StatusOK {
					writeJSON(w, http.StatusOK, SettingsResponse{
						Source: "daemon",
						State:  json.RawMessage(body),
					})
					return
				}
			}
		}
		// Fall through to file source if the daemon round-trip failed for any
		// reason (transient socket issue, daemon mid-restart, etc.) — better
		// to show stale config than a blank page.
	}

	// File fallback: read .ralph/config.toml from the repo path.
	cfg, readErr := s.readRepoConfigToml(r.Context(), fp)
	if readErr != nil {
		if errors.Is(readErr, errRepoNotFound) {
			http.NotFound(w, r)
			return
		}
		// File missing or unparseable → return source:file with empty config,
		// not a 500. The SPA can still render the offline banner.
		cfg = map[string]interface{}{}
	}
	writeJSON(w, http.StatusOK, SettingsResponse{
		Source: "file",
		Config: cfg,
	})
}

var errRepoNotFound = errors.New("repo_not_found")

// handleSettingsPost serves POST /api/live/:fp/settings. The body is a
// SettingsUpdateRequest (= config.TomlConfig). When the daemon socket is
// reachable, the request is forwarded to the daemon's POST /api/settings —
// the daemon mutates its live Config under cfg.mu, persists to disk, and
// broadcasts a daemon_state SSE so other viewers see the change. Source is
// "daemon" in that case.
//
// When the daemon is offline, the viewer validates and writes
// <RepoMeta.Path>/.ralph/config.toml directly via config.SaveConfig — the
// daemon will pick up the new values on its next start. Source is "file".
//
// On a TOML validation error (workers < 1, etc.) the response is 400 with
// {error:"validation_failed", fields:{...}}, regardless of source. On
// successful daemon-forward where the daemon returns 4xx, the daemon's
// response body is propagated verbatim.
func (s *Server) handleSettingsPost(w http.ResponseWriter, r *http.Request) {
	fp := r.PathValue("fp")
	w.Header().Set("Cache-Control", "no-store")

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":  "read_body",
			"detail": err.Error(),
		})
		return
	}

	var tc SettingsUpdateRequest
	if err := json.Unmarshal(body, &tc); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":  "invalid_json",
			"detail": err.Error(),
		})
		return
	}

	// Client-side validation also lives in the SPA, but server-side is
	// authoritative — same rules whether the daemon is up or down.
	if errs := tc.Validate(); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, SettingsValidationError{
			Error:  "validation_failed",
			Fields: errs,
		})
		return
	}

	// Daemon-forward path: round-trip POST /api/settings on the unix socket.
	if sock, sockErr := s.resolveSock(r.Context(), fp); sockErr == nil {
		if status, applied, fwdErr := forwardSettingsToDaemon(r.Context(), sock, body); fwdErr == nil {
			if status == http.StatusOK {
				writeJSON(w, http.StatusOK, SettingsUpdateResponse{
					Source:  "daemon",
					Applied: applied,
				})
				return
			}
			// Daemon returned a non-200 — propagate its status and an opaque
			// error so the SPA knows the daemon rejected the write rather
			// than silently falling back.
			writeJSON(w, status, map[string]any{
				"error":  "daemon_rejected",
				"status": status,
			})
			return
		}
		// Forward failed (transient socket issue). Fall through to file
		// fallback so the user's edit is not lost.
	}

	// File fallback: validate, then load + merge + save through SaveConfig.
	meta, ok := s.lookupRepo(w, r, fp)
	if !ok {
		return
	}
	cfg, loadErr := config.NewForRepo(meta.Path)
	if loadErr != nil {
		http.Error(w, "load config: "+loadErr.Error(), http.StatusInternalServerError)
		return
	}
	applied := cfg.ApplySettings(&tc)
	if err := cfg.SaveConfig(); err != nil {
		http.Error(w, "save config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, SettingsUpdateResponse{
		Source:  "file",
		Applied: applied,
	})
}

// forwardSettingsToDaemon proxies the POST body to the daemon's
// /api/settings endpoint and returns the daemon's status code plus the
// "applied" field from a successful response. A non-nil error means the
// round-trip itself failed (socket gone, transport error) and the caller
// should fall through to the file path.
func forwardSettingsToDaemon(ctx context.Context, sock string, body []byte) (int, []string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://daemon/api/settings", bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := unixRoundTrip(sock, req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil, nil
	}
	var parsed struct {
		Applied []string `json:"applied"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	return resp.StatusCode, parsed.Applied, nil
}

// readRepoConfigToml resolves :fp to RepoMeta.Path and parses
// <Path>/.ralph/config.toml into a generic map. Returns errRepoNotFound when
// the fp does not match a known repo. A missing config.toml is reported by
// returning (nil, os.ErrNotExist) so the caller can decide how to render.
func (s *Server) readRepoConfigToml(ctx context.Context, fp string) (map[string]interface{}, error) {
	repos, err := s.Index.Get(ctx)
	if err != nil {
		return nil, err
	}
	var meta *history.RepoMeta
	for i := range repos {
		if repos[i].FP == fp {
			meta = &repos[i].Meta
			break
		}
	}
	if meta == nil {
		return nil, errRepoNotFound
	}
	path := filepath.Join(meta.Path, ".ralph", "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := toml.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// handleRepoMeta serves GET /api/repos/:fp/meta. Returns the on-disk RepoMeta
// joined with aggregate cost stats from costs.LoadHistory and a per-Kind run
// count from history.LoadManifestsForRepo. Distinct from /api/repos/:fp,
// which returns RepoDetail without the per-Kind breakdown.
func (s *Server) handleRepoMeta(w http.ResponseWriter, r *http.Request) {
	fp := r.PathValue("fp")
	repos, err := s.Index.Get(r.Context())
	if err != nil {
		http.Error(w, "load repos: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var meta *history.RepoMeta
	for i := range repos {
		if repos[i].FP == fp {
			meta = &repos[i].Meta
			break
		}
	}
	if meta == nil {
		http.NotFound(w, r)
		return
	}

	h, err := costs.LoadHistory(fp)
	if err != nil {
		http.Error(w, "load history: "+err.Error(), http.StatusInternalServerError)
		return
	}

	manifests, err := history.LoadManifestsForRepo(fp)
	if err != nil {
		// Don't fail the whole response just because manifests can't be read —
		// counts default to empty.
		manifests = nil
	}
	counts := make(map[string]int)
	for _, m := range manifests {
		k := m.Kind
		if k == "" {
			k = "unknown"
		}
		counts[k]++
	}

	writeJSON(w, http.StatusOK, RepoMetaResponse{
		Meta:            *meta,
		AggCosts:        aggregate(h.Runs),
		RunCountsByKind: counts,
	})
}
