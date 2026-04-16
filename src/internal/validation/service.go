package validation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
)

// Service provides video validation operations
type Service interface {
	// ValidateVideo performs full validation on a video
	ValidateVideo(videoPath string, segments []types.Segment, outputDir string, config *ValidationConfig) (*ValidationResult, error)

	// ValidateVideoWithScript performs validation with script path for line number mapping
	ValidateVideoWithScript(videoPath string, segments []types.Segment, outputDir string, config *ValidationConfig, scriptPath string) (*ValidationResult, error)

	// ValidateTaggedVideo performs semantic validation using inline tags
	ValidateTaggedVideo(videoPath string, segments []TaggedSegment, outputDir string, config *ValidationConfig, scriptPath string, frameOffset int) (*ValidationResult, error)

	// ExtractFrames extracts frames from video at segment timestamps
	ExtractFrames(videoPath string, segments []types.Segment, outputDir string) ([]FrameInfo, error)

	// AnalyzeFrames performs OCR on extracted frames
	AnalyzeFrames(frames []FrameInfo) ([]FrameInfo, error)

	// ValidateContent validates OCR text against narration
	ValidateContent(segments []types.Segment, frames []FrameInfo, config *ValidationConfig) (*ValidationResult, error)
}

type service struct{}

// NewService creates a new validation service
func NewService() Service {
	return &service{}
}

func (s *service) ValidateVideo(videoPath string, segments []types.Segment, outputDir string, config *ValidationConfig) (*ValidationResult, error) {
	return s.ValidateVideoWithScript(videoPath, segments, outputDir, config, "")
}

func (s *service) ValidateVideoWithScript(videoPath string, segments []types.Segment, outputDir string, config *ValidationConfig, scriptPath string) (*ValidationResult, error) {
	if config == nil {
		config = DefaultConfig()
	}

	videoName := filepath.Base(filepath.Dir(videoPath))

	// Step 1: Extract frames
	framesDir := filepath.Join(outputDir, "frames")
	frames, err := ExtractFrames(videoPath, segments, framesDir)
	if err != nil {
		return nil, fmt.Errorf("frame extraction failed: %w", err)
	}

	// Step 2: Perform OCR
	frames, err = ExtractTextFromFrames(frames)
	if err != nil {
		return nil, fmt.Errorf("OCR failed: %w", err)
	}

	// Step 3: Parse test script if provided
	var narrationCalls []NarrationCall
	if scriptPath != "" {
		narrationCalls, err = ParseTestScript(scriptPath)
		if err != nil {
			// Log warning but continue - script parsing is optional
			fmt.Printf("Warning: could not parse script %s: %v\n", scriptPath, err)
		}
	}

	// Step 4: Validate content
	result, err := s.ValidateContent(segments, frames, config)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Step 5: Enrich issues with line numbers if script was parsed
	if len(narrationCalls) > 0 && scriptPath != "" {
		for i := range result.Issues {
			// Try to match by segment index first (most reliable)
			call := FindCallByIndex(narrationCalls, result.Issues[i].SegmentIndex)
			if call == nil {
				// Fall back to text matching
				call = FindMatchingCall(narrationCalls, result.Issues[i].NarrationText)
			}
			if call != nil {
				result.Issues[i].TestFile = filepath.Base(scriptPath)
				result.Issues[i].TestLine = call.LineNumber
				result.Issues[i].CodeContext = call.CodeContext
			}
		}
		result.ScriptPath = scriptPath
	}

	result.VideoName = videoName
	result.Timestamp = time.Now().Format(time.RFC3339)
	result.FramesDir = framesDir

	return result, nil
}

