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
	Email     string
	Provider  string
	Groups    []string
	Packages  []string
	Targets   []string
	SourceDir string
	DryRun    bool

	Written  []string
	Pruned   []string
	Warnings []string
}

// Sync runs one full sync cycle.
func Sync(ctx context.Context, dryRun bool) (*Report, error) {
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
		Email:     cfg.Email,
		Provider:  org.Manifest.ProviderName(),
		Targets:   org.Manifest.TargetNames(),
		SourceDir: srcDir,
		DryRun:    dryRun,
	}

	provider, err := identity.Get(report.Provider)
	if err != nil {
		return nil, err
	}
	id, err := provider.Resolve(ctx, identity.Request{
		Email:       cfg.Email,
		Options:     org.Manifest.Identity.Options,
		StaticUsers: org.Manifest.Users,
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

	written := map[string]bool{}
	for _, name := range report.Targets {
		t, err := target.Get(name)
		if err != nil {
			return nil, err
		}
		res, err := t.Apply(tctx)
		if err != nil {
			return nil, fmt.Errorf("target %s: %w", name, err)
		}
		for _, f := range res.Files {
			if !written[f] {
				written[f] = true
				report.Written = append(report.Written, f)
			}
		}
		report.Warnings = append(report.Warnings, res.Warnings...)
	}

	for _, old := range prevState.Files {
		if written[old] {
			continue
		}
		report.Pruned = append(report.Pruned, old)
		if dryRun {
			continue
		}
		if err := os.Remove(old); err != nil && !os.IsNotExist(err) {
			report.Warnings = append(report.Warnings, fmt.Sprintf("pruning %s: %v", old, err))
			continue
		}
		removeEmptyParents(filepath.Dir(old))
	}

	if !dryRun {
		newState := &config.State{
			LastSync: time.Now().UTC(),
			Source:   cfg.Source,
			Packages: packageIDs,
			Files:    report.Written,
		}
		if err := newState.Save(); err != nil {
			return nil, err
		}
	}
	return report, nil
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
