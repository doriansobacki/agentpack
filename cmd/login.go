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
		Use:   "login [email]",
		Short: "Create the local profile: who you are and where the org config lives",
		Long: `Create the local profile: who you are and where the org config lives.

With the static identity provider, pass your email:

  agentpack login you@example.com --source <git-url-or-path>

With an IdP-backed provider (e.g. entra), omit the email — your identity
comes from signing in, not from local config:

  agentpack login --source <git-url-or-path>`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadUserConfig()
			if err != nil {
				cfg = &config.UserConfig{}
			}
			if len(args) == 1 {
				cfg.Email = args[0]
			}
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
			if cfg.Email != "" {
				fmt.Printf("Logged in as %s\n", cfg.Email)
			} else {
				fmt.Println("Logged in; your identity will come from the org's identity provider.")
			}
			fmt.Printf("Org config source: %s\n\nRun `agentpack sync` to materialize your packs.\n", cfg.Source)
			return nil
		},
	}
	loginCmd.Flags().StringVarP(&source, "source", "s", "", "org config source: git URL or local directory")
	rootCmd.AddCommand(loginCmd)
}
