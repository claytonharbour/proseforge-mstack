package video

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/spf13/cobra"
)

func NewBuildCmd() *cobra.Command {
	var videoSvc = video.NewService()
	var narrationPath string
	var videoPath string
	var outputPath string
	var workingDir string
	var ttsEngine string
	var ttsModel string
	var voice string
	var wpm int
	var force bool
	var ttsTimeoutStr string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build narrated video from narration.md and source video",
		Long:  "Full pipeline: parse narration, generate TTS audio, analyze overlaps, build final video with FFmpeg.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve to absolute paths
			var err error
			narrationPath, err = filepath.Abs(narrationPath)
			if err != nil {
				return fmt.Errorf("invalid narration path: %w", err)
			}
			videoPath, err = filepath.Abs(videoPath)
			if err != nil {
				return fmt.Errorf("invalid video path: %w", err)
			}
			outputPath, err = filepath.Abs(outputPath)
			if err != nil {
				return fmt.Errorf("invalid output path: %w", err)
			}
			if workingDir != "" {
				workingDir, err = filepath.Abs(workingDir)
				if err != nil {
					return fmt.Errorf("invalid working dir: %w", err)
				}
			}

			var ttsTimeout time.Duration
			if ttsTimeoutStr != "" {
				ttsTimeout, err = time.ParseDuration(ttsTimeoutStr)
				if err != nil {
					return fmt.Errorf("invalid --tts-timeout: %w", err)
				}
			}

			result, err := videoSvc.RunPipeline(video.PipelineOpts{
				NarrationPath:  narrationPath,
				VideoPath:      videoPath,
				OutputPath:     outputPath,
				WorkingDir:     workingDir,
				TTSEngine:      ttsEngine,
				TTSModel:       ttsModel,
				Voice:          voice,
				WordsPerMinute: wpm,
				Force:          force,
				TTSTimeout:     ttsTimeout,
				Verbose:        verbose,
			})
			if err != nil {
				return err
			}

			fmt.Printf("Built video: %s (%d segments, %d overlaps)\n",
				result.OutputPath, result.SegmentCount, result.Overlaps)
			return nil
		},
	}

	cmd.Flags().StringVar(&narrationPath, "narration", "", "Path to narration.md file (required)")
	cmd.Flags().StringVar(&videoPath, "video", "", "Path to source video file (required)")
	cmd.Flags().StringVar(&outputPath, "output", "", "Path for output .mp4 file (required)")
	cmd.Flags().StringVar(&workingDir, "working-dir", "", "Directory for intermediates (default: temp dir, cleaned up)")
	cmd.Flags().StringVar(&ttsEngine, "tts-engine", "auto", "TTS engine: auto, say, gemini, cloudtts, or vertex")
	cmd.Flags().StringVar(&ttsModel, "tts-model", "", "Gemini model for TTS (default: gemini-2.5-flash-preview-tts)")
	cmd.Flags().StringVar(&voice, "voice", "", "Voice name (default: Karen for say, Kore for gemini)")
	cmd.Flags().IntVar(&wpm, "words-per-minute", 0, "Words per minute (say: -r flag, gemini: prompt pacing)")
	cmd.Flags().IntVar(&wpm, "wpm", 0, "Alias for --words-per-minute")
	cmd.Flags().BoolVar(&force, "force", false, "Force regeneration of audio files")
	cmd.Flags().StringVar(&ttsTimeoutStr, "tts-timeout", "", "Overall TTS generation timeout (e.g. '10m', '30m'). Default: 10m")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Log raw HTTP responses on TTS errors (429/500/503)")

	cmd.MarkFlagRequired("narration")
	cmd.MarkFlagRequired("video")
	cmd.MarkFlagRequired("output")

	return cmd
}
