package video

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PipelineOpts configures the unified video build pipeline.
type PipelineOpts struct {
	NarrationPath string // required — absolute path to narration.md
	VideoPath     string // required — absolute path to source video
	OutputPath    string // required — absolute path for final .mp4
	WorkingDir    string // optional — if empty, uses os.MkdirTemp then cleans up

	TTSEngine string // "say", "gemini", "cloudtts", "vertex", or "auto" (default: "auto")
	TTSModel  string // gemini model name (default: "gemini-2.5-flash-preview-tts")
	Voice     string // voice name (default: engine-appropriate)
	WordsPerMinute int           // wpm — say uses -r flag, gemini uses prompt pacing (default: 200 for say)
	Force      bool          // delete existing audio before regenerating
	TTSTimeout time.Duration // overall TTS generation timeout (default 10m)
	Verbose    bool          // log raw HTTP responses on TTS errors (429/500/503)
}

// PipelineResult contains the output of a successful pipeline run.
type PipelineResult struct {
	OutputPath   string `json:"output_path"`
	WorkingDir   string `json:"working_dir"`
	SegmentCount int    `json:"segment_count"`
	Overlaps     int    `json:"overlaps"`
}

// RunPipeline executes the full narration video pipeline:
// parse narration → generate TTS → analyze overlaps → build video with FFmpeg.
func (s *service) RunPipeline(opts PipelineOpts) (*PipelineResult, error) {
	// Validate required paths are absolute
	if !filepath.IsAbs(opts.NarrationPath) {
		return nil, fmt.Errorf("narration path must be absolute: %s", opts.NarrationPath)
	}
	if !filepath.IsAbs(opts.VideoPath) {
		return nil, fmt.Errorf("video path must be absolute: %s", opts.VideoPath)
	}
	if !filepath.IsAbs(opts.OutputPath) {
		return nil, fmt.Errorf("output path must be absolute: %s", opts.OutputPath)
	}

	// Validate input files exist
	if _, err := os.Stat(opts.NarrationPath); err != nil {
		return nil, fmt.Errorf("narration file not found: %w", err)
	}
	if _, err := os.Stat(opts.VideoPath); err != nil {
		return nil, fmt.Errorf("video file not found: %w", err)
	}

	// Resolve engine (auto-detect if empty or "auto")
	resolved, err := ResolveEngine(opts.TTSEngine, "")
	if err != nil {
		return nil, err
	}
	opts.TTSEngine = resolved
	if opts.Voice == "" {
		if opts.TTSEngine == "say" {
			opts.Voice = "Karen"
		} else {
			opts.Voice = "Kore"
		}
	}
	if opts.WordsPerMinute == 0 && opts.TTSEngine == "say" {
		opts.WordsPerMinute = 200
	}

	// Resolve working dir
	autoCleanup := false
	workingDir := opts.WorkingDir
	if workingDir == "" {
		tmpDir, err := os.MkdirTemp("", "mstack-video-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp working dir: %w", err)
		}
		workingDir = tmpDir
		autoCleanup = true
	} else {
		if !filepath.IsAbs(workingDir) {
			return nil, fmt.Errorf("working dir must be absolute: %s", workingDir)
		}
	}
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create working dir: %w", err)
	}

	// Ensure output dir exists
	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Step 1: Parse narration → write segments.json to working dir
	segments, err := parseNarrationMD(opts.NarrationPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse narration: %w", err)
	}

	segmentsFile := filepath.Join(workingDir, "segments.json")
	segmentsJSON, err := json.MarshalIndent(segments, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal segments: %w", err)
	}
	if err := os.WriteFile(segmentsFile, segmentsJSON, 0644); err != nil {
		return nil, fmt.Errorf("failed to write segments.json: %w", err)
	}

	// Step 2: Generate TTS → write audio files to workingDir/audio/
	audioDir := filepath.Join(workingDir, "audio")
	if opts.Force {
		os.RemoveAll(audioDir)
	}

	ttsOpts := TTSOpts{
		Segments:       segments,
		OutputDir:      audioDir,
		Engine:         opts.TTSEngine,
		Voice:          opts.Voice,
		WordsPerMinute: opts.WordsPerMinute,
		Model:          opts.TTSModel,
		UpdateJSON:     true,
		SegmentsFile:   segmentsFile,
		Timeout:        opts.TTSTimeout,
		Verbose:        opts.Verbose,
	}
	if opts.TTSEngine == "say" {
		err = generateTTSSay(ttsOpts)
	} else {
		err = generateTTSAPI(ttsOpts)
	}
	if err != nil {
		// On error, keep working dir for debugging
		return nil, err
	}

	// Step 3: Re-read segments.json (TTS updates audio_file extensions on disk)
	updatedData, err := os.ReadFile(segmentsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read updated segments: %w", err)
	}
	if err := json.Unmarshal(updatedData, &segments); err != nil {
		return nil, fmt.Errorf("failed to parse updated segments: %w", err)
	}

	// Step 4: Analyze overlaps (warn-only, non-fatal)
	overlaps := 0
	if results, err := analyzeSegmentsFromSlice(segments, audioDir); err == nil {
		overlaps = results.Summary.Overlaps
		if overlaps > 0 {
			fmt.Printf("Warning: %d audio overlap(s) detected\n", overlaps)
		}
	}

	// Step 5: Build video with FFmpeg → write to OutputPath
	_, err = buildFFmpegCommand(segments, segmentsFile, opts.VideoPath, opts.OutputPath, true)
	if err != nil {
		// On error, keep working dir for debugging
		return nil, err
	}

	// Step 6: Cleanup if auto-created and no error
	if autoCleanup {
		os.RemoveAll(workingDir)
		workingDir = "(cleaned up)"
	}

	return &PipelineResult{
		OutputPath:   opts.OutputPath,
		WorkingDir:   workingDir,
		SegmentCount: len(segments),
		Overlaps:     overlaps,
	}, nil
}
