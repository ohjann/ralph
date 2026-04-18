// Package viewer provides the singleton HTTP server that serves Ralph's
// web UI on localhost. Two concurrent viewers on the same host would race
// for project data, so startup is guarded by an fcntl advisory lock whose
// payload doubles as a discovery record ({pid, port, startedAt}) for the
// second invocation to learn the URL instead of failing.
package viewer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ohjann/ralphplusplus/internal/userdata"
)

// LockInfo is the payload written to viewer.lock after the lock is acquired.
// A second invocation reads this record to locate the live viewer.
type LockInfo struct {
	PID       int       `json:"pid"`
	Port      int       `json:"port"`
	StartedAt time.Time `json:"startedAt"`
}

// LockPath returns <userdata>/viewer.lock.
func LockPath() (string, error) {
	d, err := userdata.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "viewer.lock"), nil
}

// Acquire tries to take the fcntl advisory write-lock on viewer.lock. On
// success it returns the held file handle (keep it open for the viewer's
// lifetime). On contention it returns the existing LockInfo so the caller
// can print the live URL and exit. Callers must not write to the file until
// Write is called.
func Acquire() (*os.File, *LockInfo, error) {
	path, err := LockPath()
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("ensure viewer dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open lockfile: %w", err)
	}

	lk := syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Whence: 0,
		Start:  0,
		Len:    0,
	}
	if err := syscall.FcntlFlock(f.Fd(), syscall.F_SETLK, &lk); err != nil {
		_ = f.Close()
		existing, readErr := readLockInfo(path)
		if readErr != nil {
			return nil, nil, fmt.Errorf("lock held but lockfile unreadable: %w", readErr)
		}
		return nil, existing, nil
	}
	return f, nil, nil
}

// Write replaces the lockfile payload with info. The caller must hold the
// lock (i.e. pass the file returned from Acquire).
func Write(f *os.File, info LockInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

// Release drops the advisory lock and closes the file. Safe to call with nil.
func Release(f *os.File) {
	if f == nil {
		return
	}
	lk := syscall.Flock_t{Type: syscall.F_UNLCK}
	_ = syscall.FcntlFlock(f.Fd(), syscall.F_SETLK, &lk)
	_ = f.Close()
}

func readLockInfo(path string) (*LockInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &info, nil
}
