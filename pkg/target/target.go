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
	"fmt"
	"sort"
	"sync"
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
}

// Owns reports whether path may be overwritten: either agentpack wrote it on
// a previous sync, or it does not exist yet.
func (c *Context) Owns(path string, exists bool) bool {
	return !exists || c.PreviouslyOwned[path]
}

// Result is what a target produced.
type Result struct {
	// Files are the absolute paths written (or that would be written, under
	// DryRun). They are recorded in the sync state and pruned when a later
	// sync stops producing them.
	Files []string
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

var (
	mu       sync.RWMutex
	registry = map[string]Target{}
)

// Register makes a target available by name. Registering the same name twice
// panics: it is a programmer error in wiring, not a runtime condition.
func Register(t Target) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[t.Name()]; exists {
		panic(fmt.Sprintf("target: %q registered twice", t.Name()))
	}
	registry[t.Name()] = t
}

// Get returns the target registered under name.
func Get(name string) (Target, error) {
	mu.RLock()
	defer mu.RUnlock()
	t, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("target: unknown target %q (available: %v)", name, Names())
	}
	return t, nil
}

// Names lists registered targets, sorted.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
