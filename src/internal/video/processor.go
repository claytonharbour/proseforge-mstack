package video

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
)

// buildFFmpegCommand builds FFmpeg command as a slice of arguments
func buildFFmpegCommand(segments []types.Segment, segmentsFile string, videoPath string, outputPath string, execute bool) ([]string, error) {
	if len(segments) == 0 {
		return nil, fmt.Errorf("no segments found")
	}

	// Audio directory is next to segments.json
	audioDir := filepath.Dir(segmentsFile)
	audioDir = filepath.Join(audioDir, "audio")

	// Start building command
	cmd := []string{"ffmpeg", "-y"} // -y to overwrite output

	// Add video input
	cmd = append(cmd, "-i", videoPath)

	// Add audio inputs
	for _, seg := range segments {
		audioPath := filepath.Join(audioDir, seg.AudioFile)
		cmd = append(cmd, "-i", audioPath)
	}

	// Build filter_complex
	filterParts := []string{}
	mixerInputs := []string{}

	for i, seg := range segments {
		streamIdx := i + 1 // Video is [0], audio starts at [1]
		delayMs := seg.TimestampMs
		// adelay format: delay_left|delay_right (stereo)
		filterParts = append(filterParts, fmt.Sprintf("[%d]adelay=%d|%d[a%d]", streamIdx, delayMs, delayMs, streamIdx))
		mixerInputs = append(mixerInputs, fmt.Sprintf("[a%d]", streamIdx))
	}

	// Combine all delayed audio with amix
	// normalize=0 prevents volume ducking
	filterComplex := strings.Join(filterParts, "; ")
	filterComplex += fmt.Sprintf("; %samix=inputs=%d:duration=longest:normalize=0[aout]", strings.Join(mixerInputs, ""), len(segments))

	cmd = append(cmd, "-filter_complex", filterComplex)

	// Map video and mixed audio
	cmd = append(cmd, "-map", "0:v", "-map", "[aout]")

	// Output encoding settings - optimized for screen recordings
	cmd = append(cmd,
		"-c:v", "libx264",      // H.264 video codec
		"-preset", "medium",    // Balance speed/quality
		"-crf", "18",           // Higher quality (lower = better, 18 is visually lossless)
		"-tune", "animation",   // Optimized for flat areas/sharp edges in screen recordings
		"-c:a", "aac",          // AAC audio codec
		"-b:a", "192k",         // Audio bitrate
		outputPath,
	)

	if execute {
		ffmpegCmd := exec.Command(cmd[0], cmd[1:]...)
		ffmpegCmd.Stdout = os.Stdout
		ffmpegCmd.Stderr = os.Stderr
		if err := ffmpegCmd.Run(); err != nil {
			return nil, &FFmpegError{Cause: err}
		}
	}

	return cmd, nil
}

// formatCommandForDisplay formats command for readable display with line breaks
func formatCommandForDisplay(cmd []string) string {
	lines := []string{"ffmpeg -y \\"}

	i := 1 // Skip 'ffmpeg' and '-y'
	for i < len(cmd) {
		arg := cmd[i]
		if arg == "-i" {
			lines = append(lines, fmt.Sprintf(`  -i "%s" \`, cmd[i+1]))
			i += 2
		} else if arg == "-filter_complex" {
			// Split filter_complex for readability
			fc := cmd[i+1]
			lines = append(lines, `  -filter_complex "`)
			// Split on semicolons for multiline
			parts := strings.Split(fc, "; ")
			for j, part := range parts {
				if j < len(parts)-1 {
					lines = append(lines, fmt.Sprintf("    %s;", part))
				} else {
					lines = append(lines, fmt.Sprintf("    %s", part))
				}
			}
			lines = append(lines, `  " \`)
			i += 2
		} else if arg == "-map" {
			lines = append(lines, fmt.Sprintf("  -map %s \\", cmd[i+1]))
			i += 2
		} else if strings.HasPrefix(arg, "-") {
			if i+1 < len(cmd) && !strings.HasPrefix(cmd[i+1], "-") {
				lines = append(lines, fmt.Sprintf("  %s %s \\", arg, cmd[i+1]))
				i += 2
			} else {
				lines = append(lines, fmt.Sprintf("  %s \\", arg))
				i += 1
			}
		} else {
			// Last argument (output file)
			lines = append(lines, fmt.Sprintf(`  "%s"`, arg))
			i += 1
		}
	}

	return strings.Join(lines, "\n")
}
