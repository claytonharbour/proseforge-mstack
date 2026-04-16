package video

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/spf13/cobra"
)

const defaultProject = "proseforge"

// log prints a formatted log message with color codes
func log(msg string, level string) {
	prefix := map[string]string{
		"info":  "\033[34m[INFO]\033[0m",
		"ok":    "\033[32m[OK]\033[0m",
		"warn":  "\033[33m[WARN]\033[0m",
		"error": "\033[31m[ERROR]\033[0m",
		"step":  "\033[36m[STEP]\033[0m",
	}[level]
	fmt.Printf("%s %s\n", prefix, msg)
}

// getProjectDirs returns project-specific directories as absolute paths
func getProjectDirs(project string) (inputDir, buildDir, stagingDir string) {
	inputDir, _ = filepath.Abs(filepath.Join("input", project))
	buildDir, _ = filepath.Abs(filepath.Join("build", project))
	stagingDir = filepath.Join(buildDir, "staging")
	return
}

// findVideoDir finds input directory matching video name
func findVideoDir(videoName, inputDir string) (string, error) {
	// First try exact match
	exact := filepath.Join(inputDir, videoName)
	if info, err := os.Stat(exact); err == nil && info.IsDir() {
		return exact, nil
	}

	// Try pattern match
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), videoName) {
			return filepath.Join(inputDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("video directory not found")
}

// validateInput validates input files exist
func validateInput(videoName, inputDir string) (videoDir, videoFile, narrationFile string, err error) {
	videoDir, err = findVideoDir(videoName, inputDir)
	if err != nil {
		return "", "", "", err
	}

	videoFile = filepath.Join(videoDir, "video.webm")
	narrationFile = filepath.Join(videoDir, "narration.md")

	var errors []string
	if _, err := os.Stat(videoFile); os.IsNotExist(err) {
		errors = append(errors, fmt.Sprintf("video.webm not found in %s", videoDir))
	}
	if _, err := os.Stat(narrationFile); os.IsNotExist(err) {
		errors = append(errors, fmt.Sprintf("narration.md not found in %s", videoDir))
	}

	if len(errors) > 0 {
		for _, e := range errors {
			log(e, "error")
		}
		return "", "", "", fmt.Errorf("validation failed")
	}

	return videoDir, videoFile, narrationFile, nil
}

func NewProcessCmd() *cobra.Command {
	var videoSvc = video.NewService()
	var project string
	var ttsEngine string
	var ttsModel string
	var upload bool
	var dryRun bool
	var ttsTimeoutStr string

	cmd := &cobra.Command{
		Use:   "process [video-name]",
		Short: "Process a video through the full narration pipeline",
		Long:  "Processes a video through parse -> TTS -> analyze -> build pipeline.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoName := args[0]
			if project == "" {
				project = defaultProject
			}
			resolved, err := video.ResolveEngine(ttsEngine, project)
			if err != nil {
				return err
			}
			ttsEngine = resolved

			log(fmt.Sprintf("Processing video: %s (project: %s)", videoName, project), "info")

			// Get project-specific directories (absolute paths)
			inputDir, buildDir, stagingDir := getProjectDirs(project)

			// Validate input
			log("Validating input files...", "step")
			videoDir, videoFile, narrationFile, err := validateInput(videoName, inputDir)
			if err != nil {
				log(fmt.Sprintf("Input directory not found for '%s'", videoName), "error")
				log(fmt.Sprintf("Available videos in %s:", filepath.Base(inputDir)), "info")
				if entries, err := os.ReadDir(inputDir); err == nil {
					for _, entry := range entries {
						if entry.IsDir() {
							if _, err := os.Stat(filepath.Join(inputDir, entry.Name(), "narration.md")); err == nil {
								fmt.Printf("  - %s\n", entry.Name())
							}
						}
					}
				}
				return err
			}
			log(fmt.Sprintf("Input directory: %s", filepath.Base(videoDir)), "ok")

			if dryRun {
				log("Dry run — would run full pipeline", "info")
				return nil
			}

			// Setup paths for RunPipeline
			buildVideoDir := filepath.Join(buildDir, filepath.Base(videoDir))
			outputFile := filepath.Join(buildVideoDir, fmt.Sprintf("%s.mp4", videoName))

			var ttsTimeout time.Duration
			if ttsTimeoutStr != "" {
				ttsTimeout, err = time.ParseDuration(ttsTimeoutStr)
				if err != nil {
					return fmt.Errorf("invalid --tts-timeout: %w", err)
				}
			}

			result, err := videoSvc.RunPipeline(video.PipelineOpts{
				NarrationPath: narrationFile,
				VideoPath:     videoFile,
				OutputPath:    outputFile,
				WorkingDir:    buildVideoDir,
				TTSEngine:     ttsEngine,
				TTSModel:      ttsModel,
				Force:         false,
				TTSTimeout:    ttsTimeout,
			})
			if err != nil {
				log(fmt.Sprintf("Pipeline failed: %v", err), "error")
				return err
			}

			log(fmt.Sprintf("Output: %s (%d segments, %d overlaps)", result.OutputPath, result.SegmentCount, result.Overlaps), "ok")

			// Copy to staging directory
			os.MkdirAll(stagingDir, 0755)
			shortName := videoName
			if idx := strings.Index(videoName, "."); idx != -1 {
				shortName = videoName[:idx]
			}
			stagedFile := filepath.Join(stagingDir, fmt.Sprintf("%s.mp4", shortName))

			src, err := os.Open(outputFile)
			if err != nil {
				log(fmt.Sprintf("Failed to open output file: %v", err), "error")
				return err
			}
			defer src.Close()

			dst, err := os.Create(stagedFile)
			if err != nil {
				log(fmt.Sprintf("Failed to create staged file: %v", err), "error")
				return err
			}
			defer dst.Close()

			if _, err := io.Copy(dst, src); err != nil {
				log(fmt.Sprintf("Failed to copy file: %v", err), "error")
				return err
			}

			log(fmt.Sprintf("Staged: %s", stagedFile), "ok")

			// Upload to YouTube (optional)
			if upload {
				log("Upload to YouTube not yet implemented in new CLI", "warn")
			}

			log(fmt.Sprintf("Pipeline complete for %s!", videoName), "ok")
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", defaultProject, fmt.Sprintf("Project name (default: %s)", defaultProject))
	cmd.Flags().StringVar(&ttsEngine, "tts-engine", "auto", "TTS engine: auto, say, gemini, cloudtts, or vertex")
	cmd.Flags().StringVar(&ttsModel, "tts-model", "", "Gemini model for TTS (default: gemini-2.5-flash-preview-tts)")
	cmd.Flags().BoolVar(&upload, "upload", false, "Upload to YouTube after processing")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview steps without executing")
	cmd.Flags().StringVar(&ttsTimeoutStr, "tts-timeout", "", "Overall TTS generation timeout (e.g. '10m', '30m'). Default: 10m")

	return cmd
}
