package audio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/claytonharbour/proseforge-mstack/src/internal/audio"
	"github.com/spf13/cobra"
)

// NewNarrateCmd creates the "audio narrate" subcommand.
func NewNarrateCmd() *cobra.Command {
	var (
		outputDir     string
		engine        string
		voice         string
		model         string
		wpm           int
		chapterGap    float64
		keepChunks    bool
		chapterTitles bool
		dryRun        bool
		ttsTimeout    string
		verbose       bool
		project       string
		jsonOutput    bool
	)

	cmd := &cobra.Command{
		Use:   "narrate [markdown-file]",
		Short: "Generate audiobook-style TTS from long-form markdown",
		Long: `Generates audiobook-style TTS audio from a markdown file with ## chapter headings.

Chapters are chunked into API-sized segments, processed through TTS, then
concatenated into per-chapter audio files and a combined audiobook.

Examples:
  mstack audio narrate story.md
  mstack audio narrate story.md --engine=cloudtts --voice=Kore
  mstack audio narrate story.md --dry-run
  mstack audio narrate story.md -o ./audiobook --keep-chunks`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputFile := args[0]

			// Resolve to absolute path
			absInput, err := filepath.Abs(inputFile)
			if err != nil {
				return fmt.Errorf("failed to resolve input path: %w", err)
			}

			// Verify file exists
			if _, err := os.Stat(absInput); os.IsNotExist(err) {
				return fmt.Errorf("file not found: %s", inputFile)
			}

			// Parse timeout
			timeout := 30 * time.Minute
			if ttsTimeout != "" {
				parsed, err := time.ParseDuration(ttsTimeout)
				if err != nil {
					return fmt.Errorf("invalid --tts-timeout: %w", err)
				}
				timeout = parsed
			}

			// Resolve output dir to absolute
			if outputDir != "" {
				absOutput, err := filepath.Abs(outputDir)
				if err != nil {
					return fmt.Errorf("failed to resolve output path: %w", err)
				}
				outputDir = absOutput
			}

			// Get project from persistent flag if not set
			if project == "" {
				project, _ = cmd.Flags().GetString("project")
			}

			result, err := audio.RunNarrate(audio.NarrateOpts{
				InputFile:      absInput,
				OutputDir:      outputDir,
				Engine:         engine,
				Voice:          voice,
				Model:          model,
				WordsPerMinute: wpm,
				ChapterGapSec:  chapterGap,
				KeepChunks:     keepChunks,
				ChapterTitles:  chapterTitles,
				Timeout:        timeout,
				Verbose:        verbose,
				Project:        project,
				DryRun:         dryRun,
			})
			if err != nil {
				return err
			}

			if jsonOutput {
				data, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal result: %w", err)
				}
				fmt.Println(string(data))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "Output directory (default: ./<story-slug>/)")
	cmd.Flags().StringVar(&engine, "engine", "auto", "TTS engine: say, gemini, cloudtts, vertex, auto")
	cmd.Flags().StringVar(&voice, "voice", "", "Voice name (engine default if empty)")
	cmd.Flags().StringVar(&model, "model", "", "Model name (engine default if empty)")
	cmd.Flags().IntVar(&wpm, "wpm", 0, "Words per minute (engine default if 0)")
	cmd.Flags().Float64Var(&chapterGap, "chapter-gap", 2.0, "Silence between chapters in seconds")
	cmd.Flags().BoolVar(&keepChunks, "keep-chunks", false, "Keep intermediate chunk audio files")
	cmd.Flags().BoolVar(&chapterTitles, "chapter-titles", true, "Speak chapter headings as audio")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Parse and estimate only, no TTS generation")
	cmd.Flags().StringVar(&ttsTimeout, "tts-timeout", "30m", "TTS API timeout duration")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Verbose logging")
	cmd.Flags().StringVar(&project, "project", "", "Project name for OAuth engines")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output result as JSON")

	return cmd
}