func (s *service) ValidateTaggedVideo(videoPath string, segments []TaggedSegment, outputDir string, config *ValidationConfig, scriptPath string, frameOffset int) (*ValidationResult, error) {
	if config == nil {
		config = DefaultConfig()
	}

	videoName := filepath.Base(filepath.Dir(videoPath))
	framesDir := filepath.Join(outputDir, "frames")

	// Step 1: Extract frames for tagged segments (multiple frames per segment)
	segments, err := ExtractFramesForTaggedSegments(videoPath, segments, framesDir, frameOffset)
	if err != nil {
		return nil, fmt.Errorf("frame extraction failed: %w", err)
	}

	// Step 2: Perform OCR on all frames
	for i := range segments {
		for j := range segments[i].Frames {
			frames, err := ExtractTextFromFrames([]FrameInfo{segments[i].Frames[j]})
			if err != nil {
				return nil, fmt.Errorf("OCR failed for segment %d frame %d: %w", i, j, err)
			}
			segments[i].Frames[j] = frames[0]
		}
	}

	// Step 3: Parse test script if provided
	var narrationCalls []NarrationCall
	if scriptPath != "" {
		narrationCalls, err = ParseTestScript(scriptPath)
		if err != nil {
			fmt.Printf("Warning: could not parse script %s: %v\n", scriptPath, err)
		}
	}

	// Step 4: Validate tagged segments
	issues := []ValidationIssue{}
	segmentsChecked := 0

	for _, seg := range segments {
		segmentsChecked++
		if seg.HasTag() {
			// Use semantic validation for tagged segments
			segIssues := ValidateTaggedSegment(&seg, config)
			issues = append(issues, segIssues...)
		} else {
			// Fall back to standard validation for untagged segments
			if len(seg.Frames) > 0 {
				stdIssues := ValidateSegment(types.Segment{
					Index:       seg.Index,
					Timestamp:   seg.Timestamp,
					TimestampMs: seg.TimestampMs,
					Text:        seg.Text,
				}, seg.Frames[0].OCRText, config)
				issues = append(issues, stdIssues...)
			}
		}
	}

	// Step 5: Enrich issues with line numbers
	if len(narrationCalls) > 0 && scriptPath != "" {
		for i := range issues {
			call := FindCallByIndex(narrationCalls, issues[i].SegmentIndex)
			if call == nil {
				call = FindMatchingCall(narrationCalls, issues[i].NarrationText)
			}
			if call != nil {
				issues[i].TestFile = filepath.Base(scriptPath)
				issues[i].TestLine = call.LineNumber
				issues[i].CodeContext = call.CodeContext
			}
		}
	}

	// Calculate summary
	summary := ValidationSummary{
		TotalSegments:   len(segments),
		SegmentsChecked: segmentsChecked,
		TotalIssues:     len(issues),
	}

	for _, issue := range issues {
		switch issue.Severity {
		case "high":
			summary.HighSeverity++
		case "medium":
			summary.MediumSeverity++
		case "low":
			summary.LowSeverity++
		}
	}

	return &ValidationResult{
		VideoName: videoName,
		Timestamp: time.Now().Format(time.RFC3339),
		FramesDir: framesDir,
		ScriptPath: scriptPath,
		Issues:    issues,
		Summary:   summary,
	}, nil
}

func (s *service) ExtractFrames(videoPath string, segments []types.Segment, outputDir string) ([]FrameInfo, error) {
	return ExtractFrames(videoPath, segments, outputDir)
}

func (s *service) AnalyzeFrames(frames []FrameInfo) ([]FrameInfo, error) {
	return ExtractTextFromFrames(frames)
}

func (s *service) ValidateContent(segments []types.Segment, frames []FrameInfo, config *ValidationConfig) (*ValidationResult, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Build frame map by index
	frameMap := make(map[int]FrameInfo)
	for _, f := range frames {
		frameMap[f.SegmentIndex] = f
	}

	issues := []ValidationIssue{}
	segmentsChecked := 0

	for _, seg := range segments {
		frame, ok := frameMap[seg.Index]
		if !ok {
			continue
		}

		segmentsChecked++
		segIssues := ValidateSegment(seg, frame.OCRText, config)
		issues = append(issues, segIssues...)
	}

	// Calculate summary
	summary := ValidationSummary{
		TotalSegments:   len(segments),
		SegmentsChecked: segmentsChecked,
		TotalIssues:     len(issues),
	}

	for _, issue := range issues {
		switch issue.Severity {
		case "high":
			summary.HighSeverity++
		case "medium":
			summary.MediumSeverity++
		case "low":
			summary.LowSeverity++
		}
	}

	return &ValidationResult{
		Issues:  issues,
		Summary: summary,
	}, nil
}

// SaveResult saves validation result to JSON file
func SaveResult(result *ValidationResult, outputPath string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write result: %w", err)
	}

	return nil
}

// SaveFramesInfo saves frame info with OCR to JSON file
func SaveFramesInfo(frames []FrameInfo, outputPath string) error {
	data, err := json.MarshalIndent(frames, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal frames: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write frames: %w", err)
	}

	return nil
}
