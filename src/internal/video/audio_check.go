package video

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// AudioCheckParams contains options for audio sync checking
type AudioCheckParams struct {
	VideoPath            string
	SilenceThresholdDB   int // default -50
	SilenceMinDurationMs int // default 100
	DriftThresholdMs     int // default 100
}

// AudioCheckResult contains the results of an audio sync check
type AudioCheckResult struct {
	VideoPath       string       `json:"video_path"`
	VideoDurationMs int          `json:"video_duration_ms"`
	AudioDurationMs int          `json:"audio_duration_ms"`
	DurationDriftMs int          `json:"duration_drift_ms"`
	DriftPercent    float64      `json:"drift_percent"`
	SilenceGaps     []SilenceGap `json:"silence_gaps,omitempty"`
	AudioCodec      string       `json:"audio_codec"`
	AudioSampleRate int          `json:"audio_sample_rate"`
	AudioChannels   int          `json:"audio_channels"`
	HasSyncIssue    bool         `json:"has_sync_issue"`
	Issues          []string     `json:"issues,omitempty"`
}

// SilenceGap represents a period of silence in the audio
type SilenceGap struct {
	StartMs    int `json:"start_ms"`
	EndMs      int `json:"end_ms"`
	DurationMs int `json:"duration_ms"`
}

// CheckAudioSync analyzes a video file for audio synchronization issues
func CheckAudioSync(params AudioCheckParams) (*AudioCheckResult, error) {
	// Set defaults
	if params.SilenceThresholdDB == 0 {
		params.SilenceThresholdDB = -50
	}
	if params.SilenceMinDurationMs == 0 {
		params.SilenceMinDurationMs = 100
	}
	if params.DriftThresholdMs == 0 {
		params.DriftThresholdMs = 100
	}

	result := &AudioCheckResult{
		VideoPath: params.VideoPath,
	}

	// 1. Get stream info via ffprobe
	if err := getAudioStreamInfo(params.VideoPath, result); err != nil {
		return nil, fmt.Errorf("failed to get stream info: %w", err)
	}

	// 2. Calculate drift
	result.DurationDriftMs = result.VideoDurationMs - result.AudioDurationMs
	if result.VideoDurationMs > 0 {
		result.DriftPercent = float64(result.DurationDriftMs) / float64(result.VideoDurationMs) * 100
	}

	// 3. Detect silence gaps
	gaps, err := detectSilenceGaps(params)
	if err != nil {
		// Non-fatal - continue with other checks
		result.Issues = append(result.Issues, fmt.Sprintf("silence detection failed: %v", err))
	} else {
		result.SilenceGaps = gaps
	}

	// 4. Determine if there's a sync issue
	if absInt(result.DurationDriftMs) > params.DriftThresholdMs {
		result.HasSyncIssue = true
		result.Issues = append(result.Issues,
			fmt.Sprintf("audio/video duration drift: %dms (threshold: %dms)",
				result.DurationDriftMs, params.DriftThresholdMs))
	}

	// Flag significant silence gaps (potential dropouts)
	for _, gap := range result.SilenceGaps {
		if gap.DurationMs > 500 { // 500ms+ silence is suspicious
			result.HasSyncIssue = true
			result.Issues = append(result.Issues,
				fmt.Sprintf("long silence gap at %dms (duration: %dms)",
					gap.StartMs, gap.DurationMs))
		}
	}

	return result, nil
}

// getAudioStreamInfo extracts video and audio stream information via ffprobe
func getAudioStreamInfo(videoPath string, result *AudioCheckResult) error {
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		videoPath,
	}

	cmd := exec.Command("ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	// Parse JSON output
	var probeResult struct {
		Streams []struct {
			CodecType  string `json:"codec_type"`
			CodecName  string `json:"codec_name"`
			Duration   string `json:"duration"`
			SampleRate string `json:"sample_rate"`
			Channels   int    `json:"channels"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}

	if err := json.Unmarshal(output, &probeResult); err != nil {
		return err
	}

	for _, stream := range probeResult.Streams {
		duration, _ := strconv.ParseFloat(stream.Duration, 64)
		durationMs := int(duration * 1000)

		switch stream.CodecType {
		case "video":
			result.VideoDurationMs = durationMs
		case "audio":
			result.AudioDurationMs = durationMs
			result.AudioCodec = stream.CodecName
			result.AudioSampleRate, _ = strconv.Atoi(stream.SampleRate)
			result.AudioChannels = stream.Channels
		}
	}

	return nil
}

// detectSilenceGaps uses ffmpeg silencedetect filter to find silence periods
func detectSilenceGaps(params AudioCheckParams) ([]SilenceGap, error) {
	args := []string{
		"-i", params.VideoPath,
		"-af", fmt.Sprintf("silencedetect=noise=%ddB:d=%f",
			params.SilenceThresholdDB,
			float64(params.SilenceMinDurationMs)/1000),
		"-f", "null",
		"-",
	}

	cmd := exec.Command("ffmpeg", args...)
	output, _ := cmd.CombinedOutput() // silencedetect outputs to stderr

	// Parse silence_start and silence_end from output
	var gaps []SilenceGap
	lines := strings.Split(string(output), "\n")

	var currentStart float64
	for _, line := range lines {
		if strings.Contains(line, "silence_start:") {
			parts := strings.Split(line, "silence_start:")
			if len(parts) > 1 {
				currentStart, _ = strconv.ParseFloat(strings.TrimSpace(strings.Split(parts[1], " ")[0]), 64)
			}
		}
		if strings.Contains(line, "silence_end:") {
			parts := strings.Split(line, "silence_end:")
			if len(parts) > 1 {
				endStr := strings.TrimSpace(strings.Split(parts[1], " ")[0])
				end, _ := strconv.ParseFloat(endStr, 64)
				gaps = append(gaps, SilenceGap{
					StartMs:    int(currentStart * 1000),
					EndMs:      int(end * 1000),
					DurationMs: int((end - currentStart) * 1000),
				})
			}
		}
	}

	return gaps, nil
}

// absInt returns the absolute value of an integer
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
