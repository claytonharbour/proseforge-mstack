package video

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveEngineExplicit(t *testing.T) {
	tests := []struct {
		engine string
		want   string
	}{
		{"say", "say"},
		{"gemini", "gemini"},
		{"cloudtts", "cloudtts"},
		{"vertex", "vertex"},
	}
	for _, tt := range tests {
		t.Run(tt.engine, func(t *testing.T) {
			got, err := ResolveEngine(tt.engine, "proseforge")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ResolveEngine(%q) = %q, want %q", tt.engine, got, tt.want)
			}
		})
	}
}

func TestResolveEngineInvalid(t *testing.T) {
	_, err := ResolveEngine("bogus", "proseforge")
	if err == nil {
		t.Fatal("expected error for invalid engine")
	}
}

func TestResolveEngineAutoWithToken(t *testing.T) {
	// Create a temporary token file to simulate OAuth credentials
	tmpDir := t.TempDir()
	tokenDir := filepath.Join(tmpDir, "projects", "testproj", "google")
	if err := os.MkdirAll(tokenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tokenDir, "token.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Point MSTACK_CONFIG_DIR at our temp dir so AuthConfigForProject resolves there
	t.Setenv("MSTACK_CONFIG_DIR", tmpDir)
	os.Unsetenv("GOOGLE_AI_API_KEY")

	got, err := ResolveEngine("auto", "testproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "cloudtts" {
		t.Errorf("ResolveEngine(auto) with token = %q, want 'cloudtts'", got)
	}
}

func TestResolveEngineAutoWithAPIKey(t *testing.T) {
	// No token file, but API key is set
	t.Setenv("MSTACK_CONFIG_DIR", t.TempDir()) // empty dir, no token
	t.Setenv("GOOGLE_AI_API_KEY", "test-key")

	got, err := ResolveEngine("", "testproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "gemini" {
		t.Errorf("ResolveEngine('') with API key = %q, want 'gemini'", got)
	}
}

func TestResolveEngineAutoFallbackSay(t *testing.T) {
	// No token file, no API key
	t.Setenv("MSTACK_CONFIG_DIR", t.TempDir())
	os.Unsetenv("GOOGLE_AI_API_KEY")

	got, err := ResolveEngine("", "testproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "say" {
		t.Errorf("ResolveEngine('') with nothing = %q, want 'say'", got)
	}
}

func TestHasOAuthWithToken(t *testing.T) {
	tmpDir := t.TempDir()
	tokenDir := filepath.Join(tmpDir, "projects", "testproj", "google")
	if err := os.MkdirAll(tokenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tokenDir, "token.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MSTACK_CONFIG_DIR", tmpDir)

	if !hasOAuth("testproj") {
		t.Error("hasOAuth should return true when token.json exists")
	}
}

func TestHasOAuthWithoutToken(t *testing.T) {
	t.Setenv("MSTACK_CONFIG_DIR", t.TempDir())

	if hasOAuth("testproj") {
		t.Error("hasOAuth should return false when token.json is missing")
	}
}
