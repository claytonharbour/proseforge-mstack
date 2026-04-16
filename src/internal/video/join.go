package video

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// JoinParams contains options for joining videos
type JoinParams struct {
	VideoPaths []string // Paths to videos to join (in order)
	OutputPath string   // Output file path
	Lossless   bool     // Use -c copy for lossless join (default true)
	AudioSync  bool     // Use TS-based method for audio sync (default true when lossless=false)
}

// JoinInput represents info about an input video
type JoinInput struct {
	Index      int    `json:"index"`
	Path       string `json:"path"`
	DurationMs int    `json:"duration_ms"`
}

// JoinResult contains the result of a video join operation
type JoinResult struct {
	Inputs         []JoinInput `json:"inputs"`
	OutputPath     string      `json:"output_path"`
	TotalDuration  int         `json:"total_duration_ms"`
	OutputFileSize int64       `json:"output_file_size_bytes"`
}

// JoinVideos concatenates multiple video files into one
func JoinVideos(params JoinParams) (*JoinResult, error) {
	if len(params.VideoPaths) < 2 {
		return nil, fmt.Errorf("at least 2 videos required for joining")
	}

	// Validate all input videos exist and get their durations
	inputs := make([]JoinInput, 0, len(params.VideoPaths))
	totalDuration := 0

	for i, videoPath := range params.VideoPaths {
		if _, err := os.Stat(videoPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("video file not found: %s", videoPath)
		}

		duration, err := getVideoDurationMs(videoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get duration for %s: %w", videoPath, err)
		}

		inputs = append(inputs, JoinInput{
			Index:      i + 1,
			Path:       videoPath,
			DurationMs: duration,
		})
		totalDuration += duration
	}

	// Validate output path
	if params.OutputPath == "" {
		return nil, fmt.Errorf("output path is required")
	}

	// Create output directory if needed
	outputDir := filepath.Dir(params.OutputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Use audio sync method when re-encoding (non-lossless) and AudioSync is enabled
	if !params.Lossless && params.AudioSync {
		return joinVideosWithAudioSync(inputs, totalDuration, params)
	}

	// Create temporary concat file
	concatFile, err := os.CreateTemp("", "ffmpeg-concat-*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(concatFile.Name())

	// Write concat file contents
	for _, videoPath := range params.VideoPaths {
		// Get absolute path and escape single quotes for ffmpeg
		absPath, err := filepath.Abs(videoPath)
		if err != nil {
			absPath = videoPath
		}
		// Escape single quotes by replacing ' with '\''
		escapedPath := strings.ReplaceAll(absPath, "'", "'\\''")
		fmt.Fprintf(concatFile, "file '%s'\n", escapedPath)
	}
	concatFile.Close()

	// Build ffmpeg command
	args := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile.Name(),
	}

	if params.Lossless {
		args = append(args, "-c", "copy")
	} else {
		args = append(args, "-c:v", "libx264", "-preset", "fast", "-crf", "18")
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	}

	args = append(args, params.OutputPath)

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to join videos: %w\nOutput: %s", err, string(output))
	}

	// Get output file size
	fileInfo, err := os.Stat(params.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat output file: %w", err)
	}

	return &JoinResult{
		Inputs:         inputs,
		OutputPath:     params.OutputPath,
		TotalDuration:  totalDuration,
		OutputFileSize: fileInfo.Size(),
	}, nil
}

// ParseVideoPaths parses a comma-separated list of video paths
func ParseVideoPaths(input string) ([]string, error) {
	if input == "" {
		return nil, fmt.Errorf("no video paths provided")
	}

	parts := strings.Split(input, ",")
	paths := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		paths = append(paths, part)
	}

	if len(paths) < 2 {
		return nil, fmt.Errorf("at least 2 videos required for joining")
	}

	return paths, nil
}

// joinVideosWithAudioSync uses MPEG-TS intermediate format with audio resampling
// to fix audio sync issues when concatenating videos.
// This method is slower but produces reliable audio sync at segment boundaries.
func joinVideosWithAudioSync(inputs []JoinInput, totalDuration int, params JoinParams) (*JoinResult, error) {
	// Create temp directory for intermediate files
	tempDir, err := os.MkdirTemp("", "ffmpeg-join-ts-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Convert each video to .ts format with audio sync correction
	tsFiles := make([]string, 0, len(params.VideoPaths))
	for i, videoPath := range params.VideoPaths {
		tsPath := filepath.Join(tempDir, fmt.Sprintf("part_%03d.ts", i))
		tsFiles = append(tsFiles, tsPath)

		args := []string{
			"-y",
			"-i", videoPath,
			// Video settings
			"-c:v", "libx264", "-preset", "medium", "-crf", "23",
			"-r", "25", "-pix_fmt", "yuv420p",
			// Audio settings with sync correction
			"-c:a", "aac", "-b:a", "128k", "-ar", "44100", "-ac", "2",
			"-af", "aresample=async=1000",
			// Ensure matching duration
			"-shortest",
			tsPath,
		}

		cmd := exec.Command("ffmpeg", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("failed to convert %s to TS: %w\nOutput: %s", videoPath, err, string(output))
		}
	}

	// Binary concatenate all TS files
	combinedPath := filepath.Join(tempDir, "combined.ts")
	combinedFile, err := os.Create(combinedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create combined file: %w", err)
	}

	for _, tsPath := range tsFiles {
		tsFile, err := os.Open(tsPath)
		if err != nil {
			combinedFile.Close()
			return nil, fmt.Errorf("failed to open %s: %w", tsPath, err)
		}
		_, err = io.Copy(combinedFile, tsFile)
		tsFile.Close()
		if err != nil {
			combinedFile.Close()
			return nil, fmt.Errorf("failed to copy %s: %w", tsPath, err)
		}
	}
	combinedFile.Close()

	// Re-encode to final MP4
	args := []string{
		"-y",
		"-i", combinedPath,
		"-c:v", "libx264", "-preset", "medium", "-crf", "23",
		"-c:a", "aac", "-b:a", "128k",
		params.OutputPath,
	}

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to encode final video: %w\nOutput: %s", err, string(output))
	}

	// Get output file size
	fileInfo, err := os.Stat(params.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat output file: %w", err)
	}

	return &JoinResult{
		Inputs:         inputs,
		OutputPath:     params.OutputPath,
		TotalDuration:  totalDuration,
		OutputFileSize: fileInfo.Size(),
	}, nil
}
