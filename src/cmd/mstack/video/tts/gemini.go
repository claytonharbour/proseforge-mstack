package tts

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/spf13/cobra"
)

func NewGeminiCmd() *cobra.Command {
	var videoSvc = video.NewService()
	var outputDir string
	var voice string
	var wpm int
	var updateJSON bool
	var ttsTimeoutStr string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "gemini [segments.json]",
		Short: "Generate TTS audio using Google Gemini API",
		Long:  "Generates TTS audio files for each segment using Google Gemini API.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputDir == "" {
				return fmt.Errorf("--output-dir is required")
			}

			segmentsFile := args[0]

			// Load segments
			data, err := os.ReadFile(segmentsFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to read segments file: %v\n", err)
				return err
			}

			var segments []types.Segment
			if err := json.Unmarshal(data, &segments); err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to parse segments JSON: %v\n", err)
				return err
			}

			if voice == "" {
				voice = "Charon"
			}

			var timeout time.Duration
			if ttsTimeoutStr != "" {
				timeout, err = time.ParseDuration(ttsTimeoutStr)
				if err != nil {
					return fmt.Errorf("invalid --tts-timeout: %w", err)
				}
			}

			err = videoSvc.GenerateTTS(video.TTSOpts{
				Segments:       segments,
				OutputDir:      outputDir,
				Engine:         "gemini",
				Voice:          voice,
				WordsPerMinute: wpm,
				UpdateJSON:     updateJSON,
				Timeout:        timeout,
				Verbose:        verbose,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			// Output success message to stderr (not stdout for pipeability)
			fmt.Fprintf(os.Stderr, "Generated %d audio files.\n", len(segments))
			return nil
		},
	}

	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory for audio files (required)")
	cmd.Flags().StringVar(&voice, "voice", "Charon", "Voice name (default: Charon)")
	cmd.Flags().IntVar(&wpm, "words-per-minute", 0, "Words per minute (prompt pacing directive)")
	cmd.Flags().IntVar(&wpm, "wpm", 0, "Alias for --words-per-minute")
	cmd.Flags().BoolVar(&updateJSON, "update-json", false, "Update segments.json with .wav extension")
	cmd.Flags().StringVar(&ttsTimeoutStr, "tts-timeout", "", "Overall TTS generation timeout (e.g. '10m', '30m'). Default: 10m")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Log raw HTTP responses on TTS errors (429/500/503)")
	cmd.MarkFlagRequired("output-dir")

	return cmd
}
