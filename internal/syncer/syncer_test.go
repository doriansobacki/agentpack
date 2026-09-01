package syncer_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/doriansobacki/agentpack/internal/builtins"
	"github.com/doriansobacki/agentpack/internal/config"
	"github.com/doriansobacki/agentpack/internal/syncer"
)

const manifestFull = `
targets: [claude, agentsmd]
groups:
  "*": [org-baseline]
  team-a: [dotnet]
users:
  dev@example.com: [team-a]
`

const manifestNoTeam = `
targets: [claude, agentsmd]
groups:
  "*": [org-baseline]
  team-a: [dotnet]
users:
  dev@example.com: []
`

func setup(t *testing.T) (orgDir string) {
	t.Helper()
	builtins.Register()
	t.Setenv("AGENTPACK_HOME", filepath.Join(t.TempDir(), "home"))
	t.Setenv("AGENTPACK_CLAUDE_DIR", filepath.Join(t.TempDir(), "claude"))

	orgDir = t.TempDir()
	files := map[string]string{
		"agentpack.yaml": manifestFull,
		"packages/org-baseline/rules/standards.md":       "org rule: reference a ticket",
		"packages/org-baseline/memories/glossary.md":     "a pack is a bundle",
		"packages/dotnet/rules/dotnet.md":                "dotnet rule: use xunit",
		"packages/dotnet/skills/dotnet-testing/SKILL.md": "---\nname: dotnet-testing\n---\ntest skill",
		"packages/dotnet/agents/dotnet-reviewer.md":      "reviewer agent",
	}
	for rel, content := range files {
		path := filepath.Join(orgDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.UserConfig{Email: "dev@example.com", Source: orgDir}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	return orgDir
}

func TestSyncMaterializesAndPrunes(t *testing.T) {
	orgDir := setup(t)

	report, err := syncer.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"org-baseline", "dotnet"}; strings.Join(report.Packages, ",") != strings.Join(want, ",") {
		t.Fatalf("packages = %v, want %v", report.Packages, want)
	}

	claudeDir := os.Getenv("AGENTPACK_CLAUDE_DIR")
	skillPath := filepath.Join(claudeDir, "skills", "dotnet-testing", "SKILL.md")
	agentPath := filepath.Join(claudeDir, "agents", "dotnet-reviewer.md")
	claudeMD := read(t, filepath.Join(claudeDir, "CLAUDE.md"))
	agentsMD := read(t, filepath.Join(config.GeneratedDir(), "AGENTS.md"))

	for _, path := range []string{skillPath, agentPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	for _, want := range []string{"org rule: reference a ticket", "dotnet rule: use xunit", "a pack is a bundle"} {
		if !strings.Contains(claudeMD, want) {
			t.Fatalf("CLAUDE.md missing %q:\n%s", want, claudeMD)
		}
		if !strings.Contains(agentsMD, want) {
			t.Fatalf("AGENTS.md missing %q:\n%s", want, agentsMD)
		}
	}

	// Drop team-a membership: the dotnet pack's files must be pruned.
	if err := os.WriteFile(filepath.Join(orgDir, "agentpack.yaml"), []byte(manifestNoTeam), 0o644); err != nil {
		t.Fatal(err)
	}
	report2, err := syncer.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report2.Pruned) == 0 {
		t.Fatalf("expected pruned files, got none (written: %v)", report2.Written)
	}
	if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Fatalf("skill should be pruned: %v", err)
	}
	if _, err := os.Stat(agentPath); !os.IsNotExist(err) {
		t.Fatalf("agent should be pruned: %v", err)
	}
	claudeMD = read(t, filepath.Join(claudeDir, "CLAUDE.md"))
	if strings.Contains(claudeMD, "dotnet rule") {
		t.Fatalf("CLAUDE.md still contains dropped pack content:\n%s", claudeMD)
	}
	if !strings.Contains(claudeMD, "org rule: reference a ticket") {
		t.Fatalf("CLAUDE.md lost wildcard pack content:\n%s", claudeMD)
	}
}

func TestSyncDoesNotClobberForeignFiles(t *testing.T) {
	setup(t)

	// A user-made skill already occupies the destination.
	claudeDir := os.Getenv("AGENTPACK_CLAUDE_DIR")
	foreign := filepath.Join(claudeDir, "skills", "dotnet-testing", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(foreign), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, []byte("mine, hands off"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := syncer.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, foreign); got != "mine, hands off" {
		t.Fatalf("foreign file was overwritten: %q", got)
	}
	if len(report.Warnings) == 0 {
		t.Fatal("expected a warning about the skipped skill")
	}
}

func TestSyncDryRunWritesNothing(t *testing.T) {
	setup(t)
	report, err := syncer.Sync(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Written) == 0 {
		t.Fatal("dry run should still report what it would write")
	}
	claudeDir := os.Getenv("AGENTPACK_CLAUDE_DIR")
	if _, err := os.Stat(claudeDir); !os.IsNotExist(err) {
		t.Fatalf("dry run created files under %s", claudeDir)
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
