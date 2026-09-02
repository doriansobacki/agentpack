// Package config holds agentpack's local (per-user) configuration and sync state.
package config

import (
	"os"
	"path/filepath"
)

// Home returns the agentpack home directory.
// Override with AGENTPACK_HOME (used by tests and demos).
func Home() string {
	if v := os.Getenv("AGENTPACK_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".agentpack"
	}
	return filepath.Join(home, ".agentpack")
}

// ClaudeDir returns the Claude Code user-level config directory.
// Override with AGENTPACK_CLAUDE_DIR (used by tests and demos).
func ClaudeDir() string {
	if v := os.Getenv("AGENTPACK_CLAUDE_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude"
	}
	return filepath.Join(home, ".claude")
}

// ConfigPath is the location of the user's agentpack profile.
func ConfigPath() string { return filepath.Join(Home(), "config.json") }

// StatePath is the location of the last-sync state file.
func StatePath() string { return filepath.Join(Home(), "state.json") }

// CacheDir is where remote org config repositories are cloned.
func CacheDir() string { return filepath.Join(Home(), "cache") }

// GeneratedDir is where merged cross-tool outputs (e.g. AGENTS.md) are written.
func GeneratedDir() string { return filepath.Join(Home(), "generated") }

// LogsDir is where scheduled and watch-mode syncs append their logs.
func LogsDir() string { return filepath.Join(Home(), "logs") }

// LockPath is the lock file that serializes concurrent syncs.
func LockPath() string { return filepath.Join(Home(), "sync.lock") }
