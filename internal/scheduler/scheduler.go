// Package scheduler registers agentpack with the operating system's native
// per-user job scheduler (Task Scheduler, launchd, systemd user timers), so
// `agentpack sync` runs on an interval with no daemon of our own to keep
// alive. Install/Uninstall/Status are implemented per OS; the file and
// argument builders are pure functions shared across platforms for testing.
package scheduler

import (
	"fmt"
	"os/exec"
	"time"
)

// TaskName identifies the job in every scheduler (task name, launchd label,
// systemd unit name).
const TaskName = "agentpack-sync"

// LaunchdLabel is the reverse-DNS label used on macOS.
const LaunchdLabel = "dev.agentpack.sync"

// Config describes the job to install.
type Config struct {
	// Executable is the absolute path of the agentpack binary to run.
	Executable string
	// Interval between syncs. Schedulers round it to their granularity
	// (Task Scheduler: whole minutes, minimum 1).
	Interval time.Duration
	// LogDir is where the scheduled runs write their output.
	LogDir string
}

// MinInterval is the smallest supported interval (Task Scheduler's floor).
const MinInterval = time.Minute

// Validate rejects configs no scheduler can express.
func (c Config) Validate() error {
	if c.Executable == "" {
		return fmt.Errorf("scheduler: executable path is empty")
	}
	if c.Interval < MinInterval {
		return fmt.Errorf("scheduler: interval %s is below the minimum %s", c.Interval, MinInterval)
	}
	return nil
}

// run executes a scheduler CLI (schtasks, launchctl, systemctl) and returns
// its combined output.
func run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %v: %w\n%s", name, args, err, out)
	}
	return string(out), nil
}
