package x

import (
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/social"
	"github.com/spf13/cobra"
)

func NewUpdateCmd() *cobra.Command {
	var socialSvc = social.NewService()
	var project string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update X profile",
		Long:  "Update X (Twitter) profile settings.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			settings := map[string]string{} // TODO: Load from project settings
			err := socialSvc.UpdateXProfile(project, settings)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}
			fmt.Fprintf(os.Stderr, "Profile updated successfully\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")

	return cmd
}
