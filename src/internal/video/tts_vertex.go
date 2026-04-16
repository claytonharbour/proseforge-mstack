package video

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// VertexProvider implements TTSProvider for Google Vertex AI.
type VertexProvider struct {
	HTTPClient *http.Client // OAuth-authenticated client
	ProjectID  string       // GCP project ID
	Region     string       // e.g. "us-central1"
}

func (p *VertexProvider) Name() string { return "vertex" }

func (p *VertexProvider) AudioExtension() string { return ".wav" }

func (p *VertexProvider) DefaultModels() []string { return DefaultVertexModels }

// vertexBaseURL is overridable for testing.
var vertexBaseURL = ""

func (p *VertexProvider) endpoint(model string) string {
	base := vertexBaseURL
	if base == "" {
		base = fmt.Sprintf("https://%s-aiplatform.googleapis.com", p.Region)
	}
	return fmt.Sprintf("%s/v1beta1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		base, p.ProjectID, p.Region, model)
}

func (p *VertexProvider) GenerateAudio(opts ProviderOpts) (*ProviderResult, error) {
	url := p.endpoint(opts.Model)

	// Vertex AI uses the same generateContent request format as AI Studio
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

	client := p.HTTPClient
	if client == nil {
		return nil, fmt.Errorf("no HTTP client configured for Vertex AI")
	}

	var lastRetryInfo GeminiRetryInfo
	for attempt := 0; attempt < opts.MaxRetries; attempt++ {
		verboseLog(opts.Verbose, "[verbose] POST %s", url)
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
			// Vertex AI uses same Gemini error body format
			lastRetryInfo = parseGeminiRetryInfo(body)

			if lastRetryInfo.RetryDelay == 0 {
				if headerSec := parseRetryAfter(resp); headerSec > 0 {
					lastRetryInfo.RetryDelay = time.Duration(headerSec) * time.Second
				}
			}

			if lastRetryInfo.Quota == QuotaPerDay {
				fmt.Printf("    Daily quota exhausted for %s, breaking out\n", opts.Model)
				break
			}

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
			return nil, fmt.Errorf("Vertex AI API error: %d - %s", resp.StatusCode, string(body))
		}

		// Same response format as AI Studio
		var response GeminiTTSResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
			return nil, fmt.Errorf("no audio data in response")
		}

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
