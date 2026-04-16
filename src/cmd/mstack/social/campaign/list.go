package campaign

import (
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/social"
	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	var socialSvc = social.NewService()
	var project string
	var campaignName string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List campaign posts",
		Long:  "List all posts in a campaign.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}
			if campaignName == "" {
				campaignName = "launch"
			}

			err := socialSvc.ListPosts(project, campaignName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")
	cmd.Flags().StringVar(&campaignName, "campaign", "launch", "Campaign name (default: launch)")

	return cmd
}
