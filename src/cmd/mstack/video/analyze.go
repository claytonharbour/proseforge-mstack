package video

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/spf13/cobra"
)

func NewAnalyzeCmd() *cobra.Command {
	var videoSvc = video.NewService()
	var narrationPath string
	var audioDir string

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze audio segment timing for overlaps",
		Long:  "Parses narration.md and analyzes audio files to detect timing overlaps and tight fits.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve to absolute paths
			var err error
			narrationPath, err = filepath.Abs(narrationPath)
			if err != nil {
				return fmt.Errorf("invalid narration path: %w", err)
			}
			audioDir, err = filepath.Abs(audioDir)
			if err != nil {
				return fmt.Errorf("invalid audio-dir path: %w", err)
			}

			if _, err := os.Stat(narrationPath); os.IsNotExist(err) {
				return fmt.Errorf("narration file not found: %s", narrationPath)
			}

			if _, err := os.Stat(audioDir); os.IsNotExist(err) {
				return fmt.Errorf("audio directory not found: %s — run 'mstack video tts' first", audioDir)
			}

			// Parse narration.md internally
			segments, err := videoSvc.ParseNarrationMD(narrationPath)
			if err != nil {
				return fmt.Errorf("parsing narration: %w", err)
			}

			// Analyze with parsed segments
			results, err := videoSvc.AnalyzeOverlapWithSegments(segments, audioDir)
			if err != nil {
				return err
			}

			// Output JSON to stdout (pipeable)
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(results)
		},
	}

	cmd.Flags().StringVar(&narrationPath, "narration", "", "Path to narration.md file (required)")
	cmd.Flags().StringVar(&audioDir, "audio-dir", "", "Path to audio directory (required)")

	cmd.MarkFlagRequired("narration")
	cmd.MarkFlagRequired("audio-dir")

	return cmd
}
