package cmd

import (
	"fmt"

	"github.com/doriansobacki/agentpack/internal/config"
	src "github.com/doriansobacki/agentpack/internal/source"
	"github.com/spf13/cobra"
)

func init() {
	var source string
	loginCmd := &cobra.Command{
		Use:   "login <email>",
		Short: "Create the local profile: who you are and where the org config lives",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadUserConfig()
			if err != nil {
				cfg = &config.UserConfig{}
			}
			cfg.Email = args[0]
			if source != "" {
				// A local directory is stored as an absolute path so sync
				// works from any working directory; URLs pass through.
				normalized, _ := src.NormalizeLocal(source)
				cfg.Source = normalized
			}
			if cfg.Source == "" {
				return fmt.Errorf("no org config source set; pass --source <git-url-or-path>")
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Printf("Logged in as %s\nOrg config source: %s\n\nRun `agentpack sync` to materialize your packs.\n", cfg.Email, cfg.Source)
			return nil
		},
	}
	loginCmd.Flags().StringVarP(&source, "source", "s", "", "org config source: git URL or local directory")
	rootCmd.AddCommand(loginCmd)
}
