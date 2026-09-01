package syncer_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/doriansobacki/agentpack/internal/builtins"
	"github.com/doriansobacki/agentpack/internal/config"
	"github.com/doriansobacki/agentpack/internal/syncer"
	"github.com/doriansobacki/agentpack/pkg/target"
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

func TestForeignFileInsideOwnedSkillIsRetained(t *testing.T) {
	setup(t)
	if _, err := syncer.Sync(context.Background(), false); err != nil {
		t.Fatal(err)
	}

	claudeDir := os.Getenv("AGENTPACK_CLAUDE_DIR")
	skillDir := filepath.Join(claudeDir, "skills", "dotnet-testing")
	skillMD := filepath.Join(skillDir, "SKILL.md")

	// User drops a personal file into the agentpack-owned skill directory.
	if err := os.WriteFile(filepath.Join(skillDir, "notes.md"), []byte("my notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The next syncs must neither prune SKILL.md nor touch notes.md.
	for i := 0; i < 2; i++ {
		report, err := syncer.Sync(context.Background(), false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(skillMD); err != nil {
			t.Fatalf("sync %d pruned SKILL.md out of a skipped skill: %v", i+2, err)
		}
		if got := read(t, filepath.Join(skillDir, "notes.md")); got != "my notes" {
			t.Fatalf("user file modified: %q", got)
		}
		if len(report.Warnings) == 0 {
			t.Fatal("expected a warning about the untouched skill")
		}
	}
}

func TestDuplicateAgentAcrossPacksIsDeterministic(t *testing.T) {
	orgDir := setup(t)

	// A second pack ships an agent with the same file name as dotnet's,
	// and the user receives both packs.
	extra := filepath.Join(orgDir, "packages", "extra", "agents", "dotnet-reviewer.md")
	if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(extra, []byte("impostor agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `
targets: [claude, agentsmd]
groups:
  "*": [org-baseline]
  team-a: [dotnet, extra]
users:
  dev@example.com: [team-a]
`
	if err := os.WriteFile(filepath.Join(orgDir, "agentpack.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	agentPath := filepath.Join(os.Getenv("AGENTPACK_CLAUDE_DIR"), "agents", "dotnet-reviewer.md")
	for i := 0; i < 2; i++ {
		report, err := syncer.Sync(context.Background(), false)
		if err != nil {
			t.Fatal(err)
		}
		// First pack in resolution order (dotnet) must win on EVERY sync.
		if got := read(t, agentPath); got != "reviewer agent" {
			t.Fatalf("sync %d: winner flipped, agent content = %q", i+1, got)
		}
		found := false
		for _, w := range report.Warnings {
			if strings.Contains(w, "provided by packs") {
				found = true
			}
		}
		if !found {
			t.Fatalf("sync %d: expected a duplicate-provider warning, got %v", i+1, report.Warnings)
		}
	}
}

func TestUnknownTargetWritesNothing(t *testing.T) {
	orgDir := setup(t)
	manifest := strings.Replace(manifestFull, "[claude, agentsmd]", "[claude, nope]", 1)
	if err := os.WriteFile(filepath.Join(orgDir, "agentpack.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := syncer.Sync(context.Background(), false); err == nil {
		t.Fatal("expected an unknown-target error")
	}
	claudeDir := os.Getenv("AGENTPACK_CLAUDE_DIR")
	if _, err := os.Stat(claudeDir); !os.IsNotExist(err) {
		t.Fatalf("unknown target still let files be written under %s", claudeDir)
	}
}

type boomTarget struct{}

func (boomTarget) Name() string { return "boom" }
func (boomTarget) Apply(*target.Context) (*target.Result, error) {
	return nil, errors.New("boom")
}

var registerBoom sync.Once

func TestTargetFailureSalvagesState(t *testing.T) {
	registerBoom.Do(func() { target.Register(boomTarget{}) })
	orgDir := setup(t)
	manifest := strings.Replace(manifestFull, "[claude, agentsmd]", "[claude, boom]", 1)
	if err := os.WriteFile(filepath.Join(orgDir, "agentpack.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// The claude target writes its files, then boom fails the sync.
	if _, err := syncer.Sync(context.Background(), false); err == nil {
		t.Fatal("expected the boom target to fail the sync")
	}

	// After fixing the manifest, the files written before the failure must
	// be recognized as agentpack-owned — updated, not skipped as foreign.
	if err := os.WriteFile(filepath.Join(orgDir, "agentpack.yaml"), []byte(manifestFull), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := syncer.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range report.Warnings {
		if strings.Contains(w, "not created by agentpack") {
			t.Fatalf("agentpack's own pre-failure files treated as foreign: %v", report.Warnings)
		}
	}
	skillPath := filepath.Join(os.Getenv("AGENTPACK_CLAUDE_DIR"), "skills", "dotnet-testing", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected %s to be managed after recovery: %v", skillPath, err)
	}
}

func TestDroppingClaudeTargetRemovesManagedBlock(t *testing.T) {
	orgDir := setup(t)
	if _, err := syncer.Sync(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	claudeMD := filepath.Join(os.Getenv("AGENTPACK_CLAUDE_DIR"), "CLAUDE.md")
	if got := read(t, claudeMD); !strings.Contains(got, target.BeginMarker) {
		t.Fatalf("expected a managed block after first sync:\n%s", got)
	}

	manifest := strings.Replace(manifestFull, "[claude, agentsmd]", "[agentsmd]", 1)
	if err := os.WriteFile(filepath.Join(orgDir, "agentpack.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := syncer.Sync(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.PrunedBlocks) != 1 {
		t.Fatalf("expected one pruned block, got %v", report.PrunedBlocks)
	}
	if got := read(t, claudeMD); strings.Contains(got, target.BeginMarker) || strings.Contains(got, "org rule") {
		t.Fatalf("stale managed block survived dropping the claude target:\n%s", got)
	}
}

func TestUnknownEmailErrors(t *testing.T) {
	setup(t)
	cfg, err := config.LoadUserConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Email = "typo@example.com"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	_, err = syncer.Sync(context.Background(), false)
	if err == nil || !strings.Contains(err.Error(), "not listed") {
		t.Fatalf("expected an unknown-email error, got %v", err)
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
