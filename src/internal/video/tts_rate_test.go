//go:build calibration

package video

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGeminiPaceCalibration measures how different pacing prompts affect Gemini
// TTS output duration. Run manually — makes real API calls.
//
//	cd src && go test -tags=calibration -run TestGeminiPaceCalibration -count=1 ./internal/video/ -v -timeout 120s
func TestGeminiPaceCalibration(t *testing.T) {
	if os.Getenv("GOOGLE_AI_API_KEY") == "" {
		t.Skip("GOOGLE_AI_API_KEY not set")
	}

	const testText = "Welcome to ProseForge, your creative writing companion. " +
		"In this tutorial we will walk through the story creation wizard, " +
		"explore the AI-powered review system, and see how to publish your finished work. " +
		"Let's get started by clicking the Create Story button on the dashboard."

	wordCount := len(strings.Fields(testText))

	variants := []struct {
		name   string
		prefix string
	}{
		{"baseline", ""},
		{"120wpm", "Speak at approximately 120 words per minute. "},
		{"150wpm", "Speak at approximately 150 words per minute. "},
		{"180wpm", "Speak at approximately 180 words per minute. "},
		{"brisk", "Speak at a brisk, energetic pace. "},
		{"quick", "Speak quickly and clearly. "},
	}

	model := "gemini-2.5-flash-preview-tts"
	voice := "Kore"

	type result struct {
		name       string
		durationS  float64
		wpm        float64
		speedup    float64
		err        error
	}

	results := make([]result, len(variants))
	var baselineDuration float64

	for i, v := range variants {
		text := v.prefix + testText
		outPath := filepath.Join(t.TempDir(), fmt.Sprintf("calibration_%d.wav", i))

		t.Logf("Generating variant %d/%d: %s", i+1, len(variants), v.name)

		_, err := generateSpeech(text, outPath, voice, model, 0, 3)
		if err != nil {
			results[i] = result{name: v.name, err: err}
			continue
		}

		// Read WAV and compute duration from data chunk size.
		// Format: 24kHz mono 16-bit PCM → bytes_per_sec = 24000 * 2 = 48000
		data, err := os.ReadFile(outPath)
		if err != nil {
			results[i] = result{name: v.name, err: err}
			continue
		}

		// WAV header is 44 bytes; data chunk starts after header
		if len(data) < 44 {
			results[i] = result{name: v.name, err: fmt.Errorf("WAV too small: %d bytes", len(data))}
			continue
		}
		dataBytes := len(data) - 44
		durationS := float64(dataBytes) / (24000 * 2)
		effectiveWPM := (float64(wordCount) / durationS) * 60

		if i == 0 {
			baselineDuration = durationS
		}

		speedup := 1.0
		if baselineDuration > 0 {
			speedup = baselineDuration / durationS
		}

		results[i] = result{
			name:      v.name,
			durationS: durationS,
			wpm:       effectiveWPM,
			speedup:   speedup,
		}
	}

	// Print table
	t.Logf("\nWord count: %d\n", wordCount)
	t.Logf("%-12s | %10s | %10s | %10s | %s", "Prompt", "Duration", "WPM", "Speedup", "Error")
	t.Logf("%-12s-+-%10s-+-%10s-+-%10s-+-%s", "------------", "----------", "----------", "----------", "-----")
	for _, r := range results {
		if r.err != nil {
			t.Logf("%-12s | %10s | %10s | %10s | %v", r.name, "-", "-", "-", r.err)
		} else {
			t.Logf("%-12s | %9.2fs | %8.1f | %9.2fx |", r.name, r.durationS, r.wpm, r.speedup)
		}
	}
}
