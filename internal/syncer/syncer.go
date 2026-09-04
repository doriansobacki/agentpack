// Package syncer orchestrates a sync: fetch the org config, resolve the
// user's identity and package set, materialize every configured target, prune
// files from previous syncs that are no longer produced, and record state.
package syncer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/doriansobacki/agentpack/internal/config"
	"github.com/doriansobacki/agentpack/internal/orgcfg"
	"github.com/doriansobacki/agentpack/internal/resolve"
	"github.com/doriansobacki/agentpack/internal/source"
	"github.com/doriansobacki/agentpack/pkg/identity"
	"github.com/doriansobacki/agentpack/pkg/target"
)

// Report summarizes what a sync did (or, under DryRun, would do).
type Report struct {
	Email    string
	Provider string
	Groups   []string
	Packages []string
	Targets  []string
	DryRun   bool

	Written []string
	// Blocks are files carrying an agentpack-managed block after this sync.
	Blocks []string
	// PrunedBlocks are files whose managed block was removed because no
	// target produces it anymore.
	PrunedBlocks []string
	Pruned       []string
	Warnings     []string
}

// Sync runs one full sync cycle. Real syncs are serialized by a lock file so
// an interactive run and a scheduled run cannot interleave writes; dry runs
// write nothing and take no lock.
func Sync(ctx context.Context, dryRun bool) (*Report, error) {
	if !dryRun {
		release, err := config.AcquireLock()
		if err != nil {
			return nil, err
		}
		defer release()
	}

	cfg, err := config.LoadUserConfig()
	if err != nil {
		return nil, err
	}

	srcDir, err := source.Fetch(cfg.Source, config.CacheDir())
	if err != nil {
		return nil, err
	}
	org, err := orgcfg.Load(srcDir)
	if err != nil {
		return nil, err
	}

	report := &Report{
		Email:    cfg.Email,
		Provider: org.Manifest.ProviderName(),
		Targets:  org.Manifest.TargetNames(),
		DryRun:   dryRun,
	}

	provider, err := identity.Get(report.Provider)
	if err != nil {
		return nil, err
	}
	id, err := provider.Resolve(ctx, identity.Request{
		Email:       cfg.Email,
		Options:     org.Manifest.Identity.Options,
		StaticUsers: org.Manifest.Users,
		Interactive: true,
	})
	if err != nil {
		return nil, err
	}
	report.Groups = id.Groups
	if len(id.Groups) == 0 {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("%s belongs to no groups; only wildcard (*) packages apply", id.Email))
	}

	packageIDs, unknownGroups := resolve.Packages(org.Manifest, id.Groups)
	for _, g := range unknownGroups {
		report.Warnings = append(report.Warnings, fmt.Sprintf("group %q is not defined in %s", g, orgcfg.ManifestFileName))
	}
	report.Packages = packageIDs

	packs, warnings := loadPacks(org, packageIDs)
	report.Warnings = append(report.Warnings, warnings...)

	prevState, err := config.LoadState()
	if err != nil {
		return nil, err
	}
	previouslyOwned := map[string]bool{}
	for _, f := range prevState.Files {
		previouslyOwned[f] = true
	}

	tctx := &target.Context{
		Packs:           packs,
		ClaudeDir:       config.ClaudeDir(),
		GeneratedDir:    config.GeneratedDir(),
		PreviouslyOwned: previouslyOwned,
		DryRun:          dryRun,
	}

	// Resolve every target before applying any, so a typo'd target name in
	// the manifest fails the sync without writing a single file.
	targets := make([]target.Target, 0, len(report.Targets))
	for _, name := range report.Targets {
		t, err := target.Get(name)
		if err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}

	// current = files that belong to agentpack after this sync: everything
	// written plus everything deliberately retained. It becomes the new
	// state and the survivor set for pruning.
	current := map[string]bool{}
	var currentList []string
	keep := func(paths []string) {
		for _, f := range paths {
			if !current[f] {
				current[f] = true
				currentList = append(currentList, f)
			}
		}
	}
	blocks := map[string]bool{}
	var blockList []string

	for _, t := range targets {
		res, err := t.Apply(tctx)
		if err != nil {
			// Files written before the failure must not be forgotten:
			// without state they would be treated as foreign forever. Merge
			// them into the previous state and persist best-effort.
			if !dryRun {
				salvage := &config.State{
					LastSync:      prevState.LastSync,
					Packages:      prevState.Packages,
					Files:         union(prevState.Files, currentList),
					ManagedBlocks: union(prevState.ManagedBlocks, blockList),
				}
				if saveErr := salvage.Save(); saveErr != nil {
					return nil, fmt.Errorf("target %s: %w (and saving partial state failed: %v)", t.Name(), err, saveErr)
				}
			}
			return nil, fmt.Errorf("target %s: %w", t.Name(), err)
		}
		for _, f := range res.Files {
			if !current[f] {
				report.Written = append(report.Written, f)
			}
		}
		keep(res.Files)
		keep(res.Retained)
		for _, b := range res.Blocks {
			if !blocks[b] {
				blocks[b] = true
				blockList = append(blockList, b)
			}
		}
		report.Warnings = append(report.Warnings, res.Warnings...)
	}
	report.Blocks = blockList

	for _, old := range prevState.Files {
		if current[old] {
			continue
		}
		report.Pruned = append(report.Pruned, old)
		if dryRun {
			continue
		}
		if err := os.Remove(old); err != nil && !os.IsNotExist(err) {
			report.Warnings = append(report.Warnings, fmt.Sprintf("pruning %s: %v (will retry next sync)", old, err))
			// Keep the path in state so the prune is retried and the file
			// is still recognized as agentpack-owned.
			keep([]string{old})
			continue
		}
		removeEmptyParents(filepath.Dir(old))
	}

	for _, old := range prevState.ManagedBlocks {
		if blocks[old] {
			continue
		}
		report.PrunedBlocks = append(report.PrunedBlocks, old)
		if dryRun {
			continue
		}
		if err := target.RemoveManagedBlock(old); err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("removing managed block from %s: %v (will retry next sync)", old, err))
			blocks[old] = true
			blockList = append(blockList, old)
		}
	}

	if !dryRun {
		newState := &config.State{
			LastSync:      time.Now().UTC(),
			Packages:      packageIDs,
			Files:         currentList,
			ManagedBlocks: blockList,
		}
		if err := newState.Save(); err != nil {
			return nil, err
		}
	}
	return report, nil
}

