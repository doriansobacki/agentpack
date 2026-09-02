//go:build darwin

package scheduler

import (
	"os"
	"path/filepath"
)

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", LaunchdLabel+".plist"), nil
}

// Install writes the launch agent plist and (re)loads it.
func Install(cfg Config) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	path, err := plistPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(LaunchdPlist(cfg)), 0o644); err != nil {
		return "", err
	}
	// Reload so an updated interval takes effect; the unload of a job that
	// was never loaded fails harmlessly.
	_, _ = run("launchctl", "unload", path)
	if _, err := run("launchctl", "load", path); err != nil {
		return "", err
	}
	return "Launch agent " + LaunchdLabel + " installed (" + path + ").", nil
}

// Uninstall unloads and removes the launch agent; missing is not an error.
func Uninstall() (string, error) {
	path, err := plistPath()
	if err != nil {
		return "", err
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return "Launch agent " + LaunchdLabel + " is not installed.", nil
	}
	_, _ = run("launchctl", "unload", path)
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return "Launch agent " + LaunchdLabel + " removed.", nil
}

// Status reports the job as launchd sees it.
func Status() (string, error) {
	return run("launchctl", "list", LaunchdLabel)
}
