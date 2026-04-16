package video

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/claytonharbour/proseforge-mstack/src/internal/validation"
	"github.com/spf13/cobra"
)

func NewExtractFrameCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "extract-frame <video-path> <timestamp>",
		Short: "Extract a single frame from a video at a specific timestamp",
		Long: `Extract a single frame from a video at a specific timestamp.

Timestamp can be specified in multiple formats:
  - Seconds: 20, 20.5
  - MM:SS: 00:20, 1:30
  - HH:MM:SS: 00:00:20, 01:30:45

Examples:
  mstack video extract-frame video.mp4 20
  mstack video extract-frame video.mp4 00:20
  mstack video extract-frame video.mp4 00:00:20 --output frame.png`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoPath := args[0]
			timestampStr := args[1]

			// Parse timestamp to milliseconds
			timestampMs, err := parseTimestamp(timestampStr)
			if err != nil {
				return fmt.Errorf("invalid timestamp format: %w", err)
			}

			// Generate output path if not provided
			if outputPath == "" {
				videoDir := filepath.Dir(videoPath)
				videoBase := filepath.Base(videoPath)
				videoName := strings.TrimSuffix(videoBase, filepath.Ext(videoBase))
				timestampLabel := formatTimestampForFilename(timestampMs)
				outputPath = filepath.Join(videoDir, fmt.Sprintf("%s_%s.png", videoName, timestampLabel))
			}

			// Extract frame
			if err := validation.ExtractSingleFrame(videoPath, timestampMs, outputPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			fmt.Printf("Frame extracted to: %s\n", outputPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path for extracted frame (default: <video-name>_<timestamp>.png)")

	return cmd
}

// parseTimestamp parses various timestamp formats and returns milliseconds
func parseTimestamp(timestampStr string) (int, error) {
	// Try MM:SS or HH:MM:SS format
	if strings.Contains(timestampStr, ":") {
		parts := strings.Split(timestampStr, ":")
		if len(parts) == 2 {
			// MM:SS
			minutes, err := strconv.Atoi(parts[0])
			if err != nil {
				return 0, fmt.Errorf("invalid minutes: %s", parts[0])
			}
			seconds, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				return 0, fmt.Errorf("invalid seconds: %s", parts[1])
			}
			return int((float64(minutes)*60 + seconds) * 1000), nil
		} else if len(parts) == 3 {
			// HH:MM:SS
			hours, err := strconv.Atoi(parts[0])
			if err != nil {
				return 0, fmt.Errorf("invalid hours: %s", parts[0])
			}
			minutes, err := strconv.Atoi(parts[1])
			if err != nil {
				return 0, fmt.Errorf("invalid minutes: %s", parts[1])
			}
			seconds, err := strconv.ParseFloat(parts[2], 64)
			if err != nil {
				return 0, fmt.Errorf("invalid seconds: %s", parts[2])
			}
			return int((float64(hours)*3600 + float64(minutes)*60 + seconds) * 1000), nil
		}
		return 0, fmt.Errorf("invalid timestamp format: %s", timestampStr)
	}

	// Try as seconds (float)
	seconds, err := strconv.ParseFloat(timestampStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp format: %s", timestampStr)
	}
	return int(seconds * 1000), nil
}

// formatTimestampForFilename formats timestamp in milliseconds to a filename-friendly format
func formatTimestampForFilename(timestampMs int) string {
	totalSeconds := timestampMs / 1000
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%02d-%02d-%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d-%02d", minutes, seconds)
}
