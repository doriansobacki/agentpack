// Package synclog appends one-line sync summaries to a log file, so
// scheduled (headless) syncs stay diagnosable without popping any UI.
package synclog

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/doriansobacki/agentpack/internal/config"
	"github.com/doriansobacki/agentpack/internal/syncer"
)

// rotateAt is the size at which sync.log is rolled to sync.log.1.
const rotateAt = 1 << 20 // 1 MiB

// Line formats one sync outcome as a single log line (no trailing newline).
func Line(report *syncer.Report, syncErr error) string {
	ts := time.Now().UTC().Format(time.RFC3339)
	if syncErr != nil {
		return fmt.Sprintf("%s error %q", ts, syncErr.Error())
	}
	return fmt.Sprintf("%s ok packages=%d written=%d pruned=%d warnings=%d",
		ts, len(report.Packages), len(report.Written), len(report.Pruned), len(report.Warnings))
}

// Append writes one outcome line to <home>/logs/sync.log, rotating a single
// generation (sync.log.1) when the file grows past rotateAt.
func Append(report *syncer.Report, syncErr error) error {
	dir := config.LogsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "sync.log")
	if info, err := os.Stat(path); err == nil && info.Size() >= rotateAt {
		if err := os.Rename(path, path+".1"); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(Line(report, syncErr) + "\n")
	return err
}
