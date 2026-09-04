package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/doriansobacki/agentpack/internal/syncer"
	"github.com/doriansobacki/agentpack/internal/synclog"
	"github.com/spf13/cobra"
)

func init() {
	var dryRun bool
	var scheduled bool
	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch the org config, resolve your packs, and materialize them",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Scheduled runs are headless: one summary line to stdout, the
			// same line appended to the sync log, real exit codes preserved.
			if scheduled {
				report, err := syncer.Sync(cmd.Context(), false)
				if logErr := synclog.Append(report, err); logErr != nil {
					fmt.Fprintln(os.Stderr, "warning: writing sync log:", logErr)
				}
				if err != nil {
					return err
				}
				fmt.Println(synclog.Line(report, nil))
				return nil
			}

			report, err := syncer.Sync(cmd.Context(), dryRun)
			if err != nil {
				return err
			}

			verb := "Synced"
			if report.DryRun {
				verb = "Would sync (dry run)"
			}
			fmt.Printf("%s %s via %s provider\n", verb, report.Email, report.Provider)
			fmt.Printf("  groups:   %s\n", orNone(report.Groups))
			fmt.Printf("  packages: %s\n", orNone(report.Packages))
			fmt.Printf("  targets:  %s\n", strings.Join(report.Targets, ", "))
			fmt.Printf("  written:  %d file(s)\n", len(report.Written))
			for _, f := range report.Written {
				fmt.Printf("    %s\n", f)
			}
			for _, b := range report.Blocks {
				fmt.Printf("  managed block: %s\n", b)
			}
			for _, b := range report.PrunedBlocks {
				fmt.Printf("  removed stale managed block: %s\n", b)
			}
			if len(report.Pruned) > 0 {
				fmt.Printf("  pruned:   %d file(s) no longer part of your packs\n", len(report.Pruned))
				for _, f := range report.Pruned {
					fmt.Printf("    %s\n", f)
				}
			}
			for _, w := range report.Warnings {
				fmt.Printf("  warning:  %s\n", w)
			}
			return nil
		},
	}
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would change without writing anything")
	syncCmd.Flags().BoolVar(&scheduled, "scheduled", false, "headless mode used by the OS scheduler: one summary line, appended to the sync log")
	rootCmd.AddCommand(syncCmd)
}

func orNone(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}
