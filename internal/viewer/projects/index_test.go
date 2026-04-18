package projects_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ohjann/ralphplusplus/internal/history"
	"github.com/ohjann/ralphplusplus/internal/userdata"
	"github.com/ohjann/ralphplusplus/internal/viewer/projects"
)

// TestIndex_InvalidatesOnMkdir is the spec check from the RV-002 story:
// an fsnotify-driven invalidation must fire within 250ms of a new repo
// directory appearing in <userdata>/repos/.
func TestIndex_InvalidatesOnMkdir(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("RALPH_DATA_DIR", dataDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	idx, err := projects.New(ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Prime the cache with the (empty) initial state so subsequent reads
	// hit the cache until fsnotify invalidates it.
	got, err := idx.Get(ctx)
	if err != nil {
		t.Fatalf("Get(initial): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Get(initial): want empty, got %d entries", len(got))
	}

	reposDir, err := userdata.ReposDir()
	if err != nil {
		t.Fatalf("ReposDir: %v", err)
	}

	// Stage the fp dir with meta.json in a sibling location so the rename
	// into reposDir surfaces as a single fsnotify Create event with the
	// meta file already present — otherwise the loader races the writer.
	const fp = "feedfacecafe"
	scratch := filepath.Join(dataDir, "scratch")
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		t.Fatalf("mkdir scratch: %v", err)
	}
	now := time.Now().UTC()
	meta := history.RepoMeta{
		Path:      filepath.Join(dataDir, "fake"),
		Name:      "fake",
		FirstSeen: now,
		LastSeen:  now,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scratch, "meta.json"), data, 0o644); err != nil {
		t.Fatalf("write meta.json: %v", err)
	}

	start := time.Now()
	if err := os.Rename(scratch, filepath.Join(reposDir, fp)); err != nil {
		t.Fatalf("rename: %v", err)
	}

	// Poll Get until the cache reflects the new repo. Any reload within
	// the budget is proof that fsnotify invalidated the TTL early; without
	// it the cache would sit on the empty slice for the full 2s.
	deadline := start.Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		got, err := idx.Get(ctx)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if len(got) == 1 && got[0].FP == fp {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("cache did not invalidate within 250ms of mkdir")
}

// TestIndex_GetServesFromCache verifies the 2s TTL: a second loader call is
// not made inside the window, and a manual Invalidate triggers a reload.
func TestIndex_GetServesFromCache(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls int
	idx, err := projects.New(ctx,
		projects.WithLoader(func() ([]history.RepoWithFP, error) {
			calls++
			return []history.RepoWithFP{{FP: "cafebabe"}}, nil
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for range 5 {
		got, err := idx.Get(ctx)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("Get: want 1 entry, got %d", len(got))
		}
	}
	if calls != 1 {
		t.Fatalf("loader called %d times, want 1 (cache should absorb repeat reads)", calls)
	}

	idx.Invalidate()
	if _, err := idx.Get(ctx); err != nil {
		t.Fatalf("Get after invalidate: %v", err)
	}
	if calls != 2 {
		t.Fatalf("loader called %d times after invalidate, want 2", calls)
	}
}

// TestIndex_TTLExpires checks that the cache re-reads after DefaultTTL by
// injecting a time source. Avoids real wall-clock sleeping.
func TestIndex_TTLExpires(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls int
	clock := time.Unix(0, 0)
	idx, err := projects.New(ctx,
		projects.WithTTL(2*time.Second),
		projects.WithLoader(func() ([]history.RepoWithFP, error) {
			calls++
			return nil, nil
		}),
		projects.WithNow(func() time.Time { return clock }),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := idx.Get(ctx); err != nil {
		t.Fatalf("Get: %v", err)
	}
	clock = clock.Add(1 * time.Second)
	if _, err := idx.Get(ctx); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if calls != 1 {
		t.Fatalf("loader called %d times inside TTL, want 1", calls)
	}

	clock = clock.Add(2 * time.Second) // now past expiry
	if _, err := idx.Get(ctx); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if calls != 2 {
		t.Fatalf("loader called %d times after TTL, want 2", calls)
	}
}
