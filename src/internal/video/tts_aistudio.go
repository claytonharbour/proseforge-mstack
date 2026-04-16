package video

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// AIStudioProvider implements TTSProvider for the Google AI Studio (generativelanguage) API.
type AIStudioProvider struct {
	APIKey string
}

func (p *AIStudioProvider) Name() string { return "aistudio" }

func (p *AIStudioProvider) AudioExtension() string { return ".wav" }

func (p *AIStudioProvider) DefaultModels() []string { return DefaultTTSModels }

func (p *AIStudioProvider) GenerateAudio(opts ProviderOpts) (*ProviderResult, error) {
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		modelDiscoveryBaseURL, opts.Model, p.APIKey)

	reqBody := GeminiTTSRequest{
		Contents: []GeminiContent{
			{Parts: []GeminiPart{{Text: opts.Text}}},
		},
		GenerationConfig: GeminiConfig{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig: GeminiSpeechConfig{
				VoiceConfig: GeminiVoiceConfig{
					PrebuiltVoiceConfig: GeminiPrebuiltVoiceConfig{
						VoiceName: opts.Voice,
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	client := newHTTPClient(30 * time.Second)

	var lastRetryInfo GeminiRetryInfo
	for attempt := 0; attempt < opts.MaxRetries; attempt++ {
		verboseLog(opts.Verbose, "[verbose] POST %s", redactAPIKey(url))
		verboseLog(opts.Verbose, "[verbose] Request: %s", string(jsonData))

		req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()
		latency := time.Since(start)

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		verboseLog(opts.Verbose, "[verbose] HTTP %d (%dms)", resp.StatusCode, latency.Milliseconds())
		if opts.Verbose {
			respPreview := string(body)
			if len(respPreview) > 200 {
				respPreview = respPreview[:200] + "..."
			}
			verboseLog(true, "[verbose] Response: %s", respPreview)
		}

		if resp.StatusCode == 429 || resp.StatusCode == 500 || resp.StatusCode == 503 {
			// Parse Gemini response body for retry info
			lastRetryInfo = parseGeminiRetryInfo(body)

			// Fall back to Retry-After header if body yields nothing
			if lastRetryInfo.RetryDelay == 0 {
				if headerSec := parseRetryAfter(resp); headerSec > 0 {
					lastRetryInfo.RetryDelay = time.Duration(headerSec) * time.Second
				}
			}

			// Per-day quota: don't waste retries on this model
			if lastRetryInfo.Quota == QuotaPerDay {
				fmt.Printf("    Daily quota exhausted for %s, breaking out\n", opts.Model)
				break
			}

			// Rate limited — retry with exponential backoff + jitter
			waitTime := time.Duration(1<<uint(attempt)) * 10 * time.Second
			waitTime = addJitter(waitTime, waitTime/4)
			if lastRetryInfo.RetryDelay > 0 && lastRetryInfo.RetryDelay > waitTime {
				waitTime = lastRetryInfo.RetryDelay
			}
			fmt.Printf("    Rate limited (quota=%d, retryDelay=%v), waiting %v (attempt %d/%d)...\n",
				lastRetryInfo.Quota, lastRetryInfo.RetryDelay, waitTime, attempt+1, opts.MaxRetries)
			time.Sleep(waitTime)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
		}

		var response GeminiTTSResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
			return nil, fmt.Errorf("no audio data in response")
		}

		// Decode base64 audio data
		audioDataB64 := response.Candidates[0].Content.Parts[0].InlineData.Data
		audioData, err := base64.StdEncoding.DecodeString(audioDataB64)
		if err != nil {
			return nil, fmt.Errorf("failed to decode audio data: %w", err)
		}

		// Save as WAV file (PCM format, 24kHz, mono, 16-bit)
		if err := saveWAVFile(opts.OutputPath, audioData, 1, 24000, 2); err != nil {
			return nil, fmt.Errorf("failed to save WAV file: %w", err)
		}

		return &ProviderResult{}, nil
	}

	return &ProviderResult{RetryDelay: lastRetryInfo.RetryDelay, QuotaType: lastRetryInfo.Quota},
		&TTSRateLimitError{StatusCode: 429, Model: opts.Model, ModelsTriedCount: 1}
}

// generateSpeech is a legacy wrapper around AIStudioProvider for backward compatibility.
func generateSpeech(opts SpeechOpts) (*generateSpeechResult, error) {
	apiKey := os.Getenv("GOOGLE_AI_API_KEY")
	if apiKey == "" {
		return nil, &TTSAuthError{Detail: "GOOGLE_AI_API_KEY not set"}
	}

	provider := &AIStudioProvider{APIKey: apiKey}
	result, err := provider.GenerateAudio(ProviderOpts{
		Text:           opts.Text,
		OutputPath:     opts.OutputPath,
		Voice:          opts.Voice,
		Model:          opts.Model,
		WordsPerMinute: opts.WordsPerMinute,
		MaxRetries:     opts.MaxRetries,
		Verbose:        opts.Verbose,
	})

	if result != nil {
		return &generateSpeechResult{
			RetryDelay: result.RetryDelay,
			QuotaType:  result.QuotaType,
		}, err
	}
	if err != nil {
		return nil, err
	}
	return &generateSpeechResult{}, nil
}
