package fb

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewDeleteCmd() *cobra.Command {
	var project string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "delete [post-id]",
		Short: "Delete a Facebook post",
		Long:  "Delete a post from the Facebook Page by ID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			postID := args[0]

			if dryRun {
				fmt.Fprintf(os.Stderr, "Dry run - post not deleted\n")
				fmt.Fprintf(os.Stderr, "Would delete post: %s\n", postID)
				return nil
			}

			// Delete not yet implemented in service
			return fmt.Errorf("delete functionality not yet implemented for post %s", postID)
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without deleting")

	return cmd
}
