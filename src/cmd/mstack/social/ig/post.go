package ig

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
	var image string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "post [caption]",
		Short: "Post to Instagram",
		Long:  "Post an image with caption to Instagram.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			caption := args[0]

			if image == "" {
				return fmt.Errorf("--image URL required for posting")
			}

			if dryRun {
				fmt.Fprintf(os.Stderr, "Dry run - no post made\n")
				fmt.Fprintf(os.Stderr, "Would post: %s\n", caption)
				fmt.Fprintf(os.Stderr, "With image: %s\n", image)
				return nil
			}

			mediaID, err := socialSvc.PostToInstagram(project, caption, image)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			result := map[string]string{"media_id": mediaID}
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")
	cmd.Flags().StringVar(&image, "image", "", "Public URL of image to post (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without posting")
	cmd.MarkFlagRequired("image")

	return cmd
}
