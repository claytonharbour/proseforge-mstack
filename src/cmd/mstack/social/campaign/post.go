package campaign

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/social"
	"github.com/spf13/cobra"
)

func NewPostCmd() *cobra.Command {
	var socialSvc = social.NewService()
	var project string
	var campaignName string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "post [uuid]",
		Short: "Post a campaign item",
		Long:  "Post a draft campaign item to all platforms.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}
			if campaignName == "" {
				campaignName = "launch"
			}

			postID := args[0]

			if dryRun {
				fmt.Fprintf(os.Stderr, "Dry run - no posts made\n")
				fmt.Fprintf(os.Stderr, "Would post campaign item: %s\n", postID)
				return nil
			}

			err := socialSvc.PostCampaignItem(project, campaignName, postID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			result := map[string]string{"status": "posted", "post_id": postID}
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")
	cmd.Flags().StringVar(&campaignName, "campaign", "launch", "Campaign name (default: launch)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without posting")

	return cmd
}
