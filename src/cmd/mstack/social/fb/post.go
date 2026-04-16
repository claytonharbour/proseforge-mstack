package fb

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
	var link string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "post [text]",
		Short: "Post to Facebook Page",
		Long:  "Post a message to the Facebook Page.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			text := args[0]

			if dryRun {
				fmt.Fprintf(os.Stderr, "Dry run - no post made\n")
				fmt.Fprintf(os.Stderr, "Would post: %s\n", text)
				if link != "" {
					fmt.Fprintf(os.Stderr, "With link: %s\n", link)
				}
				return nil
			}

			postID, err := socialSvc.PostToFacebook(project, text)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			result := map[string]string{"post_id": postID}
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")
	cmd.Flags().StringVar(&link, "link", "", "Link to include with post")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without posting")

	return cmd
}
