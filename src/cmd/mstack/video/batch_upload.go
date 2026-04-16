package video

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/youtube"
	"github.com/spf13/cobra"
)

func NewBatchUploadCmd() *cobra.Command {
	var (
		dryRun bool
		resume bool
	)

	cmd := &cobra.Command{
		Use:   "batch <manifest.json>",
		Short: "Batch upload videos from manifest file",
		Long: `Upload multiple videos to YouTube from a JSON manifest file.

The manifest is a JSON file containing an array of video upload items.
Progress is saved after each upload for resumability.

Manifest format:
  [
    {
      "file": "/path/to/video1.mp4",
      "title": "Video Title",
      "description": "Video description",
      "tags": ["tag1", "tag2"],
      "privacy": "unlisted",
      "playlist_id": "PLxxxx",
      "status": "pending"
    }
  ]

Status lifecycle: pending → uploading → completed | failed | skipped

Examples:
  # Preview batch upload (dry run)
  mstack video youtube batch manifest.json --dry-run

  # Execute batch upload
  mstack video youtube batch manifest.json

  # Resume failed uploads
  mstack video youtube batch manifest.json --resume`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]
			channelFlag, _ := cmd.Parent().Flags().GetString("channel")
			project, _ := cmd.Flags().GetString("project")

			// Resolve auth config
			authConfig, err := youtube.ResolveAuthConfig(project, channelFlag)
			if err != nil {
				if multiErr, ok := err.(*youtube.MultipleChannelsError); ok {
					return fmt.Errorf("multiple channels configured: %v\nSpecify with --channel flag", multiErr.Channels)
				}
				return fmt.Errorf("failed to resolve credentials: %w", err)
			}
			if authConfig == nil {
				return fmt.Errorf("no Google auth configured. Run 'mstack google auth' first")
			}

			params := youtube.BatchParams{
				ManifestPath: manifestPath,
				DryRun:       dryRun,
				Resume:       resume,
			}

			ctx := context.Background()
			result, err := youtube.BatchUpload(ctx, *authConfig, params)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview quota cost without uploading")
	cmd.Flags().BoolVar(&resume, "resume", false, "Retry failed uploads")

	return cmd
}
