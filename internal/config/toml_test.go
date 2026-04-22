package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/BurntSushi/toml"
)

// TestSaveConfig_ConcurrentWritersProduceOneValidFile verifies RV-208's
// core invariant: two goroutines calling SaveConfig with different values
// at the same time serialize via the fcntl lock, and the final file is
// exactly one of the two writes — never a torn mix. Repeated many times
// to catch races.
func TestSaveConfig_ConcurrentWritersProduceOneValidFile(t *testing.T) {
	for trial := 0; trial < 100; trial++ {
		dir := t.TempDir()
		cfgA := &Config{ProjectDir: dir, Workers: 1}
		cfgB := &Config{ProjectDir: dir, Workers: 99}

		var wg sync.WaitGroup
		wg.Add(2)
		var errA, errB error
		go func() {
			defer wg.Done()
			errA = cfgA.SaveConfig()
		}()
		go func() {
			defer wg.Done()
			errB = cfgB.SaveConfig()
		}()
		wg.Wait()

		if errA != nil {
			t.Fatalf("trial %d: SaveConfig A: %v", trial, errA)
		}
		if errB != nil {
			t.Fatalf("trial %d: SaveConfig B: %v", trial, errB)
		}

		data, err := os.ReadFile(filepath.Join(dir, ".ralph", "config.toml"))
		if err != nil {
			t.Fatalf("trial %d: read result: %v", trial, err)
		}

		var parsed TomlConfig
		if err := toml.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("trial %d: result is not valid TOML: %v\n--- data ---\n%s", trial, err, data)
		}
		if parsed.Workers == nil {
			t.Fatalf("trial %d: Workers field missing from result", trial)
		}
		if *parsed.Workers != 1 && *parsed.Workers != 99 {
			t.Fatalf("trial %d: expected Workers 1 or 99, got %d (torn write)", trial, *parsed.Workers)
		}

		// No orphaned temp files survive beyond the rename.
		entries, err := os.ReadDir(filepath.Join(dir, ".ralph"))
		if err != nil {
			t.Fatalf("trial %d: readdir: %v", trial, err)
		}
		for _, e := range entries {
			if len(e.Name()) > 0 && e.Name() != "config.toml" && e.Name() != "config.toml.lock" {
				t.Fatalf("trial %d: stray file %q in .ralph", trial, e.Name())
			}
		}
	}
}
