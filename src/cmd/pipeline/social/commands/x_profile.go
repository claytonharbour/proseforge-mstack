package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/social"
	"github.com/spf13/cobra"
)

func NewXProfileCmd() *cobra.Command {
	var socialSvc = social.NewService()
	var project string
	var show bool
	var update bool
	var post string
	var deleteID string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "x-profile",
		Short: "Manage X (Twitter) profile and posts",
		Long:  "Show profile, update profile, post tweets, or delete tweets.",
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
				profile, err := socialSvc.ShowXProfile(project)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(profile)
			} else if update {
				settings := map[string]string{} // TODO: Load from project settings
				err := socialSvc.UpdateXProfile(project, settings)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
				fmt.Fprintf(os.Stderr, "Profile updated successfully\n")
				return nil
			} else if post != "" {
				if dryRun {
					fmt.Fprintf(os.Stderr, "Dry run - no tweet posted\n")
					return nil
				}
				tweetID, err := socialSvc.PostToX(project, post)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return err
				}
				// Output JSON to stdout (pipeable)
				result := map[string]string{"tweet_id": tweetID}
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(result)
			} else if deleteID != "" {
				if dryRun {
					fmt.Fprintf(os.Stderr, "Dry run - tweet not deleted\n")
					return nil
				}
				// Delete not yet implemented in service
				return fmt.Errorf("delete functionality not yet implemented")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")
	cmd.Flags().BoolVar(&show, "show", false, "Show current profile")
	cmd.Flags().BoolVar(&update, "update", false, "Update profile")
	cmd.Flags().StringVar(&post, "post", "", "Post a tweet")
	cmd.Flags().StringVar(&deleteID, "delete", "", "Delete a tweet by ID")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without changes")

	return cmd
}
