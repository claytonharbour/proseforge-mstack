package video

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	YouTubeThumbnailWidth  = 1280
	YouTubeThumbnailHeight = 720
)

// ThumbnailParams contains options for thumbnail extraction
type ThumbnailParams struct {
	VideoPath   string
	OutputPath  string // Optional: defaults to <video-dir>/<video-name>_thumbnail.jpg
	TimestampMs int    // Specific timestamp in milliseconds (-1 for auto-detect)
	Width       int    // Default: 1280
	Height      int    // Default: 720
	Quality     int    // JPEG quality 1-100, default 90
}

// ThumbnailResult contains the result of thumbnail extraction
type ThumbnailResult struct {
	OutputPath   string `json:"output_path"`
	TimestampMs  int    `json:"timestamp_ms"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int64  `json:"file_size_bytes"`
	AutoDetected bool   `json:"auto_detected"`
}

// ExtractThumbnail extracts a thumbnail frame from video at specified timestamp
func ExtractThumbnail(params ThumbnailParams) (*ThumbnailResult, error) {
	// Validate video exists
	if _, err := os.Stat(params.VideoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("video file not found: %s", params.VideoPath)
	}

	// Set defaults
	width := params.Width
	if width == 0 {
		width = YouTubeThumbnailWidth
	}
	height := params.Height
	if height == 0 {
		height = YouTubeThumbnailHeight
	}
	quality := params.Quality
	if quality == 0 {
		quality = 90
	}

	// Generate output path if not provided
	outputPath := params.OutputPath
	if outputPath == "" {
		videoDir := filepath.Dir(params.VideoPath)
		videoBase := filepath.Base(params.VideoPath)
		videoName := strings.TrimSuffix(videoBase, filepath.Ext(videoBase))
		outputPath = filepath.Join(videoDir, fmt.Sprintf("%s_thumbnail.jpg", videoName))
	}

	// Determine timestamp
	timestampMs := params.TimestampMs
	autoDetected := false

	if timestampMs < 0 {
		// Auto-detect interesting frame
		detected, err := detectInterestingFrame(params.VideoPath)
		if err != nil {
			// Fall back to 10% into video
			duration, dErr := getVideoDurationMs(params.VideoPath)
			if dErr != nil {
				duration = 10000 // Default to 10 seconds if can't get duration
			}
			timestampMs = int(float64(duration) * 0.1) // 10% in
		} else {
			timestampMs = detected
			autoDetected = true
		}
	}

	timestampSec := float64(timestampMs) / 1000.0

	// Build ffmpeg command with scaling
	// scale + pad to maintain aspect ratio with letterbox/pillarbox
	scaleFilter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black",
		width, height, width, height)

	// FFmpeg quality is 1-31 (lower is better), so we convert from 1-100
	ffmpegQuality := (100 - quality) / 3
	if ffmpegQuality < 1 {
		ffmpegQuality = 1
	}
	if ffmpegQuality > 31 {
		ffmpegQuality = 31
	}

	cmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", timestampSec),
		"-i", params.VideoPath,
		"-frames:v", "1",
		"-vf", scaleFilter,
		"-q:v", fmt.Sprintf("%d", ffmpegQuality),
		"-y",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to extract thumbnail: %w\nOutput: %s", err, string(output))
	}

	// Get file size
	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat output file: %w", err)
	}

	return &ThumbnailResult{
		OutputPath:   outputPath,
		TimestampMs:  timestampMs,
		Width:        width,
		Height:       height,
		FileSize:     fileInfo.Size(),
		AutoDetected: autoDetected,
	}, nil
}

// detectInterestingFrame uses ffmpeg scene detection to find a good thumbnail frame
func detectInterestingFrame(videoPath string) (int, error) {
	// Use ffmpeg's scene detection filter to find first scene change
	// This detects frames with significant visual changes
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-vf", "select='gt(scene,0.3)',showinfo",
		"-frames:v", "1",
		"-f", "null",
		"-",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Scene detection failed, try alternate approach
		return detectFirstNonBlackFrame(videoPath)
	}

	// Parse showinfo output for pts_time
	// Look for line like: [Parsed_showinfo_1 @ ...] n:0 pts:123456 pts_time:1.234
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "pts_time:") {
			parts := strings.Split(line, "pts_time:")
			if len(parts) >= 2 {
				timeStr := strings.Fields(parts[1])[0]
				timestamp, err := strconv.ParseFloat(timeStr, 64)
				if err == nil && timestamp > 0 {
					return int(timestamp * 1000), nil
				}
			}
		}
	}

	return 0, fmt.Errorf("no scene changes detected")
}

// detectFirstNonBlackFrame finds the first frame that isn't mostly black
func detectFirstNonBlackFrame(videoPath string) (int, error) {
	// Use blackdetect filter to find black regions, then pick right after
	cmd := exec.Command("ffprobe",
		"-f", "lavfi",
		"-i", fmt.Sprintf("movie=%s,blackdetect=d=0.1:pic_th=0.98", videoPath),
		"-show_entries", "tags=lavfi.black_end",
		"-of", "csv=p=0",
	)

	output, err := cmd.Output()
	if err != nil {
		// Fall back to first few seconds
		return 2000, nil // 2 seconds in
	}

	// Parse black_end from output
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line != "" {
			timestamp, err := strconv.ParseFloat(line, 64)
			if err == nil && timestamp > 0 {
				// Return 500ms after black ends
				return int((timestamp + 0.5) * 1000), nil
			}
		}
	}

	// Default fallback
	return 2000, nil
}

// getVideoDurationMs returns video duration in milliseconds
func getVideoDurationMs(videoPath string) (int, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get duration: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, err
	}

	return int(duration * 1000), nil
}
