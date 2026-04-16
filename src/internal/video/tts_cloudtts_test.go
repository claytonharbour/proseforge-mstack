package video

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCloudTTSGenerateAudioSuccess(t *testing.T) {
	// Mock MP3 data (just bytes, no real MP3 encoding needed for test)
	fakeMP3 := []byte("fake-mp3-audio-data")
	fakeMP3B64 := base64.StdEncoding.EncodeToString(fakeMP3)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/text:synthesize" {
			t.Errorf("expected path /v1/text:synthesize, got %s", r.URL.Path)
		}

		// Verify request body
		var req cloudTTSRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Input.Text != "Hello world" {
			t.Errorf("expected text 'Hello world', got %q", req.Input.Text)
		}
		if req.Input.Prompt != "" {
			t.Errorf("expected empty prompt (no WPM), got %q", req.Input.Prompt)
		}
		if req.Voice.Name != "Kore" {
			t.Errorf("expected voice 'Kore', got %q", req.Voice.Name)
		}
		if req.Voice.ModelName != "gemini-2.5-flash-tts" {
			t.Errorf("expected model 'gemini-2.5-flash-tts', got %q", req.Voice.ModelName)
		}
		if req.AudioConfig.AudioEncoding != "MP3" {
			t.Errorf("expected encoding 'MP3', got %q", req.AudioConfig.AudioEncoding)
		}

		resp := cloudTTSResponse{AudioContent: fakeMP3B64}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Override base URL for testing
	old := cloudTTSBaseURL
	cloudTTSBaseURL = server.URL
	defer func() { cloudTTSBaseURL = old }()

	outPath := filepath.Join(t.TempDir(), "test.mp3")
	provider := &CloudTTSProvider{
		HTTPClient: server.Client(),
	}

	result, err := provider.GenerateAudio(ProviderOpts{
		Text:       "Hello world",
		OutputPath: outPath,
		Voice:      "Kore",
		Model:      "gemini-2.5-flash-tts",
		MaxRetries: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RetryDelay != 0 {
		t.Errorf("expected zero RetryDelay, got %v", result.RetryDelay)
	}

	// Verify file was written
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(data) != string(fakeMP3) {
		t.Errorf("output file content mismatch")
	}
}

func TestCloudTTSPacingDirective(t *testing.T) {
	var capturedPrompt string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req cloudTTSRequest
		json.NewDecoder(r.Body).Decode(&req)
		capturedPrompt = req.Input.Prompt

		fakeMP3 := base64.StdEncoding.EncodeToString([]byte("audio"))
		resp := cloudTTSResponse{AudioContent: fakeMP3}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	old := cloudTTSBaseURL
	cloudTTSBaseURL = server.URL
	defer func() { cloudTTSBaseURL = old }()

	outPath := filepath.Join(t.TempDir(), "test.mp3")
	provider := &CloudTTSProvider{HTTPClient: server.Client()}

	// Use text long enough (>= cloudTTSMinPromptWords) so prompt is included
	_, err := provider.GenerateAudio(ProviderOpts{
		Text:           "This is a longer sentence that has more than ten words in total for testing purposes",
		OutputPath:     outPath,
		Voice:          "Kore",
		Model:          "gemini-2.5-flash-tts",
		WordsPerMinute: 180,
		MaxRetries:     1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPrompt == "" {
		t.Error("expected pacing prompt to be set for long text")
	}

	// Short text should NOT include prompt
	capturedPrompt = ""
	outPath2 := filepath.Join(t.TempDir(), "test2.mp3")
	_, err = provider.GenerateAudio(ProviderOpts{
		Text:           "Hello",
		OutputPath:     outPath2,
		Voice:          "Kore",
		Model:          "gemini-2.5-flash-tts",
		WordsPerMinute: 180,
		MaxRetries:     1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPrompt != "" {
		t.Errorf("expected empty prompt for short text, got %q", capturedPrompt)
	}
}

func TestCloudTTSRegionalEndpoint(t *testing.T) {
	provider := &CloudTTSProvider{Region: "us"}
	endpoint := provider.endpoint()
	expected := "https://us-texttospeech.googleapis.com"
	if endpoint != expected {
		t.Errorf("expected %q, got %q", expected, endpoint)
	}
}

func TestCloudTTSGlobalEndpoint(t *testing.T) {
	old := cloudTTSBaseURL
	cloudTTSBaseURL = "https://texttospeech.googleapis.com"
	defer func() { cloudTTSBaseURL = old }()

	provider := &CloudTTSProvider{}
	endpoint := provider.endpoint()
	if endpoint != "https://texttospeech.googleapis.com" {
		t.Errorf("expected global endpoint, got %q", endpoint)
	}
}

func TestCloudTTSRateLimitRetry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		fakeMP3 := base64.StdEncoding.EncodeToString([]byte("audio"))
		resp := cloudTTSResponse{AudioContent: fakeMP3}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	old := cloudTTSBaseURL
	cloudTTSBaseURL = server.URL
	defer func() { cloudTTSBaseURL = old }()

	outPath := filepath.Join(t.TempDir(), "test.mp3")
	provider := &CloudTTSProvider{HTTPClient: server.Client()}

	// With only 1 retry, the first 429 consumes it, so we get rate limit error
	_, err := provider.GenerateAudio(ProviderOpts{
		Text:       "Hello",
		OutputPath: outPath,
		Voice:      "Kore",
		Model:      "gemini-2.5-flash-tts",
		MaxRetries: 1,
	})
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if _, ok := err.(*TTSRateLimitError); !ok {
		t.Errorf("expected TTSRateLimitError, got %T: %v", err, err)
	}
}

// TestCloudTTSShortTextNoPrompt verifies the exact failing scenario: 9-word text
// with WPM=156 (> DefaultGeminiWPM=96) should NOT include a prompt field because
// the word count is below cloudTTSMinPromptWords (10).
func TestCloudTTSShortTextNoPrompt(t *testing.T) {
	var capturedReq cloudTTSRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedReq)
		fakeMP3 := base64.StdEncoding.EncodeToString([]byte("audio"))
		resp := cloudTTSResponse{AudioContent: fakeMP3}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	old := cloudTTSBaseURL
	cloudTTSBaseURL = server.URL
	defer func() { cloudTTSBaseURL = old }()

	outPath := filepath.Join(t.TempDir(), "test.mp3")
	provider := &CloudTTSProvider{HTTPClient: server.Client()}

	// 9 words — the exact segment that caused hallucination
	_, err := provider.GenerateAudio(ProviderOpts{
		Text:           "Now let's see how the author receives human feedback.",
		OutputPath:     outPath,
		Voice:          "Kore",
		Model:          "gemini-2.5-flash-tts",
		WordsPerMinute: 156,
		MaxRetries:     1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq.Input.Prompt != "" {
		t.Errorf("expected empty prompt for 9-word text, got %q", capturedReq.Input.Prompt)
	}
	if capturedReq.Input.Text != "Now let's see how the author receives human feedback." {
		t.Errorf("unexpected text: %q", capturedReq.Input.Text)
	}
}

// TestCloudTTS499Recovery verifies that a 499 CANCELLED response triggers a
// retry without the prompt field.
func TestCloudTTS499Recovery(t *testing.T) {
	callCount := 0
	var firstPrompt, secondPrompt string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var req cloudTTSRequest
		json.NewDecoder(r.Body).Decode(&req)

		if callCount == 1 {
			firstPrompt = req.Input.Prompt
			w.WriteHeader(499)
			w.Write([]byte(`{"error":{"message":"CANCELLED"}}`))
			return
		}
		secondPrompt = req.Input.Prompt
		fakeMP3 := base64.StdEncoding.EncodeToString([]byte("audio"))
		resp := cloudTTSResponse{AudioContent: fakeMP3}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	old := cloudTTSBaseURL
	cloudTTSBaseURL = server.URL
	defer func() { cloudTTSBaseURL = old }()

	outPath := filepath.Join(t.TempDir(), "test.mp3")
	provider := &CloudTTSProvider{HTTPClient: server.Client()}

	// 15 words — above threshold, so prompt IS included on first attempt
	_, err := provider.GenerateAudio(ProviderOpts{
		Text:           "This is a much longer sentence that has well over ten words in it for the test case.",
		OutputPath:     outPath,
		Voice:          "Kore",
		Model:          "gemini-2.5-flash-tts",
		WordsPerMinute: 156,
		MaxRetries:     3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if firstPrompt == "" {
		t.Error("expected prompt on first request (text >= 10 words)")
	}
	if secondPrompt != "" {
		t.Errorf("expected empty prompt after 499 recovery, got %q", secondPrompt)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (499 + success), got %d", callCount)
	}

	// Verify file was written
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Error("expected output file to exist after 499 recovery")
	}
}

// TestCloudTTSShortTextBoundary verifies the exact boundary at cloudTTSMinPromptWords (10).
func TestCloudTTSShortTextBoundary(t *testing.T) {
	var capturedPrompts []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req cloudTTSRequest
		json.NewDecoder(r.Body).Decode(&req)
		capturedPrompts = append(capturedPrompts, req.Input.Prompt)
		fakeMP3 := base64.StdEncoding.EncodeToString([]byte("audio"))
		resp := cloudTTSResponse{AudioContent: fakeMP3}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	old := cloudTTSBaseURL
	cloudTTSBaseURL = server.URL
	defer func() { cloudTTSBaseURL = old }()

	provider := &CloudTTSProvider{HTTPClient: server.Client()}

	// Exactly 10 words — should include prompt
	outPath1 := filepath.Join(t.TempDir(), "ten.mp3")
	_, err := provider.GenerateAudio(ProviderOpts{
		Text:           "One two three four five six seven eight nine ten.",
		OutputPath:     outPath1,
		Voice:          "Kore",
		Model:          "gemini-2.5-flash-tts",
		WordsPerMinute: 156,
		MaxRetries:     1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Exactly 9 words — should NOT include prompt
	outPath2 := filepath.Join(t.TempDir(), "nine.mp3")
	_, err = provider.GenerateAudio(ProviderOpts{
		Text:           "One two three four five six seven eight nine.",
		OutputPath:     outPath2,
		Voice:          "Kore",
		Model:          "gemini-2.5-flash-tts",
		WordsPerMinute: 156,
		MaxRetries:     1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedPrompts) < 2 {
		t.Fatalf("expected at least 2 captured prompts, got %d", len(capturedPrompts))
	}
	if capturedPrompts[0] == "" {
		t.Error("expected prompt for 10-word text (at threshold)")
	}
	if capturedPrompts[1] != "" {
		t.Errorf("expected empty prompt for 9-word text (below threshold), got %q", capturedPrompts[1])
	}
}

func TestCloudTTSNoClient(t *testing.T) {
	provider := &CloudTTSProvider{}
	_, err := provider.GenerateAudio(ProviderOpts{
		Text:       "Hello",
		OutputPath: "/tmp/test.mp3",
		Voice:      "Kore",
		Model:      "gemini-2.5-flash-tts",
		MaxRetries: 1,
	})
	if err == nil {
		t.Fatal("expected error when no HTTP client")
	}
}
