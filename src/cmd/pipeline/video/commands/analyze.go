package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/spf13/cobra"
)

func NewAnalyzeCmd() *cobra.Command {
	var videoSvc = video.NewService()
	var project string

	cmd := &cobra.Command{
		Use:   "analyze [video-name]",
		Short: "Analyze audio segment timing for overlaps",
		Long:  "Analyzes segments.json and audio files to detect timing overlaps and tight fits.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoName := args[0]
			if project == "" {
				project = "proseforge"
			}

			// Determine paths
			buildDir := filepath.Join("build", project, videoName)
			segmentsPath := filepath.Join(buildDir, "segments.json")

			// Fallback to direct build/<video-name>/ if project path doesn't exist
			if _, err := os.Stat(segmentsPath); os.IsNotExist(err) {
				buildDir = filepath.Join("build", videoName)
				segmentsPath = filepath.Join(buildDir, "segments.json")
			}

			audioDir := filepath.Join(buildDir, "audio")

			if _, err := os.Stat(segmentsPath); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Error: %s not found\n", segmentsPath)
				return fmt.Errorf("segments file not found")
			}

			results, err := videoSvc.AnalyzeOverlap(segmentsPath, audioDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			// Output JSON to stdout (pipeable)
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(results)
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name")

	return cmd
}
