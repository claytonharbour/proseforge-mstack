package video

import (
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/google"
)

// ResolveEngine resolves "auto" (or empty) to the best available TTS engine.
// Priority: cloudtts → vertex → gemini → say.
// Explicit engine names are returned as-is after validation.
func ResolveEngine(engine, project string) (string, error) {
	if engine != "" && engine != "auto" {
		valid := map[string]bool{"say": true, "gemini": true, "cloudtts": true, "vertex": true}
		if !valid[engine] {
			return "", fmt.Errorf("tts-engine must be 'say', 'gemini', 'cloudtts', 'vertex', or 'auto', got %q", engine)
		}
		return engine, nil
	}

	// Auto-detect: try API providers in order, fall back to say
	if hasOAuth(project) {
		return "cloudtts", nil
	}
	if os.Getenv("GOOGLE_AI_API_KEY") != "" {
		return "gemini", nil
	}
	return "say", nil
}

// hasOAuth checks whether an OAuth token file exists for the given project.
// Uses a lightweight file-existence check instead of creating a full OAuth client.
func hasOAuth(project string) bool {
	if project == "" {
		project = "proseforge"
	}
	cfg := google.AuthConfigForProject(project)
	_, err := os.Stat(cfg.TokenFilePath)
	return err == nil
}
