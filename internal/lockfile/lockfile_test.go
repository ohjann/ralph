package lockfile_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ohjann/ralphplusplus/internal/lockfile"
)

func TestTryAcquireRejectsSecondLiveHolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	h, existing, err := lockfile.TryAcquire(path)
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	if existing != nil {
		t.Fatalf("unexpected existing info: %+v", existing)
	}
	defer h.Release()

	if err := h.WriteInfo(lockfile.Info{PID: os.Getpid(), StartedAt: time.Now()}); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	_, info, err := lockfile.TryAcquire(path)
	if err != lockfile.ErrLocked {
		t.Fatalf("expected ErrLocked, got %v", err)
	}
	if info == nil || info.PID != os.Getpid() {
		t.Fatalf("expected info pid %d, got %+v", os.Getpid(), info)
	}
}

func TestTryAcquireClearsStaleLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.lock")

	// Seed a stale lockfile whose pid is dead (99999 is reliably unused here).
	if err := os.WriteFile(path, []byte(`{"pid":99999,"startedAt":"2000-01-01T00:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("seed stale: %v", err)
	}

	h, existing, err := lockfile.TryAcquire(path)
	if err != nil {
		t.Fatalf("TryAcquire after stale: %v", err)
	}
	if existing != nil {
		t.Fatalf("stale should not surface as existing holder: %+v", existing)
	}
	h.Release()
}

func TestAcquireSerializesConcurrentWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serial.lock")

	const n = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	var order []int

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			h, err := lockfile.Acquire(path)
			if err != nil {
				t.Errorf("goroutine %d Acquire: %v", id, err)
				return
			}
			mu.Lock()
			order = append(order, id)
			mu.Unlock()
			// Hold briefly to force real serialization.
			time.Sleep(5 * time.Millisecond)
			h.Release()
		}(i)
	}
	wg.Wait()

	if len(order) != n {
		t.Fatalf("expected %d writers, got %d", n, len(order))
	}
}

