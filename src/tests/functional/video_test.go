package functional

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/claytonharbour/proseforge-mstack/src/internal/config"
	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
)

var (
	projectRoot string
	mstackBin   string
	fixturesDir string
)

func init() {
	config.Load()

	// Find project root (3 levels up from src/tests/functional)
	wd, _ := os.Getwd()
	projectRoot = filepath.Join(wd, "..", "..", "..")
	mstackBin = filepath.Join(projectRoot, "bin", "mstack")
	fixturesDir = filepath.Join(projectRoot, "src", "tests", "fixtures")
}

func ensureMstackBuilt(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(mstackBin); os.IsNotExist(err) {
		t.Log("Building mstack binary...")
		cmd := exec.Command("make", "build")
		cmd.Dir = projectRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Failed to build mstack: %v\n%s", err, out)
		}
	}
}

func TestMstackHelp(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mstack --help failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "video") {
		t.Error("Expected 'video' command in help output")
	}
	if !strings.Contains(output, "social") {
		t.Error("Expected 'social' command in help output")
	}
	if !strings.Contains(output, "secrets") {
		t.Error("Expected 'secrets' command in help output")
	}
}

func TestMstackVersion(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mstack version failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "mstack version") {
		t.Errorf("Expected 'mstack version' in output, got: %s", output)
	}
}

func TestVideoParse(t *testing.T) {
	ensureMstackBuilt(t)

	narrationFile := filepath.Join(fixturesDir, "narration.md")

	cmd := exec.Command(mstackBin, "video", "parse", narrationFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mstack video parse failed: %v\n%s", err, out)
	}

	// Parse the output as JSON
	var segments []types.Segment
	if err := json.Unmarshal(out, &segments); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, out)
	}

	// Verify we got the expected number of segments
	if len(segments) != 5 {
		t.Errorf("Expected 5 segments, got %d", len(segments))
	}

	// Verify first segment
	if len(segments) > 0 {
		first := segments[0]
		if first.Index != 1 {
			t.Errorf("Expected first segment index 1, got %d", first.Index)
		}
		if first.TimestampMs != 2000 {
			t.Errorf("Expected first segment time 2000ms, got %d", first.TimestampMs)
		}
		if !strings.Contains(first.Text, "Welcome") {
			t.Errorf("Expected first segment to contain 'Welcome', got: %s", first.Text)
		}
	}

	// Verify last segment
	if len(segments) > 4 {
		last := segments[4]
		if last.TimestampMs != 30000 {
			t.Errorf("Expected last segment time 30000ms, got %d", last.TimestampMs)
		}
		if !strings.Contains(last.Text, "Thank you") {
			t.Errorf("Expected last segment to contain 'Thank you', got: %s", last.Text)
		}
	}
}

func TestVideoSubcommandHelp(t *testing.T) {
	ensureMstackBuilt(t)

	testCases := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "video help",
			args:     []string{"video", "--help"},
			expected: []string{"parse", "analyze", "build", "tts", "process"},
		},
		{
			name:     "video tts help",
			args:     []string{"video", "tts", "--help"},
			expected: []string{"say", "gemini"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(mstackBin, tc.args...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Command failed: %v\n%s", err, out)
			}

			output := string(out)
			for _, exp := range tc.expected {
				if !strings.Contains(output, exp) {
					t.Errorf("Expected '%s' in output, got: %s", exp, output)
				}
			}
		})
	}
}

func TestVideoParseInvalidFile(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "parse", "/nonexistent/file.md")
	out, err := cmd.CombinedOutput()

	// Should fail with an error
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "error") && !strings.Contains(strings.ToLower(output), "failed") {
		t.Errorf("Expected error message in output, got: %s", output)
	}
}

// ============ Duration Estimation Tests ============

