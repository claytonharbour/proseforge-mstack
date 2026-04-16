package video

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/spf13/cobra"
)

func NewThumbnailCmd() *cobra.Command {
	var outputPath string
	var timestamp string
	var width, height int
	var quality int
	var auto bool

	cmd := &cobra.Command{
		Use:   "thumbnail <video-path>",
		Short: "Extract thumbnail frame from video",
		Long: `Extract a thumbnail frame from video at specified timestamp.

The frame is resized to YouTube's optimal 1280x720 resolution by default.
Supports auto-detection of "interesting" frames using scene change analysis.

Examples:
  # Extract at specific timestamp
  mstack video thumbnail video.webm --timestamp=00:05

  # Auto-detect interesting frame
  mstack video thumbnail video.webm --auto

  # Custom output and size
  mstack video thumbnail video.webm --timestamp=10 --output=thumb.jpg --width=1920 --height=1080

Timestamp formats:
  - Seconds: 10, 10.5
  - MM:SS: 00:10, 1:30
  - HH:MM:SS: 00:01:30`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoPath := args[0]

			// Parse timestamp
			timestampMs := -1 // Default: auto-detect
			if timestamp != "" && !auto {
				var err error
				timestampMs, err = parseThumbnailTimestamp(timestamp)
				if err != nil {
					return fmt.Errorf("invalid timestamp: %w", err)
				}
			}

			params := video.ThumbnailParams{
				VideoPath:   videoPath,
				OutputPath:  outputPath,
				TimestampMs: timestampMs,
				Width:       width,
				Height:      height,
				Quality:     quality,
			}

			result, err := video.ExtractThumbnail(params)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path (default: <video>_thumbnail.jpg)")
	cmd.Flags().StringVarP(&timestamp, "timestamp", "t", "", "Timestamp to extract (default: auto-detect)")
	cmd.Flags().IntVar(&width, "width", 1280, "Output width (default: 1280)")
	cmd.Flags().IntVar(&height, "height", 720, "Output height (default: 720)")
	cmd.Flags().IntVarP(&quality, "quality", "q", 90, "JPEG quality 1-100 (default: 90)")
	cmd.Flags().BoolVar(&auto, "auto", false, "Auto-detect interesting frame using scene analysis")

	return cmd
}

// parseThumbnailTimestamp parses various timestamp formats to milliseconds
func parseThumbnailTimestamp(s string) (int, error) {
	s = strings.TrimSpace(s)

	// Try parsing as float (seconds)
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int(f * 1000), nil
	}

	// Try parsing as MM:SS or HH:MM:SS
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 2: // MM:SS
		minutes, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes: %s", parts[0])
		}
		seconds, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid seconds: %s", parts[1])
		}
		return int((float64(minutes)*60 + seconds) * 1000), nil

	case 3: // HH:MM:SS
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

	default:
		return 0, fmt.Errorf("unrecognized format: %s", s)
	}
}
