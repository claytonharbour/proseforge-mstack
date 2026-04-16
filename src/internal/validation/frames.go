package validation

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
)

// ExtractFrames extracts frames from video at segment timestamps
func ExtractFrames(videoPath string, segments []types.Segment, outputDir string) ([]FrameInfo, error) {
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create frames directory: %w", err)
	}

	frames := make([]FrameInfo, 0, len(segments))

	for _, seg := range segments {
		framePath := filepath.Join(outputDir, fmt.Sprintf("frame_%03d.png", seg.Index))

		// Convert timestamp_ms to seconds for ffmpeg
		timestampSec := float64(seg.TimestampMs) / 1000.0

		// Check if frame already exists
		if _, err := os.Stat(framePath); err == nil {
			frames = append(frames, FrameInfo{
				SegmentIndex: seg.Index,
				Timestamp:    seg.Timestamp,
				TimestampMs:  seg.TimestampMs,
				FramePath:    framePath,
			})
			continue
		}

		// Extract frame using ffmpeg
		// -ss before -i for fast seeking
		// -frames:v 1 to extract single frame
		// -update 1 to overwrite if exists
		cmd := exec.Command("ffmpeg",
			"-ss", fmt.Sprintf("%.3f", timestampSec),
			"-i", videoPath,
			"-frames:v", "1",
			"-update", "1",
			"-y", // overwrite
			framePath,
		)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("failed to extract frame at %s: %w\nOutput: %s", seg.Timestamp, err, string(output))
		}

		frames = append(frames, FrameInfo{
			SegmentIndex: seg.Index,
			Timestamp:    seg.Timestamp,
			TimestampMs:  seg.TimestampMs,
			FramePath:    framePath,
		})
	}

	return frames, nil
}

// ExtractSingleFrame extracts a frame at a specific timestamp
func ExtractSingleFrame(videoPath string, timestampMs int, outputPath string) error {
	timestampSec := float64(timestampMs) / 1000.0

	cmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", timestampSec),
		"-i", videoPath,
		"-frames:v", "1",
		"-update", "1",
		"-y",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to extract frame: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// ExtractFramesForTaggedSegment extracts multiple frames for a tagged segment
// based on its timing configuration
func ExtractFramesForTaggedSegment(videoPath string, segment *TaggedSegment, outputDir string, frameOffset int) ([]FrameInfo, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create frames directory: %w", err)
	}

	offsets := segment.GetFrameOffsets(frameOffset)
	positions := segment.GetFramePositions()
	frames := make([]FrameInfo, 0, len(offsets))

	for i, offset := range offsets {
		// Calculate absolute timestamp
		absoluteMs := segment.TimestampMs + offset
		if absoluteMs < 0 {
			absoluteMs = 0 // Clamp to video start
		}

		// Generate unique frame filename
		framePath := filepath.Join(outputDir, fmt.Sprintf("frame_%03d_%s.png", segment.Index, positions[i]))

		// Check if frame already exists
		if _, err := os.Stat(framePath); err == nil {
			frames = append(frames, FrameInfo{
				SegmentIndex: segment.Index,
				Timestamp:    fmt.Sprintf("%02d:%02d", absoluteMs/60000, (absoluteMs/1000)%60),
				TimestampMs:  absoluteMs,
				FramePath:    framePath,
			})
			continue
		}

		// Extract frame
		if err := ExtractSingleFrame(videoPath, absoluteMs, framePath); err != nil {
			return nil, fmt.Errorf("failed to extract frame at offset %d for segment %d: %w", offset, segment.Index, err)
		}

		frames = append(frames, FrameInfo{
			SegmentIndex: segment.Index,
			Timestamp:    fmt.Sprintf("%02d:%02d", absoluteMs/60000, (absoluteMs/1000)%60),
			TimestampMs:  absoluteMs,
			FramePath:    framePath,
		})
	}

	return frames, nil
}

// ExtractFramesForTaggedSegments extracts frames for all tagged segments
func ExtractFramesForTaggedSegments(videoPath string, segments []TaggedSegment, outputDir string, frameOffset int) ([]TaggedSegment, error) {
	result := make([]TaggedSegment, len(segments))
	copy(result, segments)

	for i := range result {
		frames, err := ExtractFramesForTaggedSegment(videoPath, &result[i], outputDir, frameOffset)
		if err != nil {
			return nil, err
		}
		result[i].Frames = frames
	}

	return result, nil
}
