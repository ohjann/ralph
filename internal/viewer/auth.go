package viewer

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ohjann/ralphplusplus/internal/userdata"
)

// TokenPath returns <userdata>/viewer.token.
func TokenPath() (string, error) {
	d, err := userdata.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "viewer.token"), nil
}

// LoadOrCreateToken reads the persisted viewer token, generating a new
// 256-bit random hex token on first run. The file is stored with mode 0600
// so that other local accounts cannot read it; it is the persistent source
// of truth, unlike the URL-embedded copy.
func LoadOrCreateToken() (string, error) {
	path, err := TokenPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("ensure viewer dir: %w", err)
	}
	if data, err := os.ReadFile(path); err == nil {
		tok := strings.TrimSpace(string(data))
		if tok != "" {
			return tok, nil
		}
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	tok := hex.EncodeToString(b[:])
	if err := os.WriteFile(path, []byte(tok), 0o600); err != nil {
		return "", fmt.Errorf("write token: %w", err)
	}
	return tok, nil
}

// AuthMiddleware enforces the viewer's two independent guards:
//   - Host header must be a loopback literal (403 otherwise). This blocks
//     DNS-rebinding attacks that would otherwise reach a 127.0.0.1 listener
//     through an attacker-controlled hostname.
//   - X-Ralph-Token header (or ?token= query) must match (401 otherwise).
//
// The URL query form exists for the first page load; subsequent XHRs are
// expected to send X-Ralph-Token. Since the listener binds 127.0.0.1 only,
// there is no cross-origin Referer leak path.
func AuthMiddleware(token string, next http.Handler) http.Handler {
	return LoopbackOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get("X-Ralph-Token")
		if provided == "" {
			provided = r.URL.Query().Get("token")
		}
		if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// LoopbackOnly restricts access to loopback clients without requiring a token.
func LoopbackOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) {
			http.Error(w, "forbidden: non-loopback Host", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackHost(host string) bool {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	h = strings.TrimPrefix(h, "[")
	h = strings.TrimSuffix(h, "]")
	switch h {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}
