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

func NewVertexCmd() *cobra.Command {
	var videoSvc = video.NewService()
	var outputDir string
	var voice string
	var model string
	var wpm int
	var updateJSON bool
	var ttsTimeoutStr string
	var verbose bool
	var project string

	cmd := &cobra.Command{
		Use:   "vertex [segments.json]",
		Short: "Generate TTS audio using Google Vertex AI",
		Long:  "Generates TTS audio files (WAV) for each segment using Google Vertex AI with OAuth authentication.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputDir == "" {
				return fmt.Errorf("--output-dir is required")
			}

			segmentsFile := args[0]

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
				voice = "Kore"
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
				Engine:         "vertex",
				Voice:          voice,
				Model:          model,
				WordsPerMinute: wpm,
				UpdateJSON:     updateJSON,
				SegmentsFile:   segmentsFile,
				Timeout:        timeout,
				Verbose:        verbose,
				Project:        project,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			fmt.Fprintf(os.Stderr, "Generated %d audio files.\n", len(segments))
			return nil
		},
	}

	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory for audio files (required)")
	cmd.Flags().StringVar(&voice, "voice", "Kore", "Voice name (default: Kore)")
	cmd.Flags().StringVar(&model, "model", "gemini-2.5-flash-tts", "Model name")
	cmd.Flags().IntVar(&wpm, "words-per-minute", 0, "Words per minute (prompt pacing directive)")
	cmd.Flags().IntVar(&wpm, "wpm", 0, "Alias for --words-per-minute")
	cmd.Flags().BoolVar(&updateJSON, "update-json", false, "Update segments.json with .wav extension")
	cmd.Flags().StringVar(&ttsTimeoutStr, "tts-timeout", "", "Overall TTS generation timeout (e.g. '10m', '30m'). Default: 10m")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Log raw HTTP requests and responses")
	cmd.Flags().StringVar(&project, "project", "", "Project name for OAuth (default: proseforge)")
	cmd.MarkFlagRequired("output-dir")

	return cmd
}
