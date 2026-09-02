package synclog_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/doriansobacki/agentpack/internal/config"
	"github.com/doriansobacki/agentpack/internal/syncer"
	"github.com/doriansobacki/agentpack/internal/synclog"
)

func TestLine(t *testing.T) {
	report := &syncer.Report{
		Packages: []string{"a", "b"},
		Written:  []string{"f1", "f2", "f3"},
		Warnings: []string{"w"},
	}
	line := synclog.Line(report, nil)
	for _, want := range []string{" ok ", "packages=2", "written=3", "pruned=0", "warnings=1"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line missing %q: %s", want, line)
		}
	}

	// The error form must not touch the (possibly nil) report.
	errLine := synclog.Line(nil, errors.New("boom"))
	if !strings.Contains(errLine, `error "boom"`) {
		t.Fatalf("error line wrong: %s", errLine)
	}
}

func TestAppendAndRotate(t *testing.T) {
	t.Setenv("AGENTPACK_HOME", t.TempDir())
	path := filepath.Join(config.LogsDir(), "sync.log")

	if err := synclog.Append(&syncer.Report{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := synclog.Append(nil, errors.New("x")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(string(data), "\n"); lines != 2 {
		t.Fatalf("expected 2 lines, got %d:\n%s", lines, data)
	}

	// Grow past the rotation threshold and confirm the roll to sync.log.1.
	if err := os.WriteFile(path, make([]byte, 1<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := synclog.Append(&syncer.Report{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected rotated file: %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(string(data), "\n"); lines != 1 {
		t.Fatalf("fresh log should hold exactly the new line, got %d", lines)
	}
}
