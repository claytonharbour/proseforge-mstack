package video

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
)

// resolveAudioFile tries alternate audio extensions (.wav, .m4a, .mp3) when
// the expected file isn't found. Returns the full path of the first match, or
// empty string if none exist.
func resolveAudioFile(audioDir, audioFile string) string {
	ext := filepath.Ext(audioFile)
	base := strings.TrimSuffix(audioFile, ext)

	for _, alt := range []string{".wav", ".m4a", ".mp3"} {
		if alt == ext {
			continue
		}
		candidate := filepath.Join(audioDir, base+alt)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// getAudioDuration gets the duration of an audio file in milliseconds using ffprobe
func getAudioDuration(audioPath string) (float64, error) {
	// Use os/exec to run ffprobe
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", audioPath)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration * 1000, nil // Convert to ms
}

// analyzeSegments analyzes segments for timing issues (reads from file)
func analyzeSegments(segmentsPath string, audioDir string) (*types.AnalysisResults, error) {
	data, err := os.ReadFile(segmentsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read segments file: %w", err)
	}

	var segments []types.Segment
	if err := json.Unmarshal(data, &segments); err != nil {
		return nil, fmt.Errorf("failed to parse segments JSON: %w", err)
	}

	return analyzeSegmentsFromSlice(segments, audioDir)
}

// analyzeSegmentsFromSlice analyzes segments for timing issues (takes segments directly)
func analyzeSegmentsFromSlice(segments []types.Segment, audioDir string) (*types.AnalysisResults, error) {
	results := &types.AnalysisResults{
		TotalSegments: len(segments),
		Overlaps:      []types.SegmentInfo{},
		TightFits:     []types.SegmentInfo{},
		GoodFits:      []types.SegmentInfo{},
	}

	for i, seg := range segments {
		audioFile := filepath.Join(audioDir, seg.AudioFile)
		if _, err := os.Stat(audioFile); os.IsNotExist(err) {
			// Try alternate extensions — parser defaults to .m4a but TTS may
			// produce .wav (Gemini) or vice versa.
			resolved := resolveAudioFile(audioDir, seg.AudioFile)
			if resolved == "" {
				continue
			}
			audioFile = resolved
		}

		durationMs, err := getAudioDuration(audioFile)
		if err != nil {
			return nil, fmt.Errorf("failed to get duration for %s: %w", audioFile, err)
		}

		startMs := seg.TimestampMs
		endMs := startMs + int(durationMs)

		// Check against next segment
		if i < len(segments)-1 {
			nextStart := segments[i+1].TimestampMs
			gap := nextStart - endMs

			text := seg.Text
			if len(text) > 50 {
				text = text[:50] + "..."
			}

			segInfo := types.SegmentInfo{
				Segment:     seg.Index,
				Text:        text,
				StartMs:     startMs,
				DurationMs:  int(durationMs),
				EndMs:       endMs,
				NextStartMs: nextStart,
				GapMs:       gap,
			}

			if gap < 0 {
				results.Overlaps = append(results.Overlaps, segInfo)
			} else if gap < 500 {
				results.TightFits = append(results.TightFits, segInfo)
			} else {
				results.GoodFits = append(results.GoodFits, segInfo)
			}
		}
	}

	results.Summary = types.Summary{
		Overlaps:  len(results.Overlaps),
		TightFits: len(results.TightFits),
		GoodFits:  len(results.GoodFits),
	}

	return results, nil
}
