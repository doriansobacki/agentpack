package target_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/doriansobacki/agentpack/pkg/target"
)

func TestUpsertCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "CLAUDE.md")
	if err := target.UpsertManagedBlock(path, "hello rules"); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	if !strings.Contains(got, target.BeginMarker) || !strings.Contains(got, "hello rules") || !strings.Contains(got, target.EndMarker) {
		t.Fatalf("unexpected content:\n%s", got)
	}
}

func TestUpsertPreservesUserContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	write(t, path, "# my own notes\n\nkeep me\n")

	if err := target.UpsertManagedBlock(path, "v1"); err != nil {
		t.Fatal(err)
	}
	if err := target.UpsertManagedBlock(path, "v2"); err != nil {
		t.Fatal(err)
	}

	got := read(t, path)
	if !strings.Contains(got, "keep me") {
		t.Fatalf("user content lost:\n%s", got)
	}
	if strings.Contains(got, "v1") {
		t.Fatalf("old block content still present:\n%s", got)
	}
	if !strings.Contains(got, "v2") {
		t.Fatalf("new block content missing:\n%s", got)
	}
	if strings.Count(got, target.BeginMarker) != 1 {
		t.Fatalf("expected exactly one managed block:\n%s", got)
	}
}

func TestUpsertLoneMarkerFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	write(t, path, "text\n"+target.BeginMarker+"\nbroken\n")
	if err := target.UpsertManagedBlock(path, "x"); err == nil {
		t.Fatal("expected an error for a lone begin marker")
	}
}

func TestRemoveManagedBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	write(t, path, "before\n")
	if err := target.UpsertManagedBlock(path, "managed"); err != nil {
		t.Fatal(err)
	}
	if err := target.RemoveManagedBlock(path); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	if strings.Contains(got, target.BeginMarker) || strings.Contains(got, "managed") {
		t.Fatalf("block not removed:\n%s", got)
	}
	if !strings.Contains(got, "before") {
		t.Fatalf("user content lost:\n%s", got)
	}
}

func TestUpsertDefangsEmbeddedMarkers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	write(t, path, "user text\n")

	// Pack content that contains the literal markers (e.g. a pack that
	// documents agentpack itself) must not corrupt later splices.
	malicious := "rule about markers: " + target.EndMarker + " and " + target.BeginMarker
	for i := 0; i < 3; i++ {
		if err := target.UpsertManagedBlock(path, malicious); err != nil {
			t.Fatalf("upsert %d: %v", i+1, err)
		}
	}

	got := read(t, path)
	if strings.Count(got, target.BeginMarker) != 1 || strings.Count(got, target.EndMarker) != 1 {
		t.Fatalf("markers duplicated or lost after repeated upserts:\n%s", got)
	}
	if !strings.Contains(got, "user text") {
		t.Fatalf("user content lost:\n%s", got)
	}
	if !strings.Contains(got, "rule about markers") {
		t.Fatalf("sanitized rule content missing:\n%s", got)
	}
}

func TestRemoveOnMissingFileIsNoop(t *testing.T) {
	if err := target.RemoveManagedBlock(filepath.Join(t.TempDir(), "nope.md")); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func write(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
