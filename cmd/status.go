package cmd

import (
	"errors"
	"fmt"

	"github.com/doriansobacki/agentpack/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show the current profile and what the last sync produced",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadUserConfig()
			if errors.Is(err, config.ErrNotLoggedIn) {
				fmt.Println("Not logged in. Run `agentpack login <email> --source <git-url-or-path>`.")
				return nil
			}
			if err != nil {
				return err
			}
			fmt.Printf("Email:  %s\nSource: %s\n", cfg.Email, cfg.Source)

			state, err := config.LoadState()
			if err != nil {
				return err
			}
			if state.LastSync.IsZero() {
				fmt.Println("Last sync: never (run `agentpack sync`)")
				return nil
			}
			fmt.Printf("Last sync: %s\n", state.LastSync.Local().Format("2006-01-02 15:04:05"))
			fmt.Printf("Packages:  %s\n", orNone(state.Packages))
			fmt.Printf("Files:     %d managed file(s)\n", len(state.Files))
			return nil
		},
	}
	rootCmd.AddCommand(statusCmd)
}
