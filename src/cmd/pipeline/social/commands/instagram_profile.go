package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/social"
	"github.com/spf13/cobra"
)

func NewInstagramProfileCmd() *cobra.Command {
	var socialSvc = social.NewService()
	var project string
	var show bool
	var post string
	var image string
	var deleteID string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "instagram-profile",
		Short: "Manage Instagram profile and posts",
		Long:  "Show profile info, post images, or delete posts.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			actionCount := 0
			if show {
				actionCount++
			}
			if post != "" {
				actionCount++
			}
			if deleteID != "" {
				actionCount++
			}

			if actionCount != 1 {
				return fmt.Errorf("exactly one action must be specified: --show, --post, or --delete")
			}

			if show {
				profile, err := socialSvc.ShowInstagramProfile(project)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(profile)
			} else if post != "" {
				if image == "" {
					return fmt.Errorf("--image URL required for posting")
				}
				if dryRun {
					fmt.Fprintf(os.Stderr, "Dry run - no post made\n")
					return nil
				}
				mediaID, err := socialSvc.PostToInstagram(project, post, image)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
				// Output JSON to stdout (pipeable)
				result := map[string]string{"media_id": mediaID}
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
	cmd.Flags().BoolVar(&show, "show", false, "Show profile info")
	cmd.Flags().StringVar(&post, "post", "", "Post an image with caption")
	cmd.Flags().StringVar(&image, "image", "", "Public URL of image to post")
	cmd.Flags().StringVar(&deleteID, "delete", "", "Delete a post by media ID")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without posting")

	return cmd
}
