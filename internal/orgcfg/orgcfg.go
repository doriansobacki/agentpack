// Package orgcfg loads an organization's agentpack config repository:
// the agentpack.yaml manifest plus the packages/ directory.
package orgcfg

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ManifestFileName is the org manifest at the root of a config repo.
const ManifestFileName = "agentpack.yaml"

// Manifest maps identity to packages. Groups are flat names; conventionally
// hierarchical ones use a slash ("team-a/backend"). The special group "*"
// applies to everyone.
type Manifest struct {
	// Identity selects and configures the identity provider.
	Identity IdentityConfig `yaml:"identity"`
	// Targets lists which AI-tool targets to materialize. Defaults to
	// ["claude", "agentsmd"] when omitted.
	Targets []string `yaml:"targets"`
	// Groups maps a group name to the package IDs its members receive.
	Groups map[string][]string `yaml:"groups"`
	// Users maps an email to the groups the user belongs to. This is the
	// built-in static identity provider; IdP-backed providers can replace it.
	Users map[string][]string `yaml:"users"`
}

// IdentityConfig selects the identity provider; unrecognized keys are passed
// to the provider as options.
type IdentityConfig struct {
	// Provider names a registered identity provider. Defaults to "static".
	Provider string `yaml:"provider"`
	// Options carries any additional provider-specific keys verbatim.
	Options map[string]any `yaml:",inline"`
}

// ProviderName returns the configured provider, defaulting to "static".
func (m *Manifest) ProviderName() string {
	if m.Identity.Provider == "" {
		return "static"
	}
	return m.Identity.Provider
}

// TargetNames returns the configured targets, defaulting to claude+agentsmd.
func (m *Manifest) TargetNames() []string {
	if len(m.Targets) == 0 {
		return []string{"claude", "agentsmd"}
	}
	return m.Targets
}

// PackageManifest is the optional package.yaml inside a package directory.
type PackageManifest struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// Package is one distributable bundle of rules/skills/agents/memories.
type Package struct {
	ID       string
	Dir      string
	Manifest PackageManifest
}

// Org is a fully loaded org config repository.
type Org struct {
	Root     string
	Manifest *Manifest
	Packages map[string]*Package
}

// Load reads the manifest and discovers all packages under root/packages.
func Load(root string) (*Org, error) {
	manifestPath := filepath.Join(root, ManifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading org manifest %s: %w", manifestPath, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", manifestPath, err)
	}

	packages := map[string]*Package{}
	packagesDir := filepath.Join(root, "packages")
	entries, err := os.ReadDir(packagesDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading %s: %w", packagesDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pkg := &Package{
			ID:  e.Name(),
			Dir: filepath.Join(packagesDir, e.Name()),
		}
		pkg.Manifest.Name = pkg.ID
		pmPath := filepath.Join(pkg.Dir, "package.yaml")
		if pmData, err := os.ReadFile(pmPath); err == nil {
			if err := yaml.Unmarshal(pmData, &pkg.Manifest); err != nil {
				return nil, fmt.Errorf("parsing %s: %w", pmPath, err)
			}
			if pkg.Manifest.Name == "" {
				pkg.Manifest.Name = pkg.ID
			}
		}
		packages[pkg.ID] = pkg
	}

	return &Org{Root: root, Manifest: &m, Packages: packages}, nil
}

// RuleFiles returns the package's rules/*.md, sorted by name.
func (p *Package) RuleFiles() []string { return mdFiles(filepath.Join(p.Dir, "rules")) }

// MemoryFiles returns the package's memories/*.md, sorted by name.
func (p *Package) MemoryFiles() []string { return mdFiles(filepath.Join(p.Dir, "memories")) }

// AgentFiles returns the package's agents/*.md, sorted by name.
func (p *Package) AgentFiles() []string { return mdFiles(filepath.Join(p.Dir, "agents")) }

// SkillDirs returns subdirectories of skills/ that contain a SKILL.md.
func (p *Package) SkillDirs() []string {
	skillsDir := filepath.Join(p.Dir, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(skillsDir, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	return dirs
}

func mdFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	return files
}
