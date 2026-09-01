package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// UserConfig is the local profile created by `agentpack login`.
type UserConfig struct {
	// Email identifies the user; group membership is resolved from it.
	Email string `json:"email"`
	// Source is the org config location: a git URL or a local directory.
	Source string `json:"source"`
}

// ErrNotLoggedIn is returned when no local profile exists yet.
var ErrNotLoggedIn = errors.New("no agentpack profile found; run `agentpack login <email> --source <git-url-or-path>` first")

// LoadUserConfig reads the profile written by `agentpack login`.
func LoadUserConfig() (*UserConfig, error) {
	data, err := os.ReadFile(ConfigPath())
	if os.IsNotExist(err) {
		return nil, ErrNotLoggedIn
	}
	if err != nil {
		return nil, err
	}
	var c UserConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", ConfigPath(), err)
	}
	return &c, nil
}

// Save writes the profile to disk, creating the agentpack home if needed.
func (c *UserConfig) Save() error {
	return writeJSON(ConfigPath(), c)
}

// State records what the last sync produced, so the next sync can prune
// files that are no longer part of the user's resolved package set.
type State struct {
	LastSync time.Time `json:"lastSync"`
	Source   string    `json:"source"`
	Packages []string  `json:"packages"`
	// Files lists every file agentpack wrote (absolute paths). Anything in
	// this list that a subsequent sync does not write again gets deleted.
	Files []string `json:"files"`
	// ManagedBlocks lists files containing an agentpack-managed block. A
	// block a subsequent sync stops producing is removed from its file
	// (the file itself is never deleted).
	ManagedBlocks []string `json:"managedBlocks,omitempty"`
}

// LoadState reads the last sync state; a missing file yields an empty state.
func LoadState() (*State, error) {
	data, err := os.ReadFile(StatePath())
	if os.IsNotExist(err) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", StatePath(), err)
	}
	return &s, nil
}

// Save writes the sync state to disk.
func (s *State) Save() error {
	return writeJSON(StatePath(), s)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
