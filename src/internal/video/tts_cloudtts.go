package video

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// CloudTTSProvider implements TTSProvider for Google Cloud Text-to-Speech API.
type CloudTTSProvider struct {
	HTTPClient *http.Client // OAuth-authenticated client
	Region     string       // "" for global, or "us", "eu"
}

func (p *CloudTTSProvider) Name() string { return "cloudtts" }

func (p *CloudTTSProvider) AudioExtension() string { return ".mp3" }

func (p *CloudTTSProvider) DefaultModels() []string { return DefaultCloudTTSModels }

// cloudTTSRequest is the request body for Cloud TTS text:synthesize.
type cloudTTSRequest struct {
	Input       cloudTTSInput       `json:"input"`
	Voice       cloudTTSVoice       `json:"voice"`
	AudioConfig cloudTTSAudioConfig `json:"audioConfig"`
}

type cloudTTSInput struct {
	Text   string `json:"text"`
	Prompt string `json:"prompt,omitempty"`
}

type cloudTTSVoice struct {
	LanguageCode string `json:"languageCode"`
	Name         string `json:"name"`
	ModelName    string `json:"model_name"`
}

type cloudTTSAudioConfig struct {
	AudioEncoding string `json:"audioEncoding"`
}

// cloudTTSResponse is the response from Cloud TTS text:synthesize.
type cloudTTSResponse struct {
	AudioContent string `json:"audioContent"`
}

// cloudTTSBaseURL is overridable for testing.
var cloudTTSBaseURL = "https://texttospeech.googleapis.com"

func (p *CloudTTSProvider) endpoint() string {
	if p.Region != "" {
		return fmt.Sprintf("https://%s-texttospeech.googleapis.com", p.Region)
	}
	return cloudTTSBaseURL
}

// cloudTTSMinPromptWords is the minimum word count for including the prompt
// field. Gemini models via Cloud TTS hallucinate or return 499 on short text
// when a prompt is present.
const cloudTTSMinPromptWords = 10

// cloudTTSMaxDurationRatio is the maximum ratio of actual audio duration to
// expected duration before we consider the output hallucinated.
const cloudTTSMaxDurationRatio = 3.0

func (p *CloudTTSProvider) GenerateAudio(opts ProviderOpts) (*ProviderResult, error) {
	// Try with prompt first (if text is long enough), fall back without.
	result, err := p.generateWithPrompt(opts, true)
	if err != nil {
		return result, err
	}

	// Validate: check audio duration vs expected for hallucination detection
	wordCount := len(strings.Fields(opts.Text))
	wpm := opts.WordsPerMinute
	if wpm <= 0 {
		wpm = DefaultGeminiWPM
	}
	expectedSec := float64(wordCount) / float64(wpm) * 60.0
	if expectedSec < 1.0 {
		expectedSec = 1.0
	}

	actualSec := audioDurationSec(opts.OutputPath)
	if actualSec > 0 && actualSec > expectedSec*cloudTTSMaxDurationRatio {
		verboseLog(opts.Verbose, "[verbose] Audio too long: %.1fs actual vs %.1fs expected (%.1fx) — retrying without prompt",
			actualSec, expectedSec, actualSec/expectedSec)
		os.Remove(opts.OutputPath)
		return p.generateWithPrompt(opts, false)
	}

	return result, nil
}

func (p *CloudTTSProvider) generateWithPrompt(opts ProviderOpts, usePrompt bool) (*ProviderResult, error) {
	url := fmt.Sprintf("%s/v1/text:synthesize", p.endpoint())

	// Only include prompt for longer text — short text + prompt causes
	// hallucination or 499 CANCELLED in Cloud TTS Gemini models.
	var prompt string
	if usePrompt {
		wordCount := len(strings.Fields(opts.Text))
		if wordCount >= cloudTTSMinPromptWords {
			prompt = cloudTTSPrompt(opts.WordsPerMinute)
		}
	}

	reqBody := cloudTTSRequest{
		Input: cloudTTSInput{
			Text:   opts.Text,
			Prompt: prompt,
		},
		Voice: cloudTTSVoice{
			LanguageCode: "en-us",
			Name:         opts.Voice,
			ModelName:    opts.Model,
		},
		AudioConfig: cloudTTSAudioConfig{
			AudioEncoding: "MP3",
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	client := p.HTTPClient
	if client == nil {
		return nil, fmt.Errorf("no HTTP client configured for Cloud TTS")
	}

	var lastResult ProviderResult
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

		// 499 = CANCELLED — Cloud TTS kills requests that loop on short text
		if resp.StatusCode == 499 && prompt != "" {
			verboseLog(opts.Verbose, "[verbose] 499 CANCELLED with prompt, retrying without prompt")
			prompt = ""
			reqBody.Input.Prompt = ""
			jsonData, _ = json.Marshal(reqBody)
			continue
		}

		if resp.StatusCode == 429 || resp.StatusCode == 500 || resp.StatusCode == 503 {
			var retryDelay time.Duration
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if sec, err := strconv.Atoi(ra); err == nil && sec > 0 {
					retryDelay = time.Duration(sec) * time.Second
				}
			}

			info := parseGeminiRetryInfo(body)
			if info.RetryDelay > retryDelay {
				retryDelay = info.RetryDelay
			}

			quotaType := info.Quota
			if quotaType == QuotaUnknown && resp.StatusCode == 429 {
				quotaType = QuotaPerMinute
			}

			lastResult = ProviderResult{RetryDelay: retryDelay, QuotaType: quotaType}

			if quotaType == QuotaPerDay {
				fmt.Printf("    Daily quota exhausted for %s, breaking out\n", opts.Model)
				break
			}

			waitTime := time.Duration(1<<uint(attempt)) * 10 * time.Second
			waitTime = addJitter(waitTime, waitTime/4)
			if retryDelay > 0 && retryDelay > waitTime {
				waitTime = retryDelay
			}
			fmt.Printf("    Rate limited (quota=%d, retryDelay=%v), waiting %v (attempt %d/%d)...\n",
				quotaType, retryDelay, waitTime, attempt+1, opts.MaxRetries)
			time.Sleep(waitTime)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Cloud TTS API error: %d - %s", resp.StatusCode, string(body))
		}

		var response cloudTTSResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse Cloud TTS response: %w", err)
		}

		if response.AudioContent == "" {
			return nil, fmt.Errorf("no audio content in Cloud TTS response")
		}

		audioData, err := base64.StdEncoding.DecodeString(response.AudioContent)
		if err != nil {
			return nil, fmt.Errorf("failed to decode audio data: %w", err)
		}

		if err := os.WriteFile(opts.OutputPath, audioData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write MP3 file: %w", err)
		}

		return &ProviderResult{}, nil
	}

	return &lastResult, &TTSRateLimitError{StatusCode: 429, Model: opts.Model, ModelsTriedCount: 1}
}

// audioDurationSec returns the duration of an audio file in seconds using
// ffprobe. Returns 0 if ffprobe is unavailable or the file can't be probed.
func audioDurationSec(path string) float64 {
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return 0
	}
	sec, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return sec
}
