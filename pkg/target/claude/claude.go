// Package claude materializes packs into Claude Code's user-level surfaces:
// skills into ~/.claude/skills/, agents into ~/.claude/agents/, and rules +
// memories into a managed block in ~/.claude/CLAUDE.md.
//
// This is the highest-fidelity target: every pack content type has a native
// Claude Code representation.
package claude

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/doriansobacki/agentpack/pkg/target"
)

// Name is this target's identifier in the org manifest's targets: list.
const Name = "claude"

// Target implements target.Target for Claude Code.
type Target struct{}

// New returns the Claude Code target.
func New() *Target { return &Target{} }

// Name implements target.Target.
func (*Target) Name() string { return Name }

// Apply implements target.Target.
func (*Target) Apply(ctx *target.Context) (*target.Result, error) {
	res := &target.Result{}

	for _, pack := range ctx.Packs {
		for _, skill := range pack.Skills {
			if err := applySkill(ctx, res, pack, skill); err != nil {
				return nil, err
			}
		}
		for _, agent := range pack.Agents {
			if err := applyAgent(ctx, res, pack, agent); err != nil {
				return nil, err
			}
		}
	}

	if err := applyMemoryBlock(ctx, res); err != nil {
		return nil, err
	}
	return res, nil
}

func applySkill(ctx *target.Context, res *target.Result, pack *target.Pack, skill target.SkillDir) error {
	dest := filepath.Join(ctx.ClaudeDir, "skills", skill.Name)

	if owner, ok := ctx.Claim(dest, pack.ID); !ok {
		res.Warnings = append(res.Warnings,
			fmt.Sprintf("skill %q is provided by packs %s and %s; keeping %s's version", skill.Name, owner, pack.ID, owner))
		return nil
	}

	// A destination holding files we did not create belongs to the user:
	// skip the write, and retain what we do own there so the syncer neither
	// prunes it (which would half-delete the skill) nor forgets we own it.
	if foreign, err := isForeign(ctx, dest); err != nil {
		return err
	} else if foreign {
		res.Retained = append(res.Retained, ctx.RetainOwnedUnder(dest)...)
		res.Warnings = append(res.Warnings,
			fmt.Sprintf("skill %q (pack %s): %s contains files not created by agentpack; left untouched", skill.Name, pack.ID, dest))
		return nil
	}

	files, err := copyTree(skill.Path, dest, ctx.DryRun)
	if err != nil {
		return fmt.Errorf("copying skill %q: %w", skill.Name, err)
	}
	res.Files = append(res.Files, files...)
	return nil
}

func applyAgent(ctx *target.Context, res *target.Result, pack *target.Pack, agent target.File) error {
	dest := filepath.Join(ctx.ClaudeDir, "agents", agent.Name)

	if owner, ok := ctx.Claim(dest, pack.ID); !ok {
		res.Warnings = append(res.Warnings,
			fmt.Sprintf("agent %q is provided by packs %s and %s; keeping %s's version", agent.Name, owner, pack.ID, owner))
		return nil
	}

	_, statErr := os.Stat(dest)
	exists := statErr == nil
	if !ctx.Owns(dest, exists) {
		res.Warnings = append(res.Warnings,
			fmt.Sprintf("agent %q (pack %s): %s already exists and was not created by agentpack; skipped", agent.Name, pack.ID, dest))
		return nil
	}
	if !ctx.DryRun {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, agent.Content, 0o644); err != nil {
			return err
		}
	}
	res.Files = append(res.Files, dest)
	return nil
}

// applyMemoryBlock writes the rules+memories block and reports it via
// Result.Blocks. Removal of a stale block (content gone, or this target no
// longer configured) is the syncer's job, driven by the tracked state.
func applyMemoryBlock(ctx *target.Context, res *target.Result) error {
	content := target.MergedMarkdown(ctx.Packs, true)
	if content == "" {
		return nil
	}
	claudeMD := filepath.Join(ctx.ClaudeDir, "CLAUDE.md")
	res.Blocks = append(res.Blocks, claudeMD)
	if ctx.DryRun {
		return nil
	}
	return target.UpsertManagedBlock(claudeMD, content)
}

// isForeign reports whether dest exists but contains files agentpack did not
// write on the previous sync.
func isForeign(ctx *target.Context, dest string) (bool, error) {
	info, err := os.Stat(dest)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return !ctx.PreviouslyOwned[dest], nil
	}
	foreign := false
	err = filepath.WalkDir(dest, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && !ctx.PreviouslyOwned[path] {
			foreign = true
			return fs.SkipAll
		}
		return nil
	})
	return foreign, err
}

// copyTree copies src into dest (replacing dest first so files deleted from
// the pack disappear too) and returns the list of files written.
func copyTree(src, dest string, dryRun bool) ([]string, error) {
	var files []string
	if !dryRun {
		if err := os.RemoveAll(dest); err != nil {
			return nil, err
		}
	}
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dest, rel)
		if d.IsDir() {
			if !dryRun {
				return os.MkdirAll(out, 0o755)
			}
			return nil
		}
		files = append(files, out)
		if dryRun {
			return nil
		}
		return copyFile(path, out)
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// copyFile streams src into dst with constant memory, so large skill support
// files (datasets, binaries) are not buffered whole on the heap.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
