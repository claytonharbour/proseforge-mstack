package video

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/spf13/cobra"
)

// processVideo processes a single video through the pipeline
func processVideo(videoSvc video.Service, videoName, project, ttsEngine, ttsModel string, upload, dryRun bool) error {
	log(fmt.Sprintf("Processing video: %s (project: %s)", videoName, project), "info")

	// Get project-specific directories (absolute paths)
	inputDir, buildDir, stagingDir := getProjectDirs(project)

	// Validate input
	log("Validating input files...", "step")
	videoDir, videoFile, narrationFile, err := validateInput(videoName, inputDir)
	if err != nil {
		log(fmt.Sprintf("Input directory not found for '%s'", videoName), "error")
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

	result, err := videoSvc.RunPipeline(video.PipelineOpts{
		NarrationPath: narrationFile,
		VideoPath:     videoFile,
		OutputPath:    outputFile,
		WorkingDir:    buildVideoDir,
		TTSEngine:     ttsEngine,
		TTSModel:      ttsModel,
		Force:         false,
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
}

func NewProcessAllCmd() *cobra.Command {
	var videoSvc = video.NewService()
	var project string
	var ttsEngine string
	var ttsModel string
	var upload bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "process-all",
		Short: "Process all videos in the input directory",
		Long:  "Finds all videos with both video.webm and narration.md and processes them.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}
			resolved, err := video.ResolveEngine(ttsEngine, project)
			if err != nil {
				return err
			}
			ttsEngine = resolved

			inputDir, _ := filepath.Abs(filepath.Join("input", project))
			entries, err := os.ReadDir(inputDir)
			if err != nil {
				return fmt.Errorf("failed to read input directory: %w", err)
			}

			videos := []string{}
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}

				videoPath := filepath.Join(inputDir, entry.Name(), "video.webm")
				narrationPath := filepath.Join(inputDir, entry.Name(), "narration.md")

				hasVideo := false
				hasNarration := false

				if info, err := os.Stat(videoPath); err == nil && !info.IsDir() {
					hasVideo = true
				}
				if info, err := os.Stat(narrationPath); err == nil && !info.IsDir() {
					hasNarration = true
				}

				if hasVideo && hasNarration {
					videos = append(videos, entry.Name())
				}
			}

			if len(videos) == 0 {
				fmt.Fprintf(os.Stderr, "No videos found in input/ directory\n")
				return nil
			}

			fmt.Printf("Found %d video(s) to process:\n", len(videos))
			for _, v := range videos {
				fmt.Printf("  - %s\n", v)
			}
			fmt.Println()

			if dryRun {
				fmt.Println("Dry run mode - previewing steps only")
				fmt.Println()
			}

			processed := 0
			failedVideos := []string{}

			for i, videoName := range videos {
				fmt.Printf("\n%s\n", strings.Repeat("=", 60))
				fmt.Printf("Processing video %d/%d: %s\n", i+1, len(videos), videoName)
				fmt.Printf("%s\n", strings.Repeat("=", 60))

				err := processVideo(videoSvc, videoName, project, ttsEngine, ttsModel, upload, dryRun)
				if err != nil {
					log(fmt.Sprintf("Failed to process %s: %v", videoName, err), "error")
					failedVideos = append(failedVideos, videoName)
				} else {
					processed++
				}
			}

			// Summary
			fmt.Printf("\n%s\n", strings.Repeat("=", 60))
			fmt.Println("SUMMARY")
			fmt.Printf("%s\n", strings.Repeat("=", 60))
			fmt.Printf("  Total videos: %d\n", len(videos))
			fmt.Printf("  Processed: %d\n", processed)
			fmt.Printf("  Failed: %d\n", len(failedVideos))
			if len(failedVideos) > 0 {
				fmt.Println("  Failed videos:")
				for _, v := range failedVideos {
					fmt.Printf("    - %s\n", v)
				}
			}

			if len(failedVideos) > 0 {
				return fmt.Errorf("%d video(s) failed to process", len(failedVideos))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", defaultProject, fmt.Sprintf("Project name (default: %s)", defaultProject))
	cmd.Flags().StringVar(&ttsEngine, "tts-engine", "auto", "TTS engine: auto, say, gemini, cloudtts, or vertex")
	cmd.Flags().StringVar(&ttsModel, "tts-model", "", "Gemini model for TTS (default: gemini-2.5-flash-preview-tts)")
	cmd.Flags().BoolVar(&upload, "upload", false, "Upload each video to YouTube after processing")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview steps without executing")

	return cmd
}
