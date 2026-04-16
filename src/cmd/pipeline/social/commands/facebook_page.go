package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/social"
	"github.com/spf13/cobra"
)

func NewFacebookPageCmd() *cobra.Command {
	var socialSvc = social.NewService()
	var project string
	var show bool
	var update bool
	var post string
	var deleteID string
	var link string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "facebook-page",
		Short: "Manage Facebook Page and posts",
		Long:  "Show page info, update page info, post messages, or delete posts.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			actionCount := 0
			if show {
				actionCount++
			}
			if update {
				actionCount++
			}
			if post != "" {
				actionCount++
			}
			if deleteID != "" {
				actionCount++
			}

			if actionCount != 1 {
				return fmt.Errorf("exactly one action must be specified: --show, --update, --post, or --delete")
			}

			if show {
				pageInfo, err := socialSvc.ShowFacebookPage(project)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(pageInfo)
			} else if update {
				updates := map[string]string{} // TODO: Load from project settings
				err := socialSvc.UpdateFacebookPage(project, updates)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
				fmt.Fprintf(os.Stderr, "Page updated successfully\n")
				return nil
			} else if post != "" {
				if dryRun {
					fmt.Fprintf(os.Stderr, "Dry run - no post made\n")
					return nil
				}
				postID, err := socialSvc.PostToFacebook(project, post)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
				// Output JSON to stdout (pipeable)
				result := map[string]string{"post_id": postID}
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(result)
			} else if deleteID != "" {
				if dryRun {
					fmt.Fprintf(os.Stderr, "Dry run - post not deleted\n")
					return nil
				}
				// Delete not yet implemented in service
				return fmt.Errorf("delete functionality not yet implemented")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")
	cmd.Flags().BoolVar(&show, "show", false, "Show current page info")
	cmd.Flags().BoolVar(&update, "update", false, "Update page info")
	cmd.Flags().StringVar(&post, "post", "", "Post a message to the page")
	cmd.Flags().StringVar(&deleteID, "delete", "", "Delete a post by ID")
	cmd.Flags().StringVar(&link, "link", "", "Link to include with post")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without changes")

	return cmd
}
