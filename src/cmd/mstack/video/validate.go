package video

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/claytonharbour/proseforge-mstack/src/internal/validation"
	videosvc "github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/spf13/cobra"
)

func NewValidateCmd() *cobra.Command {
	var project string
	var configPath string
	var scriptPath string
	var outputJSON bool
	var framesOnly bool
	var ocrOnly bool
	var frames int
	var frameOffset int

	cmd := &cobra.Command{
		Use:   "validate <video-name>",
		Short: "Validate video content matches narration using OCR",
		Long: `Extracts frames from video at narration timestamps and uses OCR
to validate that on-screen content matches what the narration describes.

Supports inline JSON tags in narration.md for semantic validation:
  | 00:05 | Click Settings ` + "`" + `{"action":"click","target":"Settings"}` + "`" + ` |

Action types: click, fill, navigate, wait, select, hover, scroll, assert

Detects:
- Empty state mismatches (narration describes content but screen is empty)
- Missing UI elements (referenced buttons/links not visible)
- Click targets not visible (narration says "click X" but X not on screen)
- Timing mismatches (action happened before/after expected)

Examples:
  mstack video validate story-forge
  mstack video validate story-forge --frames 5 --frame-offset 750
  mstack video validate story-forge --frames-only   # Just extract frames
  mstack video validate story-forge --ocr-only      # Just do OCR on existing frames
  mstack video validate story-forge --script /path/to/story-forge.showcase.ts`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoName := args[0]

			if project == "" {
				project = defaultProject
			}

			// Get project directories
			inputDir, buildDir, _ := getProjectDirs(project)

			// Find video directory
			videoDir, videoFile, narrationFile, err := validateInput(videoName, inputDir)
			if err != nil {
				return fmt.Errorf("video not found: %w", err)
			}

			// Parse narration.md internally
			segments, err := videosvc.NewService().ParseNarrationMD(narrationFile)
			if err != nil {
				return fmt.Errorf("failed to parse narration.md: %w", err)
			}

			buildVideoDir := filepath.Join(buildDir, filepath.Base(videoDir))

			// Load config (optional)
			var config *validation.ValidationConfig
			if configPath != "" {
				config, err = validation.LoadConfig(configPath)
				if err != nil {
					return fmt.Errorf("failed to load config: %w", err)
				}
			} else {
				config = validation.DefaultConfig()
			}

			svc := validation.NewService()
			framesDir := filepath.Join(buildVideoDir, "frames")

			// Frames only mode
			if framesOnly {
				fmt.Printf("Extracting frames from %s...\n", videoFile)
				frames, err := svc.ExtractFrames(videoFile, segments, framesDir)
				if err != nil {
					return fmt.Errorf("frame extraction failed: %w", err)
				}
				fmt.Printf("Extracted %d frames to %s\n", len(frames), framesDir)

				if outputJSON {
					framesInfoPath := filepath.Join(buildVideoDir, "frames.json")
					if err := validation.SaveFramesInfo(frames, framesInfoPath); err != nil {
						return err
					}
					fmt.Printf("Saved frame info to %s\n", framesInfoPath)
				}
				return nil
			}

			// OCR only mode
			if ocrOnly {
				// Load existing frames
				framesInfoPath := filepath.Join(buildVideoDir, "frames.json")
				data, err := os.ReadFile(framesInfoPath)
				if err != nil {
					return fmt.Errorf("frames.json not found - run 'mstack video validate %s --frames-only' first", videoName)
				}

				var frames []validation.FrameInfo
				if err := json.Unmarshal(data, &frames); err != nil {
					return fmt.Errorf("failed to parse frames.json: %w", err)
				}

				fmt.Printf("Running OCR on %d frames...\n", len(frames))
				frames, err = svc.AnalyzeFrames(frames)
				if err != nil {
					return fmt.Errorf("OCR failed: %w", err)
				}

				if outputJSON {
					if err := validation.SaveFramesInfo(frames, framesInfoPath); err != nil {
						return err
					}
					fmt.Printf("Updated %s with OCR text\n", framesInfoPath)
				} else {
					// Print OCR results
					for _, f := range frames {
						fmt.Printf("\n=== Segment %d (%s) ===\n", f.SegmentIndex, f.Timestamp)
						fmt.Printf("OCR Text:\n%s\n", truncateText(f.OCRText, 500))
					}
				}
				return nil
			}

			// Record frame metric if explicitly passed
			if cmd.Flags().Changed("frames") {
				if err := validation.RecordFrameSample(frames); err != nil {
					fmt.Printf("Warning: could not record frame sample: %v\n", err)
				}
			}

			// Parse segments with inline tags
			taggedSegments := validation.ParseSegmentsWithTags(segments)
			hasTaggedSegments := false
			for _, ts := range taggedSegments {
				if ts.HasTag() {
					hasTaggedSegments = true
					break
				}
			}

			// Full validation
			fmt.Printf("Validating video: %s\n", videoName)
			fmt.Printf("  Video: %s\n", videoFile)
			fmt.Printf("  Segments: %d\n", len(segments))
			if hasTaggedSegments {
				fmt.Printf("  Tagged segments: semantic validation enabled\n")
				fmt.Printf("  Frames per segment: %d (offset: %dms)\n", frames, frameOffset)
			}
			if scriptPath != "" {
				fmt.Printf("  Script: %s\n", scriptPath)
			}
			fmt.Println()

			var result *validation.ValidationResult
			if hasTaggedSegments {
				// Use tagged segment validation
				result, err = svc.ValidateTaggedVideo(videoFile, taggedSegments, buildVideoDir, config, scriptPath, frameOffset)
			} else {
				// Use standard validation
				result, err = svc.ValidateVideoWithScript(videoFile, segments, buildVideoDir, config, scriptPath)
			}
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			// Output results
			if outputJSON {
				resultPath := filepath.Join(buildVideoDir, "validation.json")
				if err := validation.SaveResult(result, resultPath); err != nil {
					return err
				}
				fmt.Printf("Saved validation result to %s\n", resultPath)
			}

			// Print summary
			printValidationSummary(result)

			// Print issues
			if len(result.Issues) > 0 {
				fmt.Println("\n=== Issues Found ===")
				for i, issue := range result.Issues {
					printIssue(i+1, issue)
				}
			}

			if result.Summary.HighSeverity > 0 {
				return fmt.Errorf("%d high severity issues found", result.Summary.HighSeverity)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", defaultProject, "Project name")
	cmd.Flags().StringVar(&configPath, "config", "", "Path to validation config JSON")
	cmd.Flags().StringVar(&scriptPath, "script", "", "Path to Playwright test file (.showcase.ts) for line number mapping")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Save results to JSON file")
	cmd.Flags().BoolVar(&framesOnly, "frames-only", false, "Only extract frames, skip OCR and validation")
	cmd.Flags().BoolVar(&ocrOnly, "ocr-only", false, "Only run OCR on existing frames")
	cmd.Flags().IntVar(&frames, "frames", 3, "Number of frames to extract per tagged segment")
	cmd.Flags().IntVar(&frameOffset, "frame-offset", 500, "Milliseconds offset for post-action frame extraction")

	return cmd
}

func printValidationSummary(result *validation.ValidationResult) {
	fmt.Println("=== Validation Summary ===")
	fmt.Printf("  Video: %s\n", result.VideoName)
	fmt.Printf("  Frames: %s\n", result.FramesDir)
	fmt.Printf("  Segments checked: %d/%d\n", result.Summary.SegmentsChecked, result.Summary.TotalSegments)
	fmt.Println()
	fmt.Printf("  Issues: %d total\n", result.Summary.TotalIssues)
	if result.Summary.TotalIssues > 0 {
		fmt.Printf("    High:   %d\n", result.Summary.HighSeverity)
		fmt.Printf("    Medium: %d\n", result.Summary.MediumSeverity)
		fmt.Printf("    Low:    %d\n", result.Summary.LowSeverity)
	}
}

func printIssue(num int, issue validation.ValidationIssue) {
	severityColor := ""
	switch issue.Severity {
	case "high":
		severityColor = "\033[31m" // red
	case "medium":
		severityColor = "\033[33m" // yellow
	case "low":
		severityColor = "\033[36m" // cyan
	}
	reset := "\033[0m"

	fmt.Printf("\n%d. %s[%s]%s %s (Segment %d @ %s)\n",
		num, severityColor, strings.ToUpper(issue.Severity), reset,
		issue.Type, issue.SegmentIndex, issue.Timestamp)

	// Show test file location if available
	if issue.TestFile != "" && issue.TestLine > 0 {
		fmt.Printf("   Location: %s:%d\n", issue.TestFile, issue.TestLine)
	}

	fmt.Printf("   Narration: %s\n", truncateText(issue.NarrationText, 80))

	if issue.ScreenContent != "" {
		fmt.Printf("   Screen: %s\n", truncateText(issue.ScreenContent, 80))
	}
	if len(issue.ExpectedText) > 0 {
		fmt.Printf("   Expected: %v\n", issue.ExpectedText)
	}
	if len(issue.FoundText) > 0 {
		fmt.Printf("   Found: %v\n", issue.FoundText)
	}
	if issue.CodeContext != "" {
		fmt.Printf("   Code: %s\n", truncateText(issue.CodeContext, 100))
	}
	if issue.Suggestion != nil {
		fmt.Printf("   Suggestion: %s\n", issue.Suggestion.Description)
	}
}

func truncateText(text string, maxLen int) string {
	// Remove newlines for display
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Join(strings.Fields(text), " ")

	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}
