package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/doriansobacki/agentpack/internal/config"
)

func TestLockIsExclusive(t *testing.T) {
	t.Setenv("AGENTPACK_HOME", t.TempDir())

	release, err := config.AcquireLock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.AcquireLock(); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("second acquire should fail while held, got %v", err)
	}

	release()
	release2, err := config.AcquireLock()
	if err != nil {
		t.Fatalf("acquire after release failed: %v", err)
	}
	release2()
}

func TestStaleLockIsStolen(t *testing.T) {
	t.Setenv("AGENTPACK_HOME", t.TempDir())

	path := config.LockPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	release, err := config.AcquireLock()
	if err != nil {
		t.Fatalf("stale lock not stolen: %v", err)
	}
	release()
}
