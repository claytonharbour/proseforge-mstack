package audio

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConcatAudioFiles concatenates multiple audio files into a single output file
// using ffmpeg's concat demuxer. Re-encodes when the input codec (e.g. PCM from
// macOS say) cannot be stream-copied into the output container.
func ConcatAudioFiles(inputFiles []string, outputPath string) error {
	if len(inputFiles) == 0 {
		return fmt.Errorf("no input files provided")
	}

	// Create output directory if needed
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Single file: re-encode to normalize codec
	if len(inputFiles) == 1 {
		return reencodeAudio(inputFiles[0], outputPath)
	}

	// Create temporary concat file
	concatFile, err := os.CreateTemp("", "ffmpeg-audio-concat-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(concatFile.Name())

	for _, f := range inputFiles {
		absPath, err := filepath.Abs(f)
		if err != nil {
			absPath = f
		}
		escapedPath := strings.ReplaceAll(absPath, "'", "'\\''")
		fmt.Fprintf(concatFile, "file '%s'\n", escapedPath)
	}
	concatFile.Close()

	// Determine codec args based on output extension
	codecArgs := codecForExt(filepath.Ext(outputPath))

	args := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile.Name(),
	}
	args = append(args, codecArgs...)
	args = append(args, outputPath)

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg concat failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// reencodeAudio re-encodes a single audio file to the output format.
func reencodeAudio(inputPath, outputPath string) error {
	codecArgs := codecForExt(filepath.Ext(outputPath))

	args := []string{"-y", "-i", inputPath}
	args = append(args, codecArgs...)
	args = append(args, outputPath)

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg re-encode failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// codecForExt returns ffmpeg codec arguments appropriate for the output extension.
func codecForExt(ext string) []string {
	switch strings.ToLower(ext) {
	case ".mp3":
		return []string{"-c:a", "libmp3lame", "-b:a", "192k"}
	case ".m4a", ".aac":
		return []string{"-c:a", "aac", "-b:a", "192k"}
	case ".wav":
		return []string{"-c:a", "pcm_s16le"}
	default:
		return []string{"-c:a", "aac", "-b:a", "192k"}
	}
}

// GenerateSilence generates a silent audio file of the given duration using ffmpeg.
func GenerateSilence(outputPath string, durationSec float64, sampleRate int, channels int) error {
	if durationSec <= 0 {
		return fmt.Errorf("duration must be positive, got %f", durationSec)
	}
	if sampleRate <= 0 {
		sampleRate = 24000
	}
	if channels <= 0 {
		channels = 1
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Determine codec from extension
	ext := strings.ToLower(filepath.Ext(outputPath))
	var codecArgs []string
	switch ext {
	case ".mp3":
		codecArgs = []string{"-c:a", "libmp3lame", "-b:a", "128k"}
	case ".m4a", ".aac":
		codecArgs = []string{"-c:a", "aac", "-b:a", "128k"}
	case ".wav":
		codecArgs = []string{"-c:a", "pcm_s16le"}
	default:
		codecArgs = []string{"-c:a", "libmp3lame", "-b:a", "128k"}
	}

	args := []string{
		"-y",
		"-f", "lavfi",
		"-i", fmt.Sprintf("anullsrc=r=%d:cl=%s", sampleRate, channelLayout(channels)),
		"-t", fmt.Sprintf("%.3f", durationSec),
	}
	args = append(args, codecArgs...)
	args = append(args, outputPath)

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg silence generation failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// GetAudioInfo returns sample rate and channel count from an audio file using ffprobe.
func GetAudioInfo(filePath string) (sampleRate int, channels int, err error) {
	// Get sample rate
	cmd := exec.Command("ffprobe", "-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=sample_rate,channels",
		"-of", "csv=p=0",
		filePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) >= 2 {
		fmt.Sscanf(parts[0], "%d", &sampleRate)
		fmt.Sscanf(parts[1], "%d", &channels)
	}

	if sampleRate == 0 {
		sampleRate = 24000
	}
	if channels == 0 {
		channels = 1
	}

	return sampleRate, channels, nil
}

func channelLayout(channels int) string {
	if channels >= 2 {
		return "stereo"
	}
	return "mono"
}
