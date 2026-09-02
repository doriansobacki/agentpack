//go:build linux

package scheduler

import (
	"os"
	"path/filepath"
)

func unitDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

// Install writes the systemd user service+timer and enables the timer.
func Install(cfg Config) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	dir, err := unitDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	service := filepath.Join(dir, TaskName+".service")
	timer := filepath.Join(dir, TaskName+".timer")
	if err := os.WriteFile(service, []byte(SystemdService(cfg)), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(timer, []byte(SystemdTimer(cfg)), 0o644); err != nil {
		return "", err
	}
	if _, err := run("systemctl", "--user", "daemon-reload"); err != nil {
		return "", err
	}
	if _, err := run("systemctl", "--user", "enable", "--now", TaskName+".timer"); err != nil {
		return "", err
	}
	return "systemd user timer " + TaskName + ".timer installed and started.", nil
}

// Uninstall disables the timer and removes the units; missing is not an error.
func Uninstall() (string, error) {
	dir, err := unitDir()
	if err != nil {
		return "", err
	}
	timer := filepath.Join(dir, TaskName+".timer")
	if _, statErr := os.Stat(timer); os.IsNotExist(statErr) {
		return "systemd user timer " + TaskName + ".timer is not installed.", nil
	}
	_, _ = run("systemctl", "--user", "disable", "--now", TaskName+".timer")
	_ = os.Remove(timer)
	_ = os.Remove(filepath.Join(dir, TaskName+".service"))
	if _, err := run("systemctl", "--user", "daemon-reload"); err != nil {
		return "", err
	}
	return "systemd user timer " + TaskName + ".timer removed.", nil
}

// Status reports the timer as systemd sees it.
func Status() (string, error) {
	return run("systemctl", "--user", "list-timers", TaskName+".timer", "--no-pager", "--all")
}