func TestVideoEstimateText(t *testing.T) {
	ensureMstackBuilt(t)

	testCases := []struct {
		name         string
		text         string
		engine       string
		expectedWPM  int
		expectedWords int
	}{
		{
			name:          "simple text with say engine",
			text:          "Enter your email address to sign in",
			engine:        "say",
			expectedWPM:   200,
			expectedWords: 7,
		},
		{
			name:          "simple text with gemini engine",
			text:          "Hello world",
			engine:        "gemini",
			expectedWPM:   96,
			expectedWords: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"video", "estimate", "text", tc.text}
			if tc.engine != "say" {
				args = append(args, "--engine="+tc.engine)
			}

			cmd := exec.Command(mstackBin, args...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Command failed: %v\n%s", err, out)
			}

			var result map[string]interface{}
			if err := json.Unmarshal(out, &result); err != nil {
				t.Fatalf("Failed to parse JSON: %v\nOutput: %s", err, out)
			}

			// Verify word count
			if wordCount, ok := result["word_count"].(float64); ok {
				if int(wordCount) != tc.expectedWords {
					t.Errorf("Expected word_count %d, got %d", tc.expectedWords, int(wordCount))
				}
			} else {
				t.Error("Missing word_count in response")
			}

			// Verify WPM
			if wpm, ok := result["words_per_minute"].(float64); ok {
				if int(wpm) != tc.expectedWPM {
					t.Errorf("Expected words_per_minute %d, got %d", tc.expectedWPM, int(wpm))
				}
			} else {
				t.Error("Missing words_per_minute in response")
			}

			// Verify estimated_ms is present and reasonable
			if estimatedMs, ok := result["estimated_ms"].(float64); ok {
				// Formula: (words / wpm) * 60 * 1000
				expectedMs := (float64(tc.expectedWords) / float64(tc.expectedWPM)) * 60 * 1000
				if int(estimatedMs) != int(expectedMs) {
					t.Errorf("Expected estimated_ms %d, got %d", int(expectedMs), int(estimatedMs))
				}
			} else {
				t.Error("Missing estimated_ms in response")
			}
		})
	}
}

func TestVideoEstimateNarration(t *testing.T) {
	ensureMstackBuilt(t)

	// Note: Full narration estimation requires proper project structure
	// We test the command exists and basic text estimation works

	// Test estimate text command works
	cmd := exec.Command(mstackBin, "video", "estimate", "text", "This is a test narration segment")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Estimate text command failed: %v\n%s", err, out)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if _, ok := result["estimated_ms"]; !ok {
		t.Error("Missing estimated_ms in response")
	}

	// Test with custom WPM
	cmd = exec.Command(mstackBin, "video", "estimate", "text", "Test", "--wpm=250")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Estimate text with custom WPM failed: %v\n%s", err, out)
	}

	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if wpm, ok := result["words_per_minute"].(float64); ok {
		if int(wpm) != 250 {
			t.Errorf("Expected WPM 250, got %d", int(wpm))
		}
	}

	// Verify help includes estimate command
	cmd = exec.Command(mstackBin, "video", "estimate", "--help")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Help command failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "text") {
		t.Error("Expected 'text' subcommand in help")
	}
	if !strings.Contains(output, "narration") {
		t.Error("Expected 'narration' subcommand in help")
	}
}

// ============ Gemini TTS Functional Tests ============

