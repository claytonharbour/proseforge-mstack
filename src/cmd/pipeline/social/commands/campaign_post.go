package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/social"
	"github.com/spf13/cobra"
)

const defaultProject = "proseforge"

func NewCampaignPostCmd() *cobra.Command {
	var socialSvc = social.NewService()
	var project string
	var campaign string
	var list bool
	var postID string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "campaign-post",
		Short: "Manage social media campaigns",
		Long:  "List posts, post campaign items, or retract posts.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = defaultProject
			}
			if campaign == "" {
				campaign = "launch"
			}

			actionCount := 0
			if list {
				actionCount++
			}
			if postID != "" {
				actionCount++
			}

			if actionCount != 1 {
				return fmt.Errorf("exactly one action must be specified: --list or --post")
			}

			if list {
				err := socialSvc.ListPosts(project, campaign)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
				return nil
			} else if postID != "" {
				if dryRun {
					fmt.Fprintf(os.Stderr, "Dry run - no posts made\n")
					return nil
				}
				err := socialSvc.PostCampaignItem(project, campaign, postID)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
				// Output success JSON to stdout (pipeable)
				result := map[string]string{"status": "posted", "post_id": postID}
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(result)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", defaultProject, fmt.Sprintf("Project name (default: %s)", defaultProject))
	cmd.Flags().StringVar(&campaign, "campaign", "launch", "Campaign name (default: launch)")
	cmd.Flags().BoolVar(&list, "list", false, "List all posts in campaign")
	cmd.Flags().StringVar(&postID, "post", "", "Post a draft by UUID")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without changes")

	return cmd
}
