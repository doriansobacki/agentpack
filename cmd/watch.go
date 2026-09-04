package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/doriansobacki/agentpack/internal/syncer"
	"github.com/doriansobacki/agentpack/internal/synclog"
	"github.com/spf13/cobra"
)

// minWatchInterval keeps watch mode from hammering the org config remote.
const minWatchInterval = 30 * time.Second

func init() {
	var interval time.Duration
	watchCmd := &cobra.Command{
		Use:   "watch",
		Short: "Sync on an interval in the foreground (Ctrl+C to stop)",
		Long: `Runs sync, waits for the interval, and repeats until interrupted.
Consecutive failures back the interval off (doubling, capped at 8x) so an
unreachable org config is not hammered. For unattended machines prefer
` + "`agentpack service install`" + `, which uses the OS scheduler instead of a
foreground process.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if interval < minWatchInterval {
				return fmt.Errorf("interval %s is below the minimum %s", interval, minWatchInterval)
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()

			failures := 0
			for {
				report, err := syncer.Sync(ctx, false)
				line := synclog.Line(report, err)
				if logErr := synclog.Append(report, err); logErr != nil {
					fmt.Fprintln(os.Stderr, "warning: writing sync log:", logErr)
				}
				if err != nil {
					if failures < 3 {
						failures++
					}
					fmt.Fprintln(os.Stderr, line)
				} else {
					failures = 0
					fmt.Println(line)
				}

				delay := interval << failures // 1x, 2x, 4x, 8x
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(delay):
				}
			}
		},
	}
	watchCmd.Flags().DurationVar(&interval, "interval", 5*time.Minute, "time between syncs (e.g. 90s, 5m, 1h)")
	rootCmd.AddCommand(watchCmd)
}
