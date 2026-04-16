package secrets

import (
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/social"
	"github.com/spf13/cobra"
)

func NewPullCmd() *cobra.Command {
	var socialSvc = social.NewService()
	var project string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull secrets from Bitwarden",
		Long:  "Pull secrets from Bitwarden vault to .env.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			if dryRun {
				fmt.Fprintf(os.Stderr, "Dry run mode\n")
			}

			err := socialSvc.SyncSecrets(project, "pull")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without changes")

	return cmd
}
