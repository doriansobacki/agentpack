package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/doriansobacki/agentpack/internal/orgcfg"
	"github.com/spf13/cobra"
)

func init() {
	initCmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold a new org config repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			manifest := filepath.Join(dir, orgcfg.ManifestFileName)
			if _, err := os.Stat(manifest); err == nil {
				return fmt.Errorf("%s already exists; refusing to overwrite", manifest)
			}
			for rel, content := range scaffold {
				path := filepath.Join(dir, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					return err
				}
			}
			fmt.Printf("Scaffolded org config in %s\n\nNext steps:\n", dir)
			fmt.Println("  1. Edit agentpack.yaml: define your groups and users")
			fmt.Println("  2. Put real content into packages/")
			fmt.Println("  3. Push the repo and have everyone run:")
			fmt.Println("       agentpack login <email> --source <repo-url>")
			fmt.Println("       agentpack sync")
			return nil
		},
	}
	rootCmd.AddCommand(initCmd)
}

var scaffold = map[string]string{
	"agentpack.yaml": `# agentpack org manifest.
# Groups map identity to packs; the special group "*" applies to everyone.
# Hierarchy is expressed by giving users several groups (org -> team -> role).

identity:
  provider: static # group membership from the users: map below

targets: [claude, agentsmd]

groups:
  "*": [org-baseline]
  team-a: [team-a-core]
  team-a/backend: [dotnet]
  team-a/frontend: [react]

users:
  you@example.com: [team-a, team-a/backend]
`,

	"packages/org-baseline/package.yaml": `name: Org Baseline
description: Rules that apply to everyone in the organization.
`,
	"packages/org-baseline/rules/coding-standards.md": `## Coding standards

- Every PR references a ticket number in its title.
- Code is reviewed locally before a PR is opened.
- Prefer small, focused PRs over large ones.
`,
	"packages/org-baseline/memories/glossary.md": `## Glossary

- **Pack**: a versioned bundle of AI agent configuration.
- Add organization-specific vocabulary here so agents use your terms.
`,

	"packages/team-a-core/package.yaml": `name: Team A Core
description: Conventions specific to Team A, independent of stack.
`,
	"packages/team-a-core/rules/team-conventions.md": `## Team A conventions

- Feature branches are named ` + "`<ticket>-<short-description>`" + `.
- Post in the team channel when a PR is ready for review.
`,

	"packages/dotnet/package.yaml": `name: .NET
description: Backend conventions for .NET services.
`,
	"packages/dotnet/rules/dotnet.md": `## .NET conventions

- Target the current LTS version unless the repo states otherwise.
- Use xUnit for tests; one test project per production project.
- Nullable reference types are enabled everywhere; do not suppress warnings.
`,
	"packages/dotnet/skills/dotnet-testing/SKILL.md": `---
name: dotnet-testing
description: How to structure, name, and run tests in this organization's .NET services. Use when writing or modifying tests in a .NET repository.
---

# .NET testing

- Name tests ` + "`Method_Scenario_ExpectedOutcome`" + `.
- Arrange/Act/Assert with a blank line between sections.
- Integration tests live in ` + "`*.IntegrationTests`" + ` projects and must be runnable with ` + "`dotnet test`" + ` without external setup.
`,
	"packages/dotnet/agents/dotnet-reviewer.md": `---
name: dotnet-reviewer
description: Reviews .NET changes against the organization's backend conventions. Use proactively after modifying C# code.
---

You are a code reviewer for .NET services. Check changes against the
organization's .NET conventions (test naming, nullability, project layout)
and report concrete violations with file and line references.
`,

	"packages/react/package.yaml": `name: React
description: Frontend conventions for React applications.
`,
	"packages/react/rules/react.md": `## React conventions

- Function components and hooks only; no class components.
- Co-locate component, styles, and tests in the component's folder.
- State that crosses more than two components goes into the shared store.
`,
}
