package video

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/spf13/cobra"
)

func NewEstimateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "estimate",
		Short: "Estimate TTS duration before generation",
		Long:  "Estimate narration duration and predict timing issues before generating audio.",
	}

	cmd.AddCommand(newEstimateTextCmd())
	cmd.AddCommand(newEstimateNarrationCmd())

	return cmd
}

func newEstimateTextCmd() *cobra.Command {
	var engine string
	var wpm int

	cmd := &cobra.Command{
		Use:   "text <text>",
		Short: "Estimate duration for a single text",
		Long: `Estimate TTS duration for arbitrary text.

Examples:
  mstack video estimate text "Enter your email address to sign in"
  mstack video estimate text "Hello world" --engine=gemini
  mstack video estimate text "Fast speech" --wpm=250`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text := args[0]
			result := video.EstimateSingleText(text, engine, wpm)

			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		},
	}

	cmd.Flags().StringVar(&engine, "engine", "say", "TTS engine: say or gemini")
	cmd.Flags().IntVar(&wpm, "wpm", 0, "Words per minute (default: 200 for say, 150 for gemini)")

	return cmd
}

func newEstimateNarrationCmd() *cobra.Command {
	var project string
	var engine string
	var wpm int
	var validate bool

	cmd := &cobra.Command{
		Use:   "narration <video-name>",
		Short: "Estimate duration for entire narration",
		Long: `Estimate TTS duration for all segments and predict timing issues.

Use --validate to check for segments that won't fit in their time slots.

Examples:
  # Estimate durations only
  mstack video estimate narration story-forge

  # Validate timing (predict overlaps)
  mstack video estimate narration story-forge --validate

  # With different engine
  mstack video estimate narration story-forge --engine=gemini --validate`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoName := args[0]

			if project == "" {
				project = "proseforge"
			}

			// Resolve projects root
			projectsRoot := os.Getenv("MSTACK_PROJECTS_ROOT")
			if projectsRoot == "" {
				// Default to projects/ subdirectory
				projectsRoot = "projects"
			}

			// Find narration.md
			narrationPath := filepath.Join(projectsRoot, project, "input", videoName, "narration.md")
			if _, err := os.Stat(narrationPath); os.IsNotExist(err) {
				return fmt.Errorf("narration.md not found: %s", narrationPath)
			}

			// Parse narration
			videoSvc := video.NewService()
			segments, err := videoSvc.ParseNarrationMD(narrationPath)
			if err != nil {
				return fmt.Errorf("failed to parse narration: %w", err)
			}

			// Estimate durations
			params := video.EstimationParams{
				Engine:         engine,
				WordsPerMinute: wpm,
				Validate:       validate,
			}

			result := video.EstimateSegments(segments, params)

			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name")
	cmd.Flags().StringVar(&engine, "engine", "say", "TTS engine: say or gemini")
	cmd.Flags().IntVar(&wpm, "wpm", 0, "Words per minute (default: 200 for say, 150 for gemini)")
	cmd.Flags().BoolVar(&validate, "validate", false, "Validate timing and predict overlaps")

	return cmd
}
