package video

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/spf13/cobra"
)

func NewJoinCmd() *cobra.Command {
	var outputPath string
	var reencode bool
	var noAudioSync bool

	cmd := &cobra.Command{
		Use:   "join <video1> <video2> [video3...]",
		Short: "Concatenate multiple videos into one",
		Long: `Concatenate multiple video files into a single output file.

Videos are joined in the order specified. By default uses lossless
copy mode (-c copy) for fast joining. Use --reencode for videos with
different formats or when precise concatenation is needed.

When re-encoding, audio sync correction is enabled by default to fix
choppy audio at segment boundaries. Use --no-audio-sync to disable.

Examples:
  # Join two videos
  mstack video join clip1.mp4 clip2.mp4 --output=final.mp4

  # Join multiple videos
  mstack video join intro.mp4 main.mp4 outro.mp4 -o combined.mp4

  # Join with re-encoding (for mixed formats)
  mstack video join clip1.webm clip2.mp4 --output=final.mp4 --reencode

  # Join with re-encoding but skip audio sync (faster)
  mstack video join clip1.mp4 clip2.mp4 --output=final.mp4 --reencode --no-audio-sync`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputPath == "" {
				return fmt.Errorf("--output flag is required")
			}

			params := video.JoinParams{
				VideoPaths: args,
				OutputPath: outputPath,
				Lossless:   !reencode,
				AudioSync:  !noAudioSync,
			}

			result, err := video.JoinVideos(params)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (required)")
	cmd.Flags().BoolVar(&reencode, "reencode", false, "Re-encode videos (required for mixed formats)")
	cmd.Flags().BoolVar(&noAudioSync, "no-audio-sync", false, "Disable audio sync correction when re-encoding")

	return cmd
}
