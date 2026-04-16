package video

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
)

// GeminiTTSRequest represents the request to Gemini TTS API
type GeminiTTSRequest struct {
	Contents         []GeminiContent `json:"contents"`
	GenerationConfig GeminiConfig    `json:"generationConfig"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiConfig struct {
	ResponseModalities []string          `json:"responseModalities"`
	SpeechConfig       GeminiSpeechConfig `json:"speechConfig"`
}

type GeminiSpeechConfig struct {
	VoiceConfig GeminiVoiceConfig `json:"voiceConfig"`
}

type GeminiVoiceConfig struct {
	PrebuiltVoiceConfig GeminiPrebuiltVoiceConfig `json:"prebuiltVoiceConfig"`
}

type GeminiPrebuiltVoiceConfig struct {
	VoiceName string `json:"voiceName"`
}

// GeminiTTSResponse represents the response from Gemini TTS API
type GeminiTTSResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
}

type GeminiCandidate struct {
	Content GeminiContentResponse `json:"content"`
}

type GeminiContentResponse struct {
	Parts []GeminiPartResponse `json:"parts"`
}

type GeminiPartResponse struct {
	InlineData GeminiInlineData `json:"inlineData"`
}

type GeminiInlineData struct {
	Data string `json:"data"`
}

// saveWAVFile saves PCM audio data to a WAV file
func saveWAVFile(filename string, pcmData []byte, channels, rate, sampleWidth int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// WAV header
	fileSize := uint32(36 + len(pcmData))
	header := make([]byte, 44)

	// RIFF header
	copy(header[0:4], "RIFF")
	header[4] = byte(fileSize)
	header[5] = byte(fileSize >> 8)
	header[6] = byte(fileSize >> 16)
	header[7] = byte(fileSize >> 24)
	copy(header[8:12], "WAVE")

	// fmt chunk
	copy(header[12:16], "fmt ")
	header[16] = 16 // fmt chunk size
	header[20] = 1  // audio format (PCM)
	header[22] = byte(channels)
	header[23] = byte(channels >> 8)
	header[24] = byte(rate)
	header[25] = byte(rate >> 8)
	header[26] = byte(rate >> 16)
	header[27] = byte(rate >> 24)
	byteRate := rate * channels * sampleWidth
	header[28] = byte(byteRate)
	header[29] = byte(byteRate >> 8)
	header[30] = byte(byteRate >> 16)
	header[31] = byte(byteRate >> 24)
	header[32] = byte(channels * sampleWidth)
	header[33] = byte((channels * sampleWidth) >> 8)
	header[34] = byte(sampleWidth * 8)
	header[35] = byte((sampleWidth * 8) >> 8)

	// data chunk
	copy(header[36:40], "data")
	dataSize := uint32(len(pcmData))
	header[40] = byte(dataSize)
	header[41] = byte(dataSize >> 8)
	header[42] = byte(dataSize >> 16)
	header[43] = byte(dataSize >> 24)

	if _, err := file.Write(header); err != nil {
		return err
	}
	if _, err := file.Write(pcmData); err != nil {
		return err
	}

	return nil
}

// cloudTTSPrompt returns a brief style hint for the Cloud TTS prompt field.
// Cloud TTS treats prompt as context/style guidance, not a system instruction,
// so we keep it minimal — just pacing when requested.
func cloudTTSPrompt(rate int) string {
	if rate > 0 && rate > DefaultGeminiWPM {
		return fmt.Sprintf("Speak at approximately %d words per minute.", rate)
	}
	return ""
}

// generateSpeechResult carries metadata from a generateSpeech call alongside
// the primary error. RetryDelay and QuotaType are populated from the Gemini
// response body on 429, falling back to the Retry-After header.
type generateSpeechResult struct {
	RetryDelay time.Duration
	QuotaType  QuotaType
}

// generateTTSSay generates TTS audio using macOS say command
func generateTTSSay(opts TTSOpts) error {
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate audio for each segment
	for _, seg := range opts.Segments {
		out := filepath.Join(opts.OutputDir, fmt.Sprintf("segment_%03d.m4a", seg.Index))
		textPreview := seg.Text
		if len(textPreview) > 50 {
			textPreview = textPreview[:50] + "..."
		}

		// Skip if file already exists
		if _, err := os.Stat(out); err == nil {
			fmt.Printf("  Segment %02d: [exists] %s\n", seg.Index, textPreview)
			continue
		}

		fmt.Printf("  Segment %02d: %s\n", seg.Index, textPreview)

		// Run say command
		cmd := exec.Command("say",
			"-v", opts.Voice,
			"-r", fmt.Sprintf("%d", opts.WordsPerMinute),
			"--file-format=mp4f",
			"-o", out,
			seg.Text,
		)

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to generate audio for segment %d: %w", seg.Index, err)
		}
	}

	fmt.Printf("Generated %d audio files.\n", len(opts.Segments))

	// Update segments.json if requested
	if opts.UpdateJSON {
		updatedSegments := make([]types.Segment, len(opts.Segments))
		copy(updatedSegments, opts.Segments)
		for i := range updatedSegments {
			updatedSegments[i].AudioFile = strings.ReplaceAll(updatedSegments[i].AudioFile, ".mp3", ".m4a")
			updatedSegments[i].AudioFile = strings.ReplaceAll(updatedSegments[i].AudioFile, ".wav", ".m4a")
		}

		updatedData, err := json.MarshalIndent(updatedSegments, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal segments: %w", err)
		}

		if err := os.WriteFile(opts.SegmentsFile, updatedData, 0644); err != nil {
			return fmt.Errorf("failed to write segments file: %w", err)
		}

		fmt.Printf("Updated %s\n", opts.SegmentsFile)
	}

	return nil
}

// generateTTSAPI generates TTS audio using a TTSProvider (AI Studio, Cloud TTS, or Vertex AI)
// with automatic model failover on rate limits.
func generateTTSAPI(opts TTSOpts) error {
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	provider, err := resolveProvider(opts)
	if err != nil {
		return err
	}

	ext := provider.AudioExtension()

	// Resolve models: use discovery for AI Studio, static defaults for others
	var models []string
	if opts.Engine == "gemini" {
		apiKey := os.Getenv("GOOGLE_AI_API_KEY")
		models = ResolveTTSModels(apiKey)
	} else {
		models = provider.DefaultModels()
	}

	// If caller specified a model, put it first
	if preferred := opts.Model; preferred != "" {
		reordered := []string{preferred}
		for _, m := range models {
			if m != preferred {
				reordered = append(reordered, m)
			}
		}
		models = reordered
	} else if envModel := os.Getenv("GEMINI_TTS_MODEL"); envModel != "" && opts.Engine == "gemini" {
		reordered := []string{envModel}
		for _, m := range models {
			if m != envModel {
				reordered = append(reordered, m)
			}
		}
		models = reordered
	}

	// Pre-flight quota check (AI Studio only — Cloud TTS and Vertex don't need it)
	if opts.Engine == "gemini" {
		apiKey := os.Getenv("GOOGLE_AI_API_KEY")
		fmt.Println("  Checking model quota...")
		var usable []string
		for _, m := range models {
			if ok, _ := CheckModelQuota(apiKey, m, opts.Verbose); ok {
				usable = append(usable, m)
				fmt.Printf("  Model %s: quota available\n", m)
				break
			}
			fmt.Printf("  Model %s: no quota, skipping\n", m)
		}
		if len(usable) == 0 {
			return fmt.Errorf("no TTS models available (all returned limit: 0 or failed quota check)")
		}
		// Add remaining untested models as failover candidates
		for _, m := range models {
			if m != usable[0] {
				usable = append(usable, m)
			}
		}
		models = usable
	}

	// Apply default timeout
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	deadline := time.Now().Add(timeout)

	pool := NewModelPool(models)
	sPath := statsPath(opts.OutputDir)

	const maxAttemptsPerSegment = 12
	const maxRetriesPerCall = 2

	// Generate audio for each segment
	for _, seg := range opts.Segments {
		if time.Now().After(deadline) {
			return fmt.Errorf("TTS generation timed out after %v", timeout)
		}

		out := filepath.Join(opts.OutputDir, fmt.Sprintf("segment_%03d%s", seg.Index, ext))
		textPreview := seg.Text
		if len(textPreview) > 50 {
			textPreview = textPreview[:50] + "..."
		}

		// Skip if file already exists
		if _, err := os.Stat(out); err == nil {
			fmt.Printf("  Segment %02d: [exists] %s\n", seg.Index, textPreview)
			continue
		}

		var generated bool
		attempts := 0
		segStart := time.Now()
		var lastModel string
		for !generated {
			if time.Now().After(deadline) {
				return fmt.Errorf("TTS generation timed out after %v", timeout)
			}

			chosenModel, ok := pool.PickModel()
			if !ok {
				earliest := pool.EarliestAvailable()
				wait := time.Until(earliest)
				if wait > 0 {
					if remaining := time.Until(deadline); wait > remaining {
						wait = remaining
					}
					wait = addJitter(wait, 2*time.Second)
					fmt.Printf("    All models on cooldown, waiting %v...\n", wait)
					time.Sleep(wait)
				}
				continue
			}
			lastModel = chosenModel

			fmt.Printf("  Segment %02d [%s]: %s\n", seg.Index, chosenModel, textPreview)

			callStart := time.Now()
			result, err := provider.GenerateAudio(ProviderOpts{
				Text:           seg.Text,
				OutputPath:     out,
				Voice:          opts.Voice,
				Model:          chosenModel,
				WordsPerMinute: opts.WordsPerMinute,
				MaxRetries:     maxRetriesPerCall,
				Verbose:        opts.Verbose,
			})
			callLatency := time.Since(callStart)

			if err == nil {
				pool.MarkAvailable(chosenModel)
				generated = true

				// Write success stats
				_ = appendStats(sPath, TTSStats{
					Timestamp:  time.Now(),
					Provider:   provider.Name(),
					Model:      chosenModel,
					Segment:    seg.Index,
					TextLen:    len(seg.Text),
					AudioFile:  filepath.Base(out),
					DurationMS: audioDurationMS(out),
					LatencyMS:  callLatency.Milliseconds(),
					Retries:    attempts,
					Status:     "ok",
				})
				continue
			}

			// Check if it's a rate-limit error
			if _, isRL := err.(*TTSRateLimitError); isRL {
				var cooldown time.Duration
				var quotaType QuotaType
				if result != nil {
					cooldown = result.RetryDelay
					quotaType = result.QuotaType
				}

				if quotaType == QuotaPerDay {
					pool.MarkRateLimited(chosenModel, 24*time.Hour)
					fmt.Printf("    Model %s daily quota exhausted, removed from rotation\n", chosenModel)
				} else {
					pool.MarkRateLimited(chosenModel, cooldown)
					fmt.Printf("    Model %s rate limited (quota=%d, cooldown=%v), failing over...\n", chosenModel, quotaType, cooldown)
				}

				attempts++
				if attempts >= maxAttemptsPerSegment {
					// Write failure stats
					_ = appendStats(sPath, TTSStats{
						Timestamp:  time.Now(),
						Provider:   provider.Name(),
						Model:      chosenModel,
						Segment:    seg.Index,
						TextLen:    len(seg.Text),
						AudioFile:  filepath.Base(out),
						LatencyMS:  time.Since(segStart).Milliseconds(),
						Retries:    attempts,
						Status:     "error",
						Error:      err.Error(),
					})
					return fmt.Errorf("failed to generate audio for segment %d: %w",
						seg.Index, &TTSRateLimitError{
							StatusCode:       429,
							Model:            chosenModel,
							ModelsTriedCount: len(models),
						})
				}
				continue
			}

			// Non-rate-limit errors fail immediately
			_ = appendStats(sPath, TTSStats{
				Timestamp:  time.Now(),
				Provider:   provider.Name(),
				Model:      lastModel,
				Segment:    seg.Index,
				TextLen:    len(seg.Text),
				AudioFile:  filepath.Base(out),
				LatencyMS:  callLatency.Milliseconds(),
				Retries:    attempts,
				Status:     "error",
				Error:      err.Error(),
			})
			return fmt.Errorf("failed to generate audio for segment %d: %w", seg.Index, err)
		}

		// Inter-segment delay with jitter
		time.Sleep(addJitter(5*time.Second, 2*time.Second))
	}

	fmt.Printf("Generated %d audio files.\n", len(opts.Segments))

	// Update segments.json if requested
	if opts.UpdateJSON {
		updatedSegments := make([]types.Segment, len(opts.Segments))
		copy(updatedSegments, opts.Segments)

		// Determine the target extension for updates
		targetExt := ext // e.g. ".wav" or ".mp3"
		otherExts := []string{".mp3", ".wav", ".m4a"}
		for i := range updatedSegments {
			for _, oldExt := range otherExts {
				if oldExt != targetExt {
					updatedSegments[i].AudioFile = strings.ReplaceAll(updatedSegments[i].AudioFile, oldExt, targetExt)
				}
			}
		}

		updatedData, err := json.MarshalIndent(updatedSegments, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal segments: %w", err)
		}

		if err := os.WriteFile(opts.SegmentsFile, updatedData, 0644); err != nil {
			return fmt.Errorf("failed to write segments file: %w", err)
		}

		fmt.Printf("Updated %s\n", opts.SegmentsFile)
	}

	return nil
}

// generateTTSGemini is kept as an alias for backward compatibility.
func generateTTSGemini(opts TTSOpts) error {
	return generateTTSAPI(opts)
}
