package video

import (
	"path/filepath"

	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
)

// Service provides video narration pipeline operations
type Service interface {
	// ParseNarrationMD parses a markdown narration file into segments
	ParseNarrationMD(filepath string) ([]types.Segment, error)

	// GenerateTTS generates TTS audio for segments using the given options.
	GenerateTTS(opts TTSOpts) error

	// ProcessVideo builds final video with FFmpeg
	// segmentsFile is used to determine the audio directory location
	ProcessVideo(segments []types.Segment, segmentsFile string, videoPath string, outputPath string, execute bool) ([]string, error)

	// AnalyzeOverlap analyzes segments for timing overlaps (reads segments from file)
	AnalyzeOverlap(segmentsPath string, audioDir string) (*types.AnalysisResults, error)

	// AnalyzeOverlapWithSegments analyzes segments for timing overlaps (takes segments directly)
	AnalyzeOverlapWithSegments(segments []types.Segment, audioDir string) (*types.AnalysisResults, error)

	// RunPipeline executes the full narration video pipeline end-to-end.
	RunPipeline(opts PipelineOpts) (*PipelineResult, error)
}

type service struct{}

// NewService creates a new VideoService
func NewService() Service {
	return &service{}
}

func (s *service) ParseNarrationMD(filepath string) ([]types.Segment, error) {
	return parseNarrationMD(filepath)
}

func (s *service) GenerateTTS(opts TTSOpts) error {
	// Derive segments file path for backward compat (segments.json is sibling to audio dir)
	if opts.SegmentsFile == "" {
		segmentsFile := filepath.Join(opts.OutputDir, "..", "segments.json")
		if abs, err := filepath.Abs(segmentsFile); err == nil {
			segmentsFile = abs
		}
		opts.SegmentsFile = segmentsFile
	}
	if opts.Engine == "say" {
		return generateTTSSay(opts)
	}
	return generateTTSAPI(opts)
}

func (s *service) ProcessVideo(segments []types.Segment, segmentsFile string, videoPath string, outputPath string, execute bool) ([]string, error) {
	return buildFFmpegCommand(segments, segmentsFile, videoPath, outputPath, execute)
}

func (s *service) AnalyzeOverlap(segmentsPath string, audioDir string) (*types.AnalysisResults, error) {
	return analyzeSegments(segmentsPath, audioDir)
}

func (s *service) AnalyzeOverlapWithSegments(segments []types.Segment, audioDir string) (*types.AnalysisResults, error) {
	return analyzeSegmentsFromSlice(segments, audioDir)
}
