package viewer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ohjann/ralphplusplus/internal/viewer"
)

func TestBootstrap_ReturnsMetadataWithAuth(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := viewer.NewServer(ctx, "tok-abc", "v-test")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	h := s.Handler()

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/bootstrap", nil)
	req.Header.Set("X-Ralph-Token", "tok-abc")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
	var body struct {
		Version      string   `json:"version"`
		FeatureFlags []string `json:"featureFlags"`
		Token        string   `json:"token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v body=%q", err, rr.Body.String())
	}
	if body.Version != "v-test" {
		t.Fatalf("version=%q want v-test", body.Version)
	}
	if body.Token != "tok-abc" {
		t.Fatalf("token=%q want tok-abc", body.Token)
	}
	if body.FeatureFlags == nil {
		t.Fatalf("featureFlags is nil, want []")
	}
}

func TestBootstrap_RequiresToken(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := viewer.NewServer(ctx, "tok-abc", "v-test")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	h := s.Handler()

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/bootstrap", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rr.Code)
	}
}

func TestBootstrap_RejectsNonLoopback(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := viewer.NewServer(ctx, "tok-abc", "v-test")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	h := s.Handler()

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/bootstrap", nil)
	req.Header.Set("X-Ralph-Token", "tok-abc")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rr.Code)
	}
}
