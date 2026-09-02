package scheduler

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"
)

// SchtasksCreateArgs builds the Windows `schtasks /Create` argument list for
// a per-user task running `agentpack sync --scheduled` every interval
// (rounded up to whole minutes, Task Scheduler's granularity).
func SchtasksCreateArgs(cfg Config) []string {
	minutes := int(cfg.Interval.Round(time.Minute) / time.Minute)
	if minutes < 1 {
		minutes = 1
	}
	return []string{
		"/Create", "/F",
		"/SC", "MINUTE", "/MO", strconv.Itoa(minutes),
		"/TN", TaskName,
		"/TR", fmt.Sprintf(`"%s" sync --scheduled`, cfg.Executable),
	}
}

// LaunchdPlist renders the macOS launch agent for the sync job.
func LaunchdPlist(cfg Config) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>sync</string>
		<string>--scheduled</string>
	</array>
	<key>StartInterval</key>
	<integer>%d</integer>
	<key>RunAtLoad</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, LaunchdLabel, cfg.Executable,
		int(cfg.Interval/time.Second),
		filepath.Join(cfg.LogDir, "launchd.log"),
		filepath.Join(cfg.LogDir, "launchd.log"))
}

// SystemdService renders the systemd user service unit for one sync run.
func SystemdService(cfg Config) string {
	return fmt.Sprintf(`[Unit]
Description=agentpack sync

[Service]
Type=oneshot
ExecStart=%s sync --scheduled
`, cfg.Executable)
}

// SystemdTimer renders the systemd user timer driving the service.
func SystemdTimer(cfg Config) string {
	seconds := int(cfg.Interval / time.Second)
	return fmt.Sprintf(`[Unit]
Description=Run agentpack sync every %s

[Timer]
OnBootSec=1min
OnUnitActiveSec=%ds
Persistent=true

[Install]
WantedBy=timers.target
`, cfg.Interval, seconds)
}
