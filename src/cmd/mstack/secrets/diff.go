package secrets

import (
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/social"
	"github.com/spf13/cobra"
)

func NewDiffCmd() *cobra.Command {
	var socialSvc = social.NewService()
	var project string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare local and remote secrets",
		Long:  "Compare .env secrets with Bitwarden vault.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			err := socialSvc.SyncSecrets(project, "diff")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")

	return cmd
}
