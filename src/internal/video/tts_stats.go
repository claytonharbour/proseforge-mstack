package video

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TTSStats records per-segment TTS generation metrics.
type TTSStats struct {
	Timestamp  time.Time `json:"ts"`
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	Segment    int       `json:"segment"`
	TextLen    int       `json:"text_len"`
	AudioFile  string    `json:"audio_file"`
	DurationMS int64     `json:"duration_ms"`
	LatencyMS  int64     `json:"latency_ms"`
	Retries    int       `json:"retries"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
}

// appendStats appends a stats entry as a JSONL line to the given path.
func appendStats(path string, stats TTSStats) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create stats directory: %w", err)
	}

	data, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("failed to marshal stats: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open stats file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write stats: %w", err)
	}

	return nil
}

// statsPath returns the default stats file path for an output directory.
// Stats file is a sibling of the audio dir: {output-dir}/../tts_stats.jsonl
func statsPath(outputDir string) string {
	return filepath.Join(outputDir, "..", "tts_stats.jsonl")
}

// audioDurationMS returns the duration in milliseconds of a WAV file by reading its header.
// Returns 0 if the file can't be read or isn't WAV.
func audioDurationMS(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 44 {
		return 0
	}

	// Check for WAV header
	if string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		// PCM WAV: data chunk size / (sample_rate * channels * bytes_per_sample)
		dataBytes := len(data) - 44
		// Assume 24kHz mono 16-bit (standard for Gemini TTS)
		return int64(dataBytes) * 1000 / (24000 * 2)
	}

	// For MP3 files, use file size as rough estimate
	// MP3 at ~128kbps: duration_ms ≈ file_size * 8 / 128
	if len(data) > 3 && (data[0] == 0xFF && (data[1]&0xE0) == 0xE0) || // MP3 sync word
		(string(data[0:3]) == "ID3") { // ID3 tag
		return int64(len(data)) * 8 * 1000 / (128 * 1000)
	}

	return 0
}
