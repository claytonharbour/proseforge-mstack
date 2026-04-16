package x

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewDeleteCmd() *cobra.Command {
	var project string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "delete [tweet-id]",
		Short: "Delete a tweet",
		Long:  "Delete a tweet from X (Twitter) by ID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			tweetID := args[0]

			if dryRun {
				fmt.Fprintf(os.Stderr, "Dry run - tweet not deleted\n")
				fmt.Fprintf(os.Stderr, "Would delete tweet: %s\n", tweetID)
				return nil
			}

			// Delete not yet implemented in service
			return fmt.Errorf("delete functionality not yet implemented for tweet %s", tweetID)
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without deleting")

	return cmd
}
