package video

import (
	"os"
	"testing"
)

func TestResolveProviderGemini(t *testing.T) {
	t.Setenv("GOOGLE_AI_API_KEY", "test-key")
	provider, err := resolveProvider(TTSOpts{Engine: "gemini"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.Name() != "aistudio" {
		t.Errorf("expected provider name 'aistudio', got %q", provider.Name())
	}
	if provider.AudioExtension() != ".wav" {
		t.Errorf("expected extension '.wav', got %q", provider.AudioExtension())
	}
}

func TestResolveProviderGeminiNoKey(t *testing.T) {
	os.Unsetenv("GOOGLE_AI_API_KEY")
	_, err := resolveProvider(TTSOpts{Engine: "gemini"})
	if err == nil {
		t.Fatal("expected error when GOOGLE_AI_API_KEY is not set")
	}
	if _, ok := err.(*TTSAuthError); !ok {
		t.Errorf("expected TTSAuthError, got %T", err)
	}
}

func TestResolveProviderUnknown(t *testing.T) {
	_, err := resolveProvider(TTSOpts{Engine: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown engine")
	}
}

func TestRedactAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "key at end",
			input:    "https://example.com/v1/models?key=abc123",
			expected: "https://example.com/v1/models?key=REDACTED",
		},
		{
			name:     "key in middle",
			input:    "https://example.com/v1/models?key=abc123&alt=json",
			expected: "https://example.com/v1/models?key=REDACTED&alt=json",
		},
		{
			name:     "no key param",
			input:    "https://example.com/v1/models",
			expected: "https://example.com/v1/models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactAPIKey(tt.input)
			if got != tt.expected {
				t.Errorf("redactAPIKey(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDefaultModelLists(t *testing.T) {
	if len(DefaultTTSModels) == 0 {
		t.Error("DefaultTTSModels should not be empty")
	}
	if len(DefaultCloudTTSModels) == 0 {
		t.Error("DefaultCloudTTSModels should not be empty")
	}
	if len(DefaultVertexModels) == 0 {
		t.Error("DefaultVertexModels should not be empty")
	}

	// Cloud TTS and Vertex should not include preview models
	for _, m := range DefaultCloudTTSModels {
		if contains(m, "preview") {
			t.Errorf("DefaultCloudTTSModels should not contain preview models, found %q", m)
		}
	}
	for _, m := range DefaultVertexModels {
		if contains(m, "preview") {
			t.Errorf("DefaultVertexModels should not contain preview models, found %q", m)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestAIStudioProviderDefaults(t *testing.T) {
	p := &AIStudioProvider{APIKey: "test"}
	if p.Name() != "aistudio" {
		t.Errorf("expected 'aistudio', got %q", p.Name())
	}
	if p.AudioExtension() != ".wav" {
		t.Errorf("expected '.wav', got %q", p.AudioExtension())
	}
	if len(p.DefaultModels()) == 0 {
		t.Error("expected non-empty DefaultModels")
	}
}

func TestCloudTTSProviderDefaults(t *testing.T) {
	p := &CloudTTSProvider{}
	if p.Name() != "cloudtts" {
		t.Errorf("expected 'cloudtts', got %q", p.Name())
	}
	if p.AudioExtension() != ".mp3" {
		t.Errorf("expected '.mp3', got %q", p.AudioExtension())
	}
	if len(p.DefaultModels()) == 0 {
		t.Error("expected non-empty DefaultModels")
	}
}

func TestVertexProviderDefaults(t *testing.T) {
	p := &VertexProvider{}
	if p.Name() != "vertex" {
		t.Errorf("expected 'vertex', got %q", p.Name())
	}
	if p.AudioExtension() != ".wav" {
		t.Errorf("expected '.wav', got %q", p.AudioExtension())
	}
	if len(p.DefaultModels()) == 0 {
		t.Error("expected non-empty DefaultModels")
	}
}
