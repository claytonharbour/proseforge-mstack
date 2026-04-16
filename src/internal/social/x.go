package social

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProfileSettings contains profile settings per project
var xProfileSettings = map[string]map[string]string{
	"proseforge": {
		"name":        "ProseForge",
		"description": "AI-powered story creation. Transform articles into engaging narratives. ✨",
		"url":         "https://demo.proseforge.ai",
		"location":    "The Creative Cloud",
	},
}

// showXProfile shows current X profile
func showXProfile(project string) (map[string]interface{}, error) {
	env := loadEnv(project)

	required := []string{"X_API_KEY", "X_API_SECRET", "X_ACCESS_TOKEN", "X_ACCESS_SECRET"}
	missing := []string{}
	for _, k := range required {
		if env[k] == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing credentials: %s", strings.Join(missing, ", "))
	}

	// Note: Full profile retrieval requires OAuth 1.0a which is complex
	// For now, return settings
	result := make(map[string]interface{})
	if settings := xProfileSettings[project]; settings != nil {
		for k, v := range settings {
			result[k] = v
		}
	}
	return result, nil
}

// updateXProfile updates X profile
func updateXProfile(project string, settings map[string]string) error {
	env := loadEnv(project)

	required := []string{"X_API_KEY", "X_API_SECRET", "X_ACCESS_TOKEN", "X_ACCESS_SECRET"}
	missing := []string{}
	for _, k := range required {
		if env[k] == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing credentials: %s", strings.Join(missing, ", "))
	}

	// Note: Profile update requires OAuth 1.0a which is complex
	// For now, just validate credentials are present
	return fmt.Errorf("profile update requires OAuth 1.0a implementation")
}

// postToX posts a tweet to X using API v2
func postToX(project, message string) (string, error) {
	env := loadEnv(project)

	required := []string{"X_API_KEY", "X_API_SECRET", "X_ACCESS_TOKEN", "X_ACCESS_SECRET"}
	missing := []string{}
	for _, k := range required {
		if env[k] == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("missing credentials: %s", strings.Join(missing, ", "))
	}

	// Use Twitter API v2
	endpoint := "https://api.twitter.com/2/tweets"
	client := &http.Client{Timeout: 30 * time.Second}

	payload := map[string]interface{}{
		"text": message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env["X_BEARER_TOKEN"])

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if data, ok := result["data"].(map[string]interface{}); ok {
		if tweetID, ok := data["id"].(string); ok {
			return tweetID, nil
		}
	}

	return "", fmt.Errorf("unexpected response format")
}
