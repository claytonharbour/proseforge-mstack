package video

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/spf13/cobra"
)

func NewSplitCmd() *cobra.Command {
	var outputDir string
	var atTimestamps string
	var reencode bool

	cmd := &cobra.Command{
		Use:   "split <video-path>",
		Short: "Split video at timestamps into multiple files",
		Long: `Split a video file at specified timestamps into multiple output files.

Output files are named <video>_001.<ext>, <video>_002.<ext>, etc.
By default uses lossless copy mode (-c copy) for fast splitting.

Timestamp formats supported:
  - Seconds: 30, 90, 150
  - Seconds with decimals: 30.5, 90.25
  - MM:SS: 00:30, 01:30, 02:30
  - HH:MM:SS: 00:00:30, 00:01:30

Examples:
  # Split at 30s, 60s, 90s → creates 4 files
  mstack video split video.webm --at=30,60,90

  # Split using MM:SS format
  mstack video split video.webm --at=00:30,01:00,01:30

  # Split with re-encoding for frame-accurate cuts
  mstack video split video.webm --at=30,60 --reencode

  # Specify output directory
  mstack video split video.webm --at=30,60 --output-dir=./clips`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoPath := args[0]

			if atTimestamps == "" {
				return fmt.Errorf("--at flag is required: specify comma-separated timestamps")
			}

			timestamps, err := video.ParseTimestamps(atTimestamps)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			params := video.SplitParams{
				VideoPath:  videoPath,
				Timestamps: timestamps,
				OutputDir:  outputDir,
				Lossless:   !reencode,
			}

			result, err := video.SplitVideo(params)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		},
	}

	cmd.Flags().StringVar(&atTimestamps, "at", "", "Comma-separated timestamps to split at (required)")
	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "Output directory (default: same as source)")
	cmd.Flags().BoolVar(&reencode, "reencode", false, "Re-encode for frame-accurate cuts (slower)")

	return cmd
}