func TestGeminiTTSSingleSegment(t *testing.T) {
	apiKey := os.Getenv("GOOGLE_AI_API_KEY")
	if apiKey == "" {
		t.Skip("GOOGLE_AI_API_KEY not set — skipping Gemini TTS functional test")
	}

	ensureMstackBuilt(t)

	// Create a temp dir with a single-segment segments.json
	tmpDir := t.TempDir()
	audioDir := filepath.Join(tmpDir, "audio")

	segments := []types.Segment{
		{
			Index:       1,
			TimestampMs: 1000,
			Text:        "Hello, this is a test of the Gemini text to speech pipeline.",
			AudioFile:   "segment_001.wav",
		},
	}

	segmentsJSON, err := json.MarshalIndent(segments, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal segments: %v", err)
	}

	segmentsFile := filepath.Join(tmpDir, "segments.json")
	if err := os.WriteFile(segmentsFile, segmentsJSON, 0644); err != nil {
		t.Fatalf("Failed to write segments.json: %v", err)
	}

	// Run: mstack video tts gemini segments.json --output-dir=audio --verbose
	cmd := exec.Command(mstackBin, "video", "tts", "gemini", segmentsFile,
		"--output-dir="+audioDir, "--verbose")
	cmd.Env = append(os.Environ(), "GOOGLE_AI_API_KEY="+apiKey)

	out, err := cmd.CombinedOutput()
	output := string(out)

	if err == nil {
		// Success path: audio file should exist and be non-empty
		audioFile := filepath.Join(audioDir, "segment_001.wav")
		info, statErr := os.Stat(audioFile)
		if statErr != nil {
			t.Fatalf("TTS succeeded but audio file missing: %v\nOutput: %s", statErr, output)
		}
		if info.Size() == 0 {
			t.Fatalf("TTS succeeded but audio file is empty\nOutput: %s", output)
		}
		t.Logf("TTS succeeded: %s (%d bytes)", audioFile, info.Size())
		return
	}

	// Failure path: if 429, stderr should contain parseable retry info
	t.Logf("TTS command failed (expected if rate limited):\n%s", output)

	// The verbose flag should have dumped the raw response body
	if strings.Contains(output, "[verbose]") {
		t.Log("Verbose output captured — raw response body available for debugging")
	}

	// Check that we see retry/quota info in the output (not zeros)
	if strings.Contains(output, "quota=0, retryDelay=0s") {
		t.Error("Parser returned zeros — the real Gemini 429 response format doesn't match our parser. " +
			"This is the bug we're trying to catch. Check [verbose] output above for the actual response body.")
	}
}

// ============ YouTube Quota Tests ============

// ============ Thumbnail Tests ============

func TestVideoThumbnailHelp(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "thumbnail", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Help command failed: %v\n%s", err, out)
	}

	output := string(out)
	expectedFlags := []string{"--timestamp", "--auto", "--width", "--height", "--quality", "--output"}
	for _, flag := range expectedFlags {
		if !strings.Contains(output, flag) {
			t.Errorf("Expected '%s' flag in help", flag)
		}
	}

	// Verify default values mentioned
	if !strings.Contains(output, "1280") {
		t.Error("Expected default width 1280 in help")
	}
	if !strings.Contains(output, "720") {
		t.Error("Expected default height 720 in help")
	}
}

func TestVideoThumbnailInvalidFile(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "thumbnail", "/nonexistent/video.mp4", "--timestamp=5")
	out, err := cmd.CombinedOutput()

	// Should fail with an error
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "error") && !strings.Contains(strings.ToLower(output), "not found") {
		t.Errorf("Expected error message in output, got: %s", output)
	}
}

func TestVideoCommandsIncludeNewFeatures(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Help command failed: %v\n%s", err, out)
	}

	output := string(out)
	newCommands := []string{"estimate", "thumbnail"}
	for _, newCmd := range newCommands {
		if !strings.Contains(output, newCmd) {
			t.Errorf("Expected '%s' command in video help", newCmd)
		}
	}
}

// ============ Video Split Tests ============

func TestVideoSplitHelp(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "split", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Help command failed: %v\n%s", err, out)
	}

	output := string(out)
	expectedFlags := []string{"--at", "--output-dir", "--reencode"}
	for _, flag := range expectedFlags {
		if !strings.Contains(output, flag) {
			t.Errorf("Expected '%s' flag in help", flag)
		}
	}
}

func TestVideoSplitInvalidFile(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "split", "/nonexistent/video.mp4", "--at=30,60")
	out, err := cmd.CombinedOutput()

	// Should fail with an error
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "error") && !strings.Contains(strings.ToLower(output), "not found") {
		t.Errorf("Expected error message in output, got: %s", output)
	}
}

func TestVideoSplitMissingTimestamps(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "split", "/some/video.mp4")
	out, err := cmd.CombinedOutput()

	// Should fail - missing --at flag
	if err == nil {
		t.Error("Expected error for missing timestamps")
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "--at") || !strings.Contains(strings.ToLower(output), "required") {
		t.Errorf("Expected error about missing --at flag, got: %s", output)
	}
}

