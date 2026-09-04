package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// lockStaleAfter is how old a lock file may be before it is presumed to be
// left over from a crashed process and stolen.
const lockStaleAfter = 15 * time.Minute

// AcquireLock serializes syncs across processes (an interactive sync and a
// scheduled one must not interleave writes). It returns a release function.
// A lock older than lockStaleAfter is treated as abandoned and taken over.
func AcquireLock() (release func(), err error) {
	path := LockPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			_, _ = f.WriteString(strconv.Itoa(os.Getpid()))
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			// The holder released it between our attempts; retry.
			continue
		}
		if time.Since(info.ModTime()) < lockStaleAfter {
			return nil, fmt.Errorf("another sync is already running (lock: %s); if that is wrong, delete the file", path)
		}
		// Stale: the owning process is presumed dead. Take the lock over.
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			return nil, fmt.Errorf("removing stale lock %s: %w", path, rmErr)
		}
	}
	return nil, fmt.Errorf("could not acquire sync lock %s", path)
}
