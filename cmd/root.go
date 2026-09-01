// Package cmd implements the agentpack CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/doriansobacki/agentpack/internal/builtins"
	"github.com/spf13/cobra"
)

// Version is stamped by goreleaser/ldflags on release builds.
var Version = "0.1.0-dev"

var rootCmd = &cobra.Command{
	Use:     "agentpack",
	Version: Version,
	Short:   "Distribute AI agent config (rules, skills, agents, memories) across an organization",
	Long: `agentpack is a control plane for AI coding agent configuration.

An organization keeps versioned "packs" of rules, skills, agents, and
memories in a git repository, mapped to groups (org-wide, per team, per
role). Developers log in once; agentpack resolves which packs apply to them
and materializes the content into each AI tool's native surfaces (Claude
Code today; Cursor and Copilot surfaces as they become writable). Re-running
sync keeps everyone on the latest team config and prunes what no longer
applies.`,
}

// Execute runs the CLI.
func Execute() {
	builtins.Register()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
