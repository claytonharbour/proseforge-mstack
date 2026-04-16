package fb

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
		Short: "Update Facebook Page info",
		Long:  "Update Facebook Page settings.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			updates := map[string]string{} // TODO: Load from project settings
			err := socialSvc.UpdateFacebookPage(project, updates)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}
			fmt.Fprintf(os.Stderr, "Page updated successfully\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")

	return cmd
}
