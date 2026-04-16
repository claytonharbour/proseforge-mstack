package video

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/spf13/cobra"
)

func NewCheckCmd() *cobra.Command {
	var silenceThreshold int
	var silenceMinDuration int
	var driftThreshold int

	cmd := &cobra.Command{
		Use:   "check <video>",
		Short: "Check video for audio sync issues",
		Long: `Analyze a video file for potential audio synchronization problems.

Checks performed:
  - Audio vs video duration drift
  - Silence gap detection (potential dropouts)
  - Audio stream specifications

Examples:
  # Basic check
  mstack video check output.mp4

  # Strict drift threshold (50ms)
  mstack video check output.mp4 --drift-threshold=50

  # Check joined video for silence gaps > 200ms
  mstack video check joined.mp4 --silence-min=200

  # Adjust silence detection sensitivity
  mstack video check video.mp4 --silence-threshold=-40`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params := video.AudioCheckParams{
				VideoPath:            args[0],
				SilenceThresholdDB:   silenceThreshold,
				SilenceMinDurationMs: silenceMinDuration,
				DriftThresholdMs:     driftThreshold,
			}

			result, err := video.CheckAudioSync(params)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		},
	}

	cmd.Flags().IntVar(&silenceThreshold, "silence-threshold", -50, "Silence detection threshold in dB")
	cmd.Flags().IntVar(&silenceMinDuration, "silence-min", 100, "Minimum silence duration to report (ms)")
	cmd.Flags().IntVar(&driftThreshold, "drift-threshold", 100, "Audio/video drift threshold (ms)")

	return cmd
}
