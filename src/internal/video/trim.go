package video

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// TrimParams contains options for trimming a video
type TrimParams struct {
	VideoPath    string // Path to source video
	OutputPath   string // Output path (optional, defaults to <video>_trimmed.<ext>)
	StartMs      int    // Milliseconds to trim from start (0 = no trim)
	EndMs        int    // Milliseconds to trim from end (0 = no trim, negative = from end)
	Auto         bool   // Auto-detect dead air
	PreviewOnly  bool   // Just detect trim points, don't actually trim
	Lossless     bool   // Use -c copy for lossless trim
	// Auto-detection thresholds
	SilenceThreshold float64 // dB threshold for silence (default -50)
	SilenceDuration  float64 // Minimum silence duration in seconds (default 0.5)
	BlackThreshold   float64 // Pixel value threshold for black (0-1, default 0.1)
	BlackDuration    float64 // Minimum black duration in seconds (default 0.5)
}

// TrimDetection contains auto-detected trim points
type TrimDetection struct {
	StartTrimMs     int  `json:"start_trim_ms"`
	EndTrimMs       int  `json:"end_trim_ms"`
	SilenceDetected bool `json:"silence_detected"`
	BlackDetected   bool `json:"black_detected"`
}

// TrimResult contains the result of a video trim operation
type TrimResult struct {
	SourceVideo      string         `json:"source_video"`
	OutputPath       string         `json:"output_path,omitempty"`
	OriginalDuration int            `json:"original_duration_ms"`
	TrimmedDuration  int            `json:"trimmed_duration_ms"`
	StartTrimmed     int            `json:"start_trimmed_ms"`
	EndTrimmed       int            `json:"end_trimmed_ms"`
	Detection        *TrimDetection `json:"detection,omitempty"`
	PreviewOnly      bool           `json:"preview_only"`
}

// TrimVideo trims a video's start and/or end
func TrimVideo(params TrimParams) (*TrimResult, error) {
	// Validate video exists
	if _, err := os.Stat(params.VideoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("video file not found: %s", params.VideoPath)
	}

	// Get video duration
	totalDuration, err := getVideoDurationMs(params.VideoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video duration: %w", err)
	}

	startTrimMs := params.StartMs
	endTrimMs := params.EndMs
	var detection *TrimDetection

	// Auto-detect dead air if requested
	if params.Auto {
		detection, err = detectDeadAir(params)
		if err != nil {
			// Non-fatal, just use manual values
			detection = &TrimDetection{}
		}
		if detection.StartTrimMs > startTrimMs {
			startTrimMs = detection.StartTrimMs
		}
		if detection.EndTrimMs > endTrimMs {
			endTrimMs = detection.EndTrimMs
		}
	}

	// Handle negative endMs (trim from end)
	if endTrimMs < 0 {
		endTrimMs = -endTrimMs
	}

	// Validate trim amounts don't exceed video
	if startTrimMs+endTrimMs >= totalDuration {
		return nil, fmt.Errorf("trim amounts (%d + %d ms) exceed video duration (%d ms)", startTrimMs, endTrimMs, totalDuration)
	}

	trimmedDuration := totalDuration - startTrimMs - endTrimMs

	result := &TrimResult{
		SourceVideo:      params.VideoPath,
		OriginalDuration: totalDuration,
		TrimmedDuration:  trimmedDuration,
		StartTrimmed:     startTrimMs,
		EndTrimmed:       endTrimMs,
		Detection:        detection,
		PreviewOnly:      params.PreviewOnly,
	}

	// If preview only, return without trimming
	if params.PreviewOnly {
		return result, nil
	}

	// Determine output path
	outputPath := params.OutputPath
	if outputPath == "" {
		videoDir := filepath.Dir(params.VideoPath)
		videoBase := filepath.Base(params.VideoPath)
		videoExt := filepath.Ext(videoBase)
		videoName := strings.TrimSuffix(videoBase, videoExt)
		outputPath = filepath.Join(videoDir, fmt.Sprintf("%s_trimmed%s", videoName, videoExt))
	}

	// Create output directory if needed
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build ffmpeg command
	startSec := float64(startTrimMs) / 1000.0
	durationSec := float64(trimmedDuration) / 1000.0

	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-i", params.VideoPath,
		"-t", fmt.Sprintf("%.3f", durationSec),
	}

	if params.Lossless {
		args = append(args, "-c", "copy")
	} else {
		args = append(args, "-c:v", "libx264", "-preset", "fast", "-crf", "18")
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	}

	args = append(args, "-avoid_negative_ts", "make_zero")
	args = append(args, outputPath)

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to trim video: %w\nOutput: %s", err, string(output))
	}

	result.OutputPath = outputPath
	return result, nil
}

// detectDeadAir detects silence and black frames at video start/end
func detectDeadAir(params TrimParams) (*TrimDetection, error) {
	detection := &TrimDetection{}

	// Set defaults
	silenceThreshold := params.SilenceThreshold
	if silenceThreshold == 0 {
		silenceThreshold = -50.0
	}
	silenceDuration := params.SilenceDuration
	if silenceDuration == 0 {
		silenceDuration = 0.5
	}
	blackThreshold := params.BlackThreshold
	if blackThreshold == 0 {
		blackThreshold = 0.1
	}
	blackDuration := params.BlackDuration
	if blackDuration == 0 {
		blackDuration = 0.5
	}

	// Detect silence at start
	startSilence, err := detectSilenceStart(params.VideoPath, silenceThreshold, silenceDuration)
	if err == nil && startSilence > 0 {
		detection.StartTrimMs = startSilence
		detection.SilenceDetected = true
	}

	// Detect black frames at start
	startBlack, err := detectBlackStart(params.VideoPath, blackThreshold, blackDuration)
	if err == nil && startBlack > detection.StartTrimMs {
		detection.StartTrimMs = startBlack
		detection.BlackDetected = true
	}

	// Detect silence at end
	endSilence, err := detectSilenceEnd(params.VideoPath, silenceThreshold, silenceDuration)
	if err == nil && endSilence > 0 {
		detection.EndTrimMs = endSilence
		detection.SilenceDetected = true
	}

	// Detect black frames at end
	endBlack, err := detectBlackEnd(params.VideoPath, blackThreshold, blackDuration)
	if err == nil && endBlack > detection.EndTrimMs {
		detection.EndTrimMs = endBlack
		detection.BlackDetected = true
	}

	return detection, nil
}

