package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/doriansobacki/agentpack/internal/config"
	"github.com/doriansobacki/agentpack/internal/scheduler"
	"github.com/spf13/cobra"
)

func init() {
	serviceCmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the OS-scheduled background sync",
		Long: `Registers agentpack with the operating system's own per-user job
scheduler (Windows Task Scheduler, macOS launchd, Linux systemd user timers)
so ` + "`agentpack sync`" + ` runs on an interval — no daemon to babysit, no admin
rights required. Scheduled runs log to ` + "`<agentpack home>/logs/sync.log`" + `.`,
	}

	var interval time.Duration
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install (or update) the scheduled sync job",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if interval < scheduler.MinInterval {
				return fmt.Errorf("interval %s is below the minimum %s", interval, scheduler.MinInterval)
			}
			exe, err := os.Executable()
			if err != nil {
				return err
			}
			exe, err = filepath.Abs(exe)
			if err != nil {
				return err
			}
			msg, err := scheduler.Install(scheduler.Config{
				Executable: exe,
				Interval:   interval,
				LogDir:     config.LogsDir(),
			})
			if err != nil {
				return err
			}
			fmt.Println(msg)
			fmt.Printf("Interval: %s. Logs: %s\n", interval, filepath.Join(config.LogsDir(), "sync.log"))
			return nil
		},
	}
	installCmd.Flags().DurationVar(&interval, "interval", 5*time.Minute, "time between scheduled syncs (rounded to the scheduler's granularity)")

	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the scheduled sync job",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			msg, err := scheduler.Uninstall()
			if err != nil {
				return err
			}
			fmt.Println(msg)
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show the scheduled sync job as the OS scheduler sees it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := scheduler.Status()
			if err != nil {
				return err
			}
			fmt.Print(out)
			return nil
		},
	}

	serviceCmd.AddCommand(installCmd, uninstallCmd, statusCmd)
	rootCmd.AddCommand(serviceCmd)
}
