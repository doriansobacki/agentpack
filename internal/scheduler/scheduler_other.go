//go:build !windows && !darwin && !linux

package scheduler

import (
	"fmt"
	"runtime"
)

func unsupported() error {
	return fmt.Errorf("scheduler: %s is not supported; use `agentpack watch` instead", runtime.GOOS)
}

// Install is unsupported on this OS.
func Install(Config) (string, error) { return "", unsupported() }

// Uninstall is unsupported on this OS.
func Uninstall() (string, error) { return "", unsupported() }

// Status is unsupported on this OS.
func Status() (string, error) { return "", unsupported() }
