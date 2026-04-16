package secrets

import (
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/social"
	"github.com/spf13/cobra"
)

func NewPushCmd() *cobra.Command {
	var socialSvc = social.NewService()
	var project string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push secrets to Bitwarden",
		Long:  "Push .env secrets to Bitwarden vault.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			if dryRun {
				fmt.Fprintf(os.Stderr, "Dry run mode\n")
			}

			err := socialSvc.SyncSecrets(project, "push")
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
