package video

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/spf13/cobra"
)

func NewTrimCmd() *cobra.Command {
	var outputPath string
	var startMs int
	var endMs int
	var auto bool
	var preview bool
	var reencode bool

	cmd := &cobra.Command{
		Use:   "trim <video-path>",
		Short: "Trim dead air from video start/end",
		Long: `Trim video by removing content from the start and/or end.

Supports manual trimming with exact timestamps or automatic detection
of dead air (silence and/or black frames).

Manual trimming:
  --start=2000   Trim 2 seconds from start
  --end=3000     Trim 3 seconds from end
  --end=-3000    Also trims 3 seconds from end (negative = from end)

Auto detection:
  --auto         Detect silence and black frames automatically
  --preview      Show what would be trimmed without actually trimming

Examples:
  # Manual: trim 2s from start, 3s from end
  mstack video trim video.webm --start=2000 --end=3000

  # Auto-detect and trim dead air
  mstack video trim video.webm --auto

  # Preview auto-detection without trimming
  mstack video trim video.webm --auto --preview

  # Combine auto-detection with manual minimum
  mstack video trim video.webm --auto --start=1000

  # Re-encode for frame-accurate cuts
  mstack video trim video.webm --start=2000 --reencode`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoPath := args[0]

			if !auto && startMs == 0 && endMs == 0 {
				return fmt.Errorf("either --auto or --start/--end flags required")
			}

			params := video.TrimParams{
				VideoPath:   videoPath,
				OutputPath:  outputPath,
				StartMs:     startMs,
				EndMs:       endMs,
				Auto:        auto,
				PreviewOnly: preview,
				Lossless:    !reencode,
			}

			result, err := video.TrimVideo(params)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (default: <video>_trimmed.<ext>)")
	cmd.Flags().IntVar(&startMs, "start", 0, "Milliseconds to trim from start")
	cmd.Flags().IntVar(&endMs, "end", 0, "Milliseconds to trim from end (negative also works)")
	cmd.Flags().BoolVar(&auto, "auto", false, "Auto-detect dead air (silence/black frames)")
	cmd.Flags().BoolVar(&preview, "preview", false, "Preview trim points without trimming")
	cmd.Flags().BoolVar(&reencode, "reencode", false, "Re-encode for frame-accurate cuts")

	return cmd
}
