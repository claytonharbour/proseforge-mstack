package video

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/claytonharbour/proseforge-mstack/src/internal/google"
)

// TTSProvider generates a single audio file from text.
type TTSProvider interface {
	Name() string
	// GenerateAudio produces an audio file. Returns retry/quota metadata alongside any error.
	GenerateAudio(opts ProviderOpts) (*ProviderResult, error)
	// AudioExtension returns the file extension this provider produces (".wav", ".mp3").
	AudioExtension() string
	// DefaultModels returns the static model list for this provider.
	DefaultModels() []string
}

// ProviderOpts configures a single TTS API call.
type ProviderOpts struct {
	Text           string
	OutputPath     string
	Voice          string
	Model          string
	WordsPerMinute int
	MaxRetries     int
	Verbose        bool
}

// ProviderResult carries metadata from a provider call alongside the primary error.
// RetryDelay and QuotaType are populated from rate-limit responses.
type ProviderResult struct {
	RetryDelay time.Duration
	QuotaType  QuotaType
}

// resolveProvider creates the appropriate TTSProvider for the given engine.
func resolveProvider(opts TTSOpts) (TTSProvider, error) {
	switch opts.Engine {
	case "gemini":
		apiKey := os.Getenv("GOOGLE_AI_API_KEY")
		if apiKey == "" {
			return nil, &TTSAuthError{Detail: "GOOGLE_AI_API_KEY not set"}
		}
		return &AIStudioProvider{APIKey: apiKey}, nil
	case "cloudtts":
		project := opts.Project
		if project == "" {
			project = "proseforge"
		}
		client, err := google.GetClientForProject(context.Background(), project)
		if err != nil {
			return nil, fmt.Errorf("failed to get OAuth client for Cloud TTS: %w", err)
		}
		return &CloudTTSProvider{
			HTTPClient: client,
			Region:     os.Getenv("CLOUD_TTS_REGION"),
		}, nil
	case "vertex":
		project := opts.Project
		if project == "" {
			project = "proseforge"
		}
		client, err := google.GetClientForProject(context.Background(), project)
		if err != nil {
			return nil, fmt.Errorf("failed to get OAuth client for Vertex AI: %w", err)
		}
		projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
		if projectID == "" {
			projectID = os.Getenv("VERTEX_PROJECT")
		}
		if projectID == "" {
			return nil, fmt.Errorf("GOOGLE_CLOUD_PROJECT or VERTEX_PROJECT must be set for Vertex AI")
		}
		region := os.Getenv("GOOGLE_CLOUD_REGION")
		if region == "" {
			region = os.Getenv("VERTEX_REGION")
		}
		if region == "" {
			region = "us-central1"
		}
		return &VertexProvider{
			HTTPClient: client,
			ProjectID:  projectID,
			Region:     region,
		}, nil
	default:
		return nil, fmt.Errorf("unknown TTS engine: %q", opts.Engine)
	}
}

// verboseLog logs a message to stderr when verbose is true.
func verboseLog(verbose bool, format string, args ...interface{}) {
	if verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

// redactAPIKey replaces API key values in URLs with "REDACTED".
func redactAPIKey(url string) string {
	if idx := findKeyParam(url); idx >= 0 {
		end := idx
		for end < len(url) && url[end] != '&' {
			end++
		}
		return url[:idx] + "REDACTED" + url[end:]
	}
	return url
}

func findKeyParam(s string) int {
	for i := 0; i < len(s)-4; i++ {
		if s[i:i+4] == "key=" {
			return i + 4
		}
	}
	return -1
}

// newHTTPClient returns an HTTP client with the given timeout.
func newHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{Timeout: timeout}
}
