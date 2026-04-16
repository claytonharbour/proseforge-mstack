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

func TestVertexGenerateAudioSuccess(t *testing.T) {
	// Fake PCM data (just bytes for testing)
	fakePCM := make([]byte, 48000) // 1 second at 24kHz mono 16-bit
	fakePCMB64 := base64.StdEncoding.EncodeToString(fakePCM)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify request body (same format as AI Studio)
		var req GeminiTTSRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(req.Contents) == 0 || len(req.Contents[0].Parts) == 0 {
			t.Fatal("expected non-empty contents")
		}
		if req.Contents[0].Parts[0].Text != "Hello world" {
			t.Errorf("expected text 'Hello world', got %q", req.Contents[0].Parts[0].Text)
		}

		resp := GeminiTTSResponse{
			Candidates: []GeminiCandidate{
				{
					Content: GeminiContentResponse{
						Parts: []GeminiPartResponse{
							{InlineData: GeminiInlineData{Data: fakePCMB64}},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	old := vertexBaseURL
	vertexBaseURL = server.URL
	defer func() { vertexBaseURL = old }()

	outPath := filepath.Join(t.TempDir(), "test.wav")
	provider := &VertexProvider{
		HTTPClient: server.Client(),
		ProjectID:  "test-project",
		Region:     "us-central1",
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

	// Verify WAV file was written with header
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if len(data) < 44 {
		t.Fatalf("WAV file too small: %d bytes", len(data))
	}
	if string(data[0:4]) != "RIFF" {
		t.Error("expected RIFF header")
	}
	if string(data[8:12]) != "WAVE" {
		t.Error("expected WAVE format")
	}
}

func TestVertexEndpointFormat(t *testing.T) {
	old := vertexBaseURL
	vertexBaseURL = ""
	defer func() { vertexBaseURL = old }()

	provider := &VertexProvider{
		ProjectID: "my-project",
		Region:    "us-central1",
	}

	endpoint := provider.endpoint("gemini-2.5-flash-tts")
	expected := "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/my-project/locations/us-central1/publishers/google/models/gemini-2.5-flash-tts:generateContent"
	if endpoint != expected {
		t.Errorf("expected:\n  %s\ngot:\n  %s", expected, endpoint)
	}
}

func TestVertexRateLimitRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"code":429,"message":"quota exceeded","details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"1s"}]}}`))
	}))
	defer server.Close()

	old := vertexBaseURL
	vertexBaseURL = server.URL
	defer func() { vertexBaseURL = old }()

	outPath := filepath.Join(t.TempDir(), "test.wav")
	provider := &VertexProvider{
		HTTPClient: server.Client(),
		ProjectID:  "test-project",
		Region:     "us-central1",
	}

	result, err := provider.GenerateAudio(ProviderOpts{
		Text:       "Hello",
		OutputPath: outPath,
		Voice:      "Kore",
		Model:      "gemini-2.5-flash-tts",
		MaxRetries: 1,
	})
	if err == nil {
		t.Fatal("expected error on rate limit")
	}
	if _, ok := err.(*TTSRateLimitError); !ok {
		t.Errorf("expected TTSRateLimitError, got %T", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result with retry info")
	}
	if result.RetryDelay == 0 {
		t.Error("expected non-zero RetryDelay from parsed error body")
	}
}

func TestVertexNoClient(t *testing.T) {
	provider := &VertexProvider{
		ProjectID: "test",
		Region:    "us-central1",
	}
	_, err := provider.GenerateAudio(ProviderOpts{
		Text:       "Hello",
		OutputPath: "/tmp/test.wav",
		Voice:      "Kore",
		Model:      "gemini-2.5-flash-tts",
		MaxRetries: 1,
	})
	if err == nil {
		t.Fatal("expected error when no HTTP client")
	}
}

func TestVertexAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"code":400,"message":"invalid request"}}`))
	}))
	defer server.Close()

	old := vertexBaseURL
	vertexBaseURL = server.URL
	defer func() { vertexBaseURL = old }()

	outPath := filepath.Join(t.TempDir(), "test.wav")
	provider := &VertexProvider{
		HTTPClient: server.Client(),
		ProjectID:  "test-project",
		Region:     "us-central1",
	}

	_, err := provider.GenerateAudio(ProviderOpts{
		Text:       "Hello",
		OutputPath: outPath,
		Voice:      "Kore",
		Model:      "gemini-2.5-flash-tts",
		MaxRetries: 1,
	})
	if err == nil {
		t.Fatal("expected error on 400")
	}
}

func TestVertexPacingDirective(t *testing.T) {
	var capturedText string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req GeminiTTSRequest
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Contents) > 0 && len(req.Contents[0].Parts) > 0 {
			capturedText = req.Contents[0].Parts[0].Text
		}

		fakePCM := make([]byte, 100)
		resp := GeminiTTSResponse{
			Candidates: []GeminiCandidate{
				{Content: GeminiContentResponse{
					Parts: []GeminiPartResponse{
						{InlineData: GeminiInlineData{Data: base64.StdEncoding.EncodeToString(fakePCM)}},
					},
				}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	old := vertexBaseURL
	vertexBaseURL = server.URL
	defer func() { vertexBaseURL = old }()

	outPath := filepath.Join(t.TempDir(), "test.wav")
	provider := &VertexProvider{
		HTTPClient: server.Client(),
		ProjectID:  "test-project",
		Region:     "us-central1",
	}

	_, err := provider.GenerateAudio(ProviderOpts{
		Text:           "Hello",
		OutputPath:     outPath,
		Voice:          "Kore",
		Model:          "gemini-2.5-flash-tts",
		WordsPerMinute: 180,
		MaxRetries:     1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Text should be clean (no directive prepended)
	if capturedText != "Hello" {
		t.Errorf("expected clean text 'Hello', got %q", capturedText)
	}
}