// detectSilenceStart finds silence at the beginning of a video
func detectSilenceStart(videoPath string, threshold float64, minDuration float64) (int, error) {
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-af", fmt.Sprintf("silencedetect=noise=%gdB:d=%.2f", threshold, minDuration),
		"-f", "null",
		"-",
	)

	output, _ := cmd.CombinedOutput()
	return parseSilenceStart(string(output))
}

// detectSilenceEnd finds silence at the end of a video
func detectSilenceEnd(videoPath string, threshold float64, minDuration float64) (int, error) {
	duration, err := getVideoDurationMs(videoPath)
	if err != nil {
		return 0, err
	}

	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-af", fmt.Sprintf("silencedetect=noise=%gdB:d=%.2f", threshold, minDuration),
		"-f", "null",
		"-",
	)

	output, _ := cmd.CombinedOutput()
	return parseSilenceEnd(string(output), duration)
}

// parseSilenceStart extracts the end of leading silence
func parseSilenceStart(output string) (int, error) {
	// Look for: silence_end: 1.234 | silence_duration: 1.234
	re := regexp.MustCompile(`silence_end:\s*([\d.]+)`)
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			if endTime, err := strconv.ParseFloat(matches[1], 64); err == nil {
				// Only return if silence starts at beginning (within 100ms)
				startRe := regexp.MustCompile(`silence_start:\s*([\d.]+)`)
				if startMatches := startRe.FindStringSubmatch(line); len(startMatches) > 1 {
					if startTime, err := strconv.ParseFloat(startMatches[1], 64); err == nil {
						if startTime < 0.1 {
							return int(endTime * 1000), nil
						}
					}
				}
				// Check previous lines for silence_start
				return int(endTime * 1000), nil
			}
		}
	}
	return 0, fmt.Errorf("no leading silence detected")
}

// parseSilenceEnd extracts trailing silence duration
func parseSilenceEnd(output string, totalDuration int) (int, error) {
	// Look for last silence_start that extends to near the end
	re := regexp.MustCompile(`silence_start:\s*([\d.]+)`)
	var lastSilenceStart float64 = -1

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			if startTime, err := strconv.ParseFloat(matches[1], 64); err == nil {
				lastSilenceStart = startTime
			}
		}
	}

	if lastSilenceStart > 0 {
		// Calculate how much silence is at the end
		endMs := totalDuration - int(lastSilenceStart*1000)
		if endMs > 500 { // At least 500ms of trailing silence
			return endMs, nil
		}
	}

	return 0, fmt.Errorf("no trailing silence detected")
}

// detectBlackStart finds black frames at the beginning of a video
func detectBlackStart(videoPath string, threshold float64, minDuration float64) (int, error) {
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-vf", fmt.Sprintf("blackdetect=d=%.2f:pic_th=%.2f", minDuration, threshold),
		"-f", "null",
		"-",
	)

	output, _ := cmd.CombinedOutput()
	return parseBlackStart(string(output))
}

// detectBlackEnd finds black frames at the end of a video
func detectBlackEnd(videoPath string, threshold float64, minDuration float64) (int, error) {
	duration, err := getVideoDurationMs(videoPath)
	if err != nil {
		return 0, err
	}

	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-vf", fmt.Sprintf("blackdetect=d=%.2f:pic_th=%.2f", minDuration, threshold),
		"-f", "null",
		"-",
	)

	output, _ := cmd.CombinedOutput()
	return parseBlackEnd(string(output), duration)
}

// parseBlackStart extracts the end of leading black frames
func parseBlackStart(output string) (int, error) {
	// Look for: black_end:1.234
	re := regexp.MustCompile(`black_end:\s*([\d.]+)`)
	startRe := regexp.MustCompile(`black_start:\s*([\d.]+)`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		endMatches := re.FindStringSubmatch(line)
		startMatches := startRe.FindStringSubmatch(line)

		if len(endMatches) > 1 && len(startMatches) > 1 {
			startTime, _ := strconv.ParseFloat(startMatches[1], 64)
			endTime, _ := strconv.ParseFloat(endMatches[1], 64)

			// Only return if black starts at beginning (within 100ms)
			if startTime < 0.1 {
				return int(endTime * 1000), nil
			}
		}
	}
	return 0, fmt.Errorf("no leading black frames detected")
}

// parseBlackEnd extracts trailing black frame duration
func parseBlackEnd(output string, totalDuration int) (int, error) {
	// Look for last black_start that extends to near the end
	re := regexp.MustCompile(`black_start:\s*([\d.]+)`)
	var lastBlackStart float64 = -1

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			if startTime, err := strconv.ParseFloat(matches[1], 64); err == nil {
				lastBlackStart = startTime
			}
		}
	}

	if lastBlackStart > 0 {
		endMs := totalDuration - int(lastBlackStart*1000)
		if endMs > 500 { // At least 500ms of trailing black
			return endMs, nil
		}
	}

	return 0, fmt.Errorf("no trailing black frames detected")
}
