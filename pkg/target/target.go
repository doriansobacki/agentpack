// Package target defines the extension point for materializing resolved
// packs into a specific AI tool's configuration surfaces.
//
// Built-in targets live in subpackages: claude (Claude Code: skills, agents,
// managed CLAUDE.md block), agentsmd (a merged AGENTS.md for tools that read
// the cross-vendor standard), cursor (experimental). New tools implement
// Target and register themselves.
//
// Targets receive packs as plain data (loaded file contents), not paths into
// the org repo, so implementations stay decoupled from the config-repo layout.
package target

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/doriansobacki/agentpack/internal/registry"
)

// File is one loaded content file from a pack.
type File struct {
	// Name is the base file name (e.g. "coding-standards.md").
	Name string
	// Content is the raw file content.
	Content []byte
}

// SkillDir points at a skill directory (containing SKILL.md and any support
// files). Skills are copied as directory trees, so they stay path-based.
type SkillDir struct {
	// Name is the skill's directory name, which is its identifier.
	Name string
	// Path is the absolute path of the source directory.
	Path string
}

// Pack is one resolved package, ready to materialize.
type Pack struct {
	ID          string
	Name        string
	Description string
	Rules       []File
	Memories    []File
	Agents      []File
	Skills      []SkillDir
}

// Context is the input to a target's Apply.
type Context struct {
	// Packs, in resolution order (org-wide first, then team, then role).
	Packs []*Pack
	// ClaudeDir is the Claude Code user config directory (~/.claude).
	ClaudeDir string
	// GeneratedDir is agentpack's output directory for merged files that
	// have no native per-user surface in the tool (~/.agentpack/generated).
	GeneratedDir string
	// PreviouslyOwned holds the absolute paths agentpack wrote on the last
	// sync. Targets must not overwrite an existing file that is absent from
	// this set — it belongs to the user; warn and skip instead.
	PreviouslyOwned map[string]bool
	// DryRun means: report what would be written, write nothing.
	DryRun bool

	claimed map[string]string
}

// Owns reports whether path may be overwritten: either agentpack wrote it on
// a previous sync, or it does not exist yet.
func (c *Context) Owns(path string, exists bool) bool {
	return !exists || c.PreviouslyOwned[path]
}

// Claim records that packID provides the destination path in this run. The
// first pack to claim a path wins deterministically; a later claim returns
// the winning pack's ID and false, and the caller must skip its write. This
// keeps sync idempotent when two packs ship a same-named file.
func (c *Context) Claim(path, packID string) (owner string, ok bool) {
	if c.claimed == nil {
		c.claimed = map[string]string{}
	}
	if owner, taken := c.claimed[path]; taken {
		return owner, false
	}
	c.claimed[path] = packID
	return packID, true
}

// RetainOwnedUnder returns the previously-owned files inside dir. A target
// that skips a surface to protect foreign files must report these as
// retained, so the syncer neither prunes them nor forgets it owns them.
func (c *Context) RetainOwnedUnder(dir string) []string {
	var kept []string
	prefix := dir + string(filepath.Separator)
	for p := range c.PreviouslyOwned {
		if p == dir || strings.HasPrefix(p, prefix) {
			kept = append(kept, p)
		}
	}
	sort.Strings(kept)
	return kept
}

// Result is what a target produced.
type Result struct {
	// Files are the absolute paths written (or that would be written, under
	// DryRun). They are recorded in the sync state and pruned when a later
	// sync stops producing them.
	Files []string
	// Retained are previously-owned paths the target deliberately left in
	// place without rewriting (e.g. inside a skill directory skipped because
	// a foreign file appeared in it). They stay in the sync state and are
	// excluded from pruning.
	Retained []string
	// Blocks are files that contain an agentpack-managed block after this
	// sync. The syncer tracks them in state and removes the block when a
	// later sync stops producing it (e.g. the target was dropped).
	Blocks []string
	// Warnings are non-fatal issues to surface to the user (e.g. a skill
	// skipped because a foreign file already occupies its destination).
	Warnings []string
}

// Target materializes packs into one AI tool's configuration.
type Target interface {
	// Name is the identifier used in the org manifest's `targets:` list.
	Name() string
	// Apply writes the packs' content into the tool's surfaces.
	Apply(ctx *Context) (*Result, error)
}

var targets = registry.New[Target]("target")

// Register makes a target available by name. Registering the same name twice
// panics: it is a programmer error in wiring, not a runtime condition.
func Register(t Target) { targets.Register(t) }

// Get returns the target registered under name.
func Get(name string) (Target, error) { return targets.Get(name) }

// Names lists registered targets, sorted.
func Names() []string { return targets.Names() }

// WriteGenerated writes a generated markdown file (header + content) into
// ctx.GeneratedDir and returns a Result listing it. Empty content produces
// nothing, so a stale file from a previous sync gets pruned. Single-file
// targets (agentsmd, cursor, a future copilot) share this write path.
func WriteGenerated(ctx *Context, filename, header, content string) (*Result, error) {
	if content == "" {
		return &Result{}, nil
	}
	dest := filepath.Join(ctx.GeneratedDir, filename)
	if !ctx.DryRun {
		if err := os.MkdirAll(ctx.GeneratedDir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(dest, []byte(header+content+"\n"), 0o644); err != nil {
			return nil, err
		}
	}
	return &Result{Files: []string{dest}}, nil
}
