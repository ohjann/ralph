package viewer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ohjann/ralphplusplus/internal/viewer/projects"
)

// Server owns the viewer's long-lived dependencies (auth token, version
// string, cached project index). It is built once per process and not
// intended to outlive its context.
type Server struct {
	Token   string
	Version string
	Index   *projects.Index
}

// NewServer builds a Server and starts its fsnotify-backed project index.
// The watcher runs until ctx is cancelled.
func NewServer(ctx context.Context, token, version string) (*Server, error) {
	idx, err := projects.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("projects index: %w", err)
	}
	return &Server{Token: token, Version: version, Index: idx}, nil
}

// Handler returns the composed http.Handler with AuthMiddleware applied to
// every route. The SPA first loads via ?token=... in the URL; XHRs then
// send X-Ralph-Token. /api/bootstrap lets the SPA retrieve the token once
// it is parsed from the URL so subsequent calls have a header to send.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/bootstrap", s.handleBootstrap)
	mux.HandleFunc("/", s.handleRoot)
	return AuthMiddleware(s.Token, mux)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ralph viewer\n")
}

// bootstrapResponse is the shape returned by GET /api/bootstrap. Token is
// echoed so the SPA can store it in memory once it has been extracted from
// the initial URL query; featureFlags is reserved for future toggles.
type bootstrapResponse struct {
	Version      string   `json:"version"`
	FeatureFlags []string `json:"featureFlags"`
	Token        string   `json:"token"`
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(bootstrapResponse{
		Version:      s.Version,
		FeatureFlags: []string{},
		Token:        s.Token,
	})
}
