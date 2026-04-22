package viewer

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"

	"github.com/ohjann/ralphplusplus/internal/userdata"
	"tailscale.com/client/local"
	"tailscale.com/tsnet"
)

// Tailnet runs the viewer as a node on the user's tailnet via tsnet. The
// node authenticates on first launch (tsnet prints a login URL) and persists
// its state under <userdata>/tsnet/, so subsequent launches reconnect
// silently. Once joined, peers on the tailnet can reach the viewer at
// http://<hostname>/ without a token — Tailscale's handshake is the
// authentication boundary.
type Tailnet struct {
	Srv      *tsnet.Server
	Client   *local.Client
	Hostname string
}

// NewTailnet constructs a tsnet.Server with its state dir under userdata,
// starts it (triggering interactive login on first run), and waits for the
// node to reach the Running state so the returned *Tailnet is immediately
// usable for Listen. The tsnet auth URL (if any) is written to the same
// writer used for banner output so users see the link without hunting logs.
func NewTailnet(ctx context.Context, hostname string, logTarget io.Writer) (*Tailnet, error) {
	d, err := userdata.Dir()
	if err != nil {
		return nil, fmt.Errorf("userdata dir: %w", err)
	}
	stateDir := filepath.Join(d, "tsnet", hostname)
	srv := &tsnet.Server{
		Hostname: hostname,
		Dir:      stateDir,
		// Quiet by default: Ralph logs are terse and tsnet's default stream
		// drowns the banner. Re-enable by setting TSNET_FORCE_LOG=1.
		Logf:         func(string, ...any) {},
		UserLogf:     func(format string, args ...any) { fmt.Fprintf(logTarget, format+"\n", args...) },
		Ephemeral:    false,
		RunWebClient: false,
	}
	if err := srv.Start(); err != nil {
		return nil, fmt.Errorf("tsnet start: %w", err)
	}
	lc, err := srv.LocalClient()
	if err != nil {
		_ = srv.Close()
		return nil, fmt.Errorf("tsnet local client: %w", err)
	}
	if _, err := srv.Up(ctx); err != nil {
		_ = srv.Close()
		return nil, fmt.Errorf("tsnet up: %w", err)
	}
	return &Tailnet{Srv: srv, Client: lc, Hostname: hostname}, nil
}

// Listen binds the tsnet node to the given tailnet address (e.g. ":80") and
// returns the listener. Callers wrap it with TrustHandler so peer identity
// becomes visible to downstream middleware.
func (t *Tailnet) Listen(addr string) (net.Listener, error) {
	return t.Srv.Listen("tcp", addr)
}

// Close shuts the tsnet node down. Safe to call multiple times.
func (t *Tailnet) Close() error {
	if t == nil || t.Srv == nil {
		return nil
	}
	return t.Srv.Close()
}

// TrustHandler wraps next so that every request whose remote addr maps to a
// known tailnet peer is marked trusted before it reaches the auth chain.
// Requests that fail WhoIs (e.g. node logged out mid-flight) fall through
// without a trust marker and will be rejected by the downstream guards —
// safer than default-allowing on lookup failure.
func (t *Tailnet) TrustHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		who, err := t.Client.WhoIs(r.Context(), r.RemoteAddr)
		if err == nil && who != nil && who.UserProfile != nil {
			r = WithTrust(r, who.UserProfile.LoginName)
		}
		next.ServeHTTP(w, r)
	})
}