// union merges two path lists, preserving order and dropping duplicates.
func union(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, list := range [][]string{a, b} {
		for _, f := range list {
			if !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	return out
}

func loadPacks(org *orgcfg.Org, ids []string) (packs []*target.Pack, warnings []string) {
	for _, id := range ids {
		p, ok := org.Packages[id]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("package %q is mapped but missing from packages/", id))
			continue
		}
		pack := &target.Pack{
			ID:          p.ID,
			Name:        p.Manifest.Name,
			Description: p.Manifest.Description,
		}
		var loadErr error
		load := func(paths []string) []target.File {
			var files []target.File
			for _, path := range paths {
				data, err := os.ReadFile(path)
				if err != nil {
					loadErr = err
					return nil
				}
				files = append(files, target.File{Name: filepath.Base(path), Content: data})
			}
			return files
		}
		pack.Rules = load(p.RuleFiles())
		pack.Memories = load(p.MemoryFiles())
		pack.Agents = load(p.AgentFiles())
		if loadErr != nil {
			warnings = append(warnings, fmt.Sprintf("package %q: %v; skipped", id, loadErr))
			continue
		}
		for _, dir := range p.SkillDirs() {
			pack.Skills = append(pack.Skills, target.SkillDir{Name: filepath.Base(dir), Path: dir})
		}
		packs = append(packs, pack)
	}
	return packs, warnings
}

// removeEmptyParents removes now-empty directories left behind by pruning,
// walking upward until a non-empty directory (or an error) stops it. Removal
// only ever applies to directories that are empty, so this is safe.
func removeEmptyParents(dir string) {
	for {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
