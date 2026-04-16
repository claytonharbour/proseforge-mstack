package video

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// SplitParams contains options for splitting a video
type SplitParams struct {
	VideoPath  string   // Path to source video
	Timestamps []int    // Split points in milliseconds
	OutputDir  string   // Output directory (defaults to video's directory)
	Lossless   bool     // Use -c copy for lossless split (default true)
}

// SplitSegment represents one output segment from a split operation
type SplitSegment struct {
	Index    int    `json:"index"`
	Path     string `json:"path"`
	StartMs  int    `json:"start_ms"`
	EndMs    int    `json:"end_ms"`
	Duration int    `json:"duration_ms"`
}

// SplitResult contains the result of a video split operation
type SplitResult struct {
	SourceVideo   string         `json:"source_video"`
	TotalDuration int            `json:"total_duration_ms"`
	SplitPoints   []int          `json:"split_points_ms"`
	Segments      []SplitSegment `json:"segments"`
}

// SplitVideo splits a video at specified timestamps into multiple files
func SplitVideo(params SplitParams) (*SplitResult, error) {
	// Validate video exists
	if _, err := os.Stat(params.VideoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("video file not found: %s", params.VideoPath)
	}

	// Get video duration
	totalDuration, err := getVideoDurationMs(params.VideoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video duration: %w", err)
	}

	// Sort and validate timestamps
	timestamps := make([]int, len(params.Timestamps))
	copy(timestamps, params.Timestamps)
	sortInts(timestamps)

	for _, ts := range timestamps {
		if ts <= 0 {
			return nil, fmt.Errorf("invalid timestamp %d: must be positive", ts)
		}
		if ts >= totalDuration {
			return nil, fmt.Errorf("timestamp %d exceeds video duration %d ms", ts, totalDuration)
		}
	}

	// Check for duplicates
	for i := 1; i < len(timestamps); i++ {
		if timestamps[i] == timestamps[i-1] {
			return nil, fmt.Errorf("duplicate timestamp: %d", timestamps[i])
		}
	}

	// Determine output directory
	outputDir := params.OutputDir
	if outputDir == "" {
		outputDir = filepath.Dir(params.VideoPath)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate output filename base
	videoBase := filepath.Base(params.VideoPath)
	videoExt := filepath.Ext(videoBase)
	videoName := strings.TrimSuffix(videoBase, videoExt)

	// Create segment boundaries: [0, ts1, ts2, ..., duration]
	boundaries := make([]int, 0, len(timestamps)+2)
	boundaries = append(boundaries, 0)
	boundaries = append(boundaries, timestamps...)
	boundaries = append(boundaries, totalDuration)

	segments := make([]SplitSegment, 0, len(boundaries)-1)

	for i := 0; i < len(boundaries)-1; i++ {
		startMs := boundaries[i]
		endMs := boundaries[i+1]
		duration := endMs - startMs

		outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_%03d%s", videoName, i+1, videoExt))

		// Build ffmpeg command
		args := []string{
			"-y",
			"-ss", msToTimestamp(startMs),
			"-i", params.VideoPath,
			"-t", msToTimestamp(duration),
		}

		// Use copy codecs for lossless or re-encode for accuracy
		if params.Lossless {
			args = append(args, "-c", "copy")
		} else {
			args = append(args, "-c:v", "libx264", "-preset", "fast", "-crf", "18")
			args = append(args, "-c:a", "aac", "-b:a", "192k")
		}

		// Avoid negative timestamps
		args = append(args, "-avoid_negative_ts", "make_zero")
		args = append(args, outputPath)

		cmd := exec.Command("ffmpeg", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("failed to extract segment %d: %w\nOutput: %s", i+1, err, string(output))
		}

		segments = append(segments, SplitSegment{
			Index:    i + 1,
			Path:     outputPath,
			StartMs:  startMs,
			EndMs:    endMs,
			Duration: duration,
		})
	}

	return &SplitResult{
		SourceVideo:   params.VideoPath,
		TotalDuration: totalDuration,
		SplitPoints:   timestamps,
		Segments:      segments,
	}, nil
}

// ParseTimestamps parses a comma-separated list of timestamps into milliseconds
func ParseTimestamps(input string) ([]int, error) {
	if input == "" {
		return nil, fmt.Errorf("no timestamps provided")
	}

	parts := strings.Split(input, ",")
	timestamps := make([]int, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		ms, err := parseTimestampToMs(part)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp '%s': %w", part, err)
		}
		timestamps = append(timestamps, ms)
	}

	if len(timestamps) == 0 {
		return nil, fmt.Errorf("no valid timestamps found")
	}

	return timestamps, nil
}

// parseTimestampToMs parses various timestamp formats to milliseconds
func parseTimestampToMs(s string) (int, error) {
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

// msToTimestamp converts milliseconds to HH:MM:SS.mmm format for ffmpeg
func msToTimestamp(ms int) string {
	totalSeconds := float64(ms) / 1000.0
	hours := int(totalSeconds) / 3600
	minutes := (int(totalSeconds) % 3600) / 60
	seconds := totalSeconds - float64(hours*3600) - float64(minutes*60)
	return fmt.Sprintf("%02d:%02d:%06.3f", hours, minutes, seconds)
}

// sortInts sorts a slice of ints in ascending order (simple bubble sort for small slices)
func sortInts(arr []int) {
	for i := 0; i < len(arr)-1; i++ {
		for j := 0; j < len(arr)-i-1; j++ {
			if arr[j] > arr[j+1] {
				arr[j], arr[j+1] = arr[j+1], arr[j]
			}
		}
	}
}
