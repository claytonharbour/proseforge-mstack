package x

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
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "post [text]",
		Short: "Post a tweet",
		Long:  "Post a tweet to X (Twitter).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			text := args[0]

			if dryRun {
				fmt.Fprintf(os.Stderr, "Dry run - no tweet posted\n")
				fmt.Fprintf(os.Stderr, "Would post: %s\n", text)
				return nil
			}

			tweetID, err := socialSvc.PostToX(project, text)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			result := map[string]string{"tweet_id": tweetID}
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without posting")

	return cmd
}