// ============ Video Join Tests ============

func TestVideoJoinHelp(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "join", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Help command failed: %v\n%s", err, out)
	}

	output := string(out)
	expectedFlags := []string{"--output", "--reencode"}
	for _, flag := range expectedFlags {
		if !strings.Contains(output, flag) {
			t.Errorf("Expected '%s' flag in help", flag)
		}
	}
}

func TestVideoJoinInvalidFile(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "join", "/nonexistent/video1.mp4", "/nonexistent/video2.mp4", "--output=out.mp4")
	out, err := cmd.CombinedOutput()

	// Should fail with an error
	if err == nil {
		t.Error("Expected error for nonexistent files")
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "error") && !strings.Contains(strings.ToLower(output), "not found") {
		t.Errorf("Expected error message in output, got: %s", output)
	}
}

func TestVideoJoinMissingOutput(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "join", "video1.mp4", "video2.mp4")
	out, err := cmd.CombinedOutput()

	// Should fail - missing --output flag
	if err == nil {
		t.Error("Expected error for missing output")
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "--output") || !strings.Contains(strings.ToLower(output), "required") {
		t.Errorf("Expected error about missing --output flag, got: %s", output)
	}
}

// ============ Video Trim Tests ============

func TestVideoTrimHelp(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "trim", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Help command failed: %v\n%s", err, out)
	}

	output := string(out)
	expectedFlags := []string{"--start", "--end", "--auto", "--preview", "--reencode", "--output"}
	for _, flag := range expectedFlags {
		if !strings.Contains(output, flag) {
			t.Errorf("Expected '%s' flag in help", flag)
		}
	}
}

func TestVideoTrimInvalidFile(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "trim", "/nonexistent/video.mp4", "--start=2000")
	out, err := cmd.CombinedOutput()

	// Should fail with an error
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "error") && !strings.Contains(strings.ToLower(output), "not found") {
		t.Errorf("Expected error message in output, got: %s", output)
	}
}

func TestVideoTrimMissingOptions(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "trim", "/some/video.mp4")
	out, err := cmd.CombinedOutput()

	// Should fail - need either --auto or --start/--end
	if err == nil {
		t.Error("Expected error for missing trim options")
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "--auto") || !strings.Contains(strings.ToLower(output), "--start") {
		t.Errorf("Expected error about missing trim options, got: %s", output)
	}
}

// ============ YouTube Batch Upload Tests ============

func TestYouTubeBatchHelp(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "youtube", "batch", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Help command failed: %v\n%s", err, out)
	}

	output := string(out)
	expectedFlags := []string{"--dry-run", "--resume"}
	for _, flag := range expectedFlags {
		if !strings.Contains(output, flag) {
			t.Errorf("Expected '%s' flag in help", flag)
		}
	}

	// Check for manifest format example
	if !strings.Contains(output, "manifest") {
		t.Error("Expected 'manifest' mentioned in help")
	}
}

func TestYouTubeBatchInvalidManifest(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "youtube", "batch", "/nonexistent/manifest.json")
	out, err := cmd.CombinedOutput()

	// Should fail with an error
	if err == nil {
		t.Error("Expected error for nonexistent manifest")
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "error") {
		t.Errorf("Expected error message in output, got: %s", output)
	}
}

// ============ Nice-to-Have Commands in Help ============

func TestVideoCommandsIncludeNiceToHave(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Help command failed: %v\n%s", err, out)
	}

	output := string(out)
	niceToHaveCommands := []string{"split", "join", "trim"}
	for _, cmd := range niceToHaveCommands {
		if !strings.Contains(output, cmd) {
			t.Errorf("Expected '%s' command in video help", cmd)
		}
	}
}

func TestYouTubeCommandsIncludeBatch(t *testing.T) {
	ensureMstackBuilt(t)

	cmd := exec.Command(mstackBin, "video", "youtube", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Help command failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "batch") {
		t.Error("Expected 'batch' command in youtube help")
	}
}
