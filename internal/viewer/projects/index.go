// Package projects caches history.LoadAllReposWithFP behind a short TTL and
// an fsnotify-backed invalidator. The viewer's sidebar is loaded on every
// SPA bootstrap and page refresh; without this cache the repo listing would
// re-walk <userdata>/repos and re-parse every meta.json on each hit.
package projects

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/ohjann/ralphplusplus/internal/history"
	"github.com/ohjann/ralphplusplus/internal/userdata"
)

// DefaultTTL is the freshness window for cached results. Two seconds is long
// enough to absorb a burst of SPA requests but short enough that a missed
// fsnotify event (e.g. editor-tool Write that bypasses Create/Rename/Remove)
// self-corrects quickly.
const DefaultTTL = 2 * time.Second

// Index is a hot cache over history.LoadAllReposWithFP.
//
// Reads are served from memory for up to TTL. An fsnotify watcher on the
// repos directory invalidates the cache whenever a repo dir appears,
// disappears, or is renamed — the three events that can change the
// LoadAllReposWithFP result set. meta.json edits inside an existing fp dir
// are not watched because LoadAllReposWithFP only reads the dir name and
// the static identity fields; the TTL absorbs churn within a dir.
type Index struct {
	ttl      time.Duration
	loader   func() ([]history.RepoWithFP, error)
	nowFn    func() time.Time
	reposDir string

	mu      sync.Mutex
	cached  []history.RepoWithFP
	expires time.Time
}

// Option configures an Index at construction time.
type Option func(*Index)

// WithTTL overrides DefaultTTL.
func WithTTL(d time.Duration) Option { return func(i *Index) { i.ttl = d } }

// WithLoader injects a custom loader (tests).
func WithLoader(fn func() ([]history.RepoWithFP, error)) Option {
	return func(i *Index) { i.loader = fn }
}

// WithNow injects a custom clock (tests).
func WithNow(fn func() time.Time) Option { return func(i *Index) { i.nowFn = fn } }

// WithReposDir overrides the directory the watcher observes. Defaults to
// userdata.ReposDir().
func WithReposDir(path string) Option { return func(i *Index) { i.reposDir = path } }

// New constructs an Index, creates the repos dir if absent, and starts the
// fsnotify watcher goroutine. The watcher runs until ctx is cancelled.
func New(ctx context.Context, opts ...Option) (*Index, error) {
	i := &Index{
		ttl:    DefaultTTL,
		loader: history.LoadAllReposWithFP,
		nowFn:  time.Now,
	}
	for _, o := range opts {
		o(i)
	}
	if i.reposDir == "" {
		rd, err := userdata.ReposDir()
		if err != nil {
			return nil, fmt.Errorf("repos dir: %w", err)
		}
		i.reposDir = rd
	}
	// fsnotify.Add fails if the directory doesn't exist yet, which is the
	// common case on a fresh install.
	if err := userdata.EnsureDirs(i.reposDir); err != nil {
		return nil, fmt.Errorf("ensure repos dir: %w", err)
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fsnotify: %w", err)
	}
	if err := w.Add(i.reposDir); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("watch %s: %w", i.reposDir, err)
	}
	go i.watch(ctx, w)
	return i, nil
}

// watch forwards Create/Rename/Remove events to Invalidate until ctx is
// done or the watcher channel closes. Other ops (Write, Chmod) are ignored
// because they cannot change the repo-dir membership of the repos dir.
func (i *Index) watch(ctx context.Context, w *fsnotify.Watcher) {
	defer w.Close()
	const invalidating = fsnotify.Create | fsnotify.Rename | fsnotify.Remove
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if ev.Op&invalidating != 0 {
				i.Invalidate()
			}
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
}

// Invalidate drops the cached result so the next Get reloads from disk.
func (i *Index) Invalidate() {
	i.mu.Lock()
	i.cached = nil
	i.expires = time.Time{}
	i.mu.Unlock()
}

// Get returns the cached repo list, reloading if the cache is empty or stale.
// A nil loader result is coerced to an empty (non-nil) slice so subsequent
// reads are cache hits instead of reloading on every call.
func (i *Index) Get(ctx context.Context) ([]history.RepoWithFP, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.cached != nil && i.nowFn().Before(i.expires) {
		return i.cached, nil
	}
	repos, err := i.loader()
	if err != nil {
		return nil, err
	}
	if repos == nil {
		repos = []history.RepoWithFP{}
	}
	i.cached = repos
	i.expires = i.nowFn().Add(i.ttl)
	return repos, nil
}
