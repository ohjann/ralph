// Package lockfile provides fcntl advisory locks plus an optional JSON
// payload, used to serialize concurrent writers (config.toml) and enforce
// singleton processes (daemon) within a repo. A lockfile always carries
// {pid, startedAt} so contenders can report who holds it and detect stale
// holders whose process is dead.
package lockfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Info is the JSON payload persisted in a lockfile after acquisition.
type Info struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"startedAt"`
}

// Handle is a held advisory lock. Keep it for the lifetime of the critical
// section, then call Release. Zero value is invalid.
type Handle struct {
	file *os.File
}

// Acquire takes an exclusive fcntl advisory lock on path, blocking until the
// lock is granted. If the existing lockfile payload names a dead pid the
// file is treated as stale and cleaned up before acquisition. Parent
// directories are created if missing. Callers that need non-blocking
// semantics should use TryAcquire instead.
func Acquire(path string) (*Handle, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("ensure lockfile dir: %w", err)
	}
	clearIfStale(path)

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lockfile %s: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("flock %s: %w", path, err)
	}
	return &Handle{file: f}, nil
}

// TryAcquire attempts an exclusive fcntl lock without blocking. On
// contention it returns the existing Info (if parsable) and ErrLocked so
// callers can report who holds the lock. Stale lockfiles (dead pid) are
// cleaned up and retried once before giving up.
func TryAcquire(path string) (*Handle, *Info, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("ensure lockfile dir: %w", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("open lockfile %s: %w", path, err)
		}
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			return &Handle{file: f}, nil, nil
		}
		_ = f.Close()

		info, _ := readInfo(path)
		if info != nil && !processAlive(info.PID) {
			// Stale holder — remove and retry once.
			_ = os.Remove(path)
			continue
		}
		return nil, info, ErrLocked
	}
	return nil, nil, ErrLocked
}

// ErrLocked indicates the lock is held by a live process.
var ErrLocked = fmt.Errorf("lockfile: already held")

// WriteInfo replaces the lockfile payload with info. The caller must hold
// the lock (the Handle returned from Acquire/TryAcquire).
func (h *Handle) WriteInfo(info Info) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	if _, err := h.file.Seek(0, 0); err != nil {
		return err
	}
	if err := h.file.Truncate(0); err != nil {
		return err
	}
	if _, err := h.file.Write(data); err != nil {
		return err
	}
	return h.file.Sync()
}

// Release drops the advisory lock and closes the underlying file. Safe to
// call on a nil handle.
func (h *Handle) Release() {
	if h == nil || h.file == nil {
		return
	}
	_ = syscall.Flock(int(h.file.Fd()), syscall.LOCK_UN)
	_ = h.file.Close()
	h.file = nil
}

// clearIfStale removes path if its payload references a dead pid. Silent on
// any error — the subsequent Acquire path re-creates the file as needed.
func clearIfStale(path string) {
	info, err := readInfo(path)
	if err != nil || info == nil {
		return
	}
	if !processAlive(info.PID) {
		_ = os.Remove(path)
	}
}

func readInfo(path string) (*Info, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &info, nil
}

// processAlive reports whether pid refers to a live process by sending
// signal 0. ESRCH means the process is gone; EPERM (permission denied)
// still indicates the process exists so we treat the lock as live.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return err != syscall.ESRCH
}
