package social

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"time"
)

const graphAPIBase = "https://graph.facebook.com/v18.0"

// PageSettings contains page settings per project
var facebookPageSettings = map[string]map[string]interface{}{
	"proseforge": {
		"about":       "AI-powered story creation. Transform articles into engaging narratives.",
		"description": "ProseForge uses AI to help writers create compelling stories from any source material. Whether you're transforming articles, blog posts, or research into narratives, ProseForge makes it easy.",
		"website":     "https://proseforge.ai",
		"emails":      []string{"proseforgestories@gmail.com"},
	},
}

// getPageInfo gets current Facebook Page information
func getFacebookPageInfo(pageID, accessToken string) (map[string]interface{}, error) {
	fields := "id,name,about,description,website,emails,fan_count,link,username,category"
	endpoint := fmt.Sprintf("%s/%s", graphAPIBase, pageID)
	params := url.Values{}
	params.Set("fields", fields)
	params.Set("access_token", accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// updatePageInfo updates Facebook Page information
func updateFacebookPageInfo(pageID, accessToken string, updates map[string]string) (bool, error) {
	endpoint := fmt.Sprintf("%s/%s", graphAPIBase, pageID)
	params := url.Values{}
	params.Set("access_token", accessToken)

	formData := url.Values{}
	for k, v := range updates {
		formData.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(endpoint+"?"+params.Encode(), formData)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	success, _ := result["success"].(bool)
	return success, nil
}

// showFacebookPage shows current Facebook Page information
func showFacebookPage(project string) (map[string]interface{}, error) {
	env := loadEnv(project)

	pageID := env["FACEBOOK_PAGE_ID"]
	accessToken := env["FACEBOOK_ACCESS_TOKEN"]

	if pageID == "" {
		return nil, fmt.Errorf("FACEBOOK_PAGE_ID not set in .env")
	}

	if accessToken == "" {
		return nil, fmt.Errorf("FACEBOOK_ACCESS_TOKEN not set in .env")
	}

	return getFacebookPageInfo(pageID, accessToken)
}

// updateFacebookPage updates Facebook Page with project settings
func updateFacebookPage(project string, updates map[string]string) error {
	env := loadEnv(project)

	pageID := env["FACEBOOK_PAGE_ID"]
	accessToken := env["FACEBOOK_ACCESS_TOKEN"]

	if pageID == "" || accessToken == "" {
		return fmt.Errorf("FACEBOOK_PAGE_ID and FACEBOOK_ACCESS_TOKEN required in .env")
	}

	success, err := updateFacebookPageInfo(pageID, accessToken, updates)
	if err != nil {
		return err
	}

	if !success {
		return fmt.Errorf("failed to update page")
	}

	return nil
}

// debugFacebookToken gets token expiry info from Facebook debug endpoint
func debugFacebookToken(accessToken, appID, appSecret string) (int, error) {
	debugEndpoint := fmt.Sprintf("%s/debug_token", graphAPIBase)
	params := url.Values{}
	params.Set("input_token", accessToken)
	params.Set("access_token", fmt.Sprintf("%s|%s", appID, appSecret))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(debugEndpoint + "?" + params.Encode())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("unexpected response format")
	}

	// Get expiration timestamp
	expiresAt, ok := data["expires_at"].(float64)
	if !ok || expiresAt == 0 {
		// Token never expires
		return -1, nil
	}

	expiresTime := time.Unix(int64(expiresAt), 0)
	daysUntilExpiry := int(time.Until(expiresTime).Hours() / 24)
	return daysUntilExpiry, nil
}

// refreshFacebookToken exchanges current token for a new long-lived token
func refreshFacebookToken(project string) (*RefreshResult, error) {
	env := loadEnv(project)

	accessToken := env["FACEBOOK_ACCESS_TOKEN"]
	appID := env["FACEBOOK_APP_ID"]
	appSecret := env["FACEBOOK_APP_SECRET"]

	if accessToken == "" {
		return &RefreshResult{
			Success: false,
			Error:   "FACEBOOK_ACCESS_TOKEN not set in .env",
		}, nil
	}

	if appID == "" || appSecret == "" {
		return &RefreshResult{
			Success: false,
			Error:   "FACEBOOK_APP_ID and FACEBOOK_APP_SECRET required for token refresh",
		}, nil
	}

	// Get current token expiry
	oldExpiry, err := debugFacebookToken(accessToken, appID, appSecret)
	if err != nil {
		return &RefreshResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to check current token: %v", err),
		}, nil
	}

	// Exchange for new long-lived token
	exchangeEndpoint := fmt.Sprintf("%s/oauth/access_token", graphAPIBase)
	params := url.Values{}
	params.Set("grant_type", "fb_exchange_token")
	params.Set("client_id", appID)
	params.Set("client_secret", appSecret)
	params.Set("fb_exchange_token", accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(exchangeEndpoint + "?" + params.Encode())
	if err != nil {
		return &RefreshResult{
			Success: false,
			Error:   fmt.Sprintf("Token exchange failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return &RefreshResult{
			Success: false,
			Error:   fmt.Sprintf("API error: %d - %s", resp.StatusCode, string(body)),
		}, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return &RefreshResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse response: %v", err),
		}, nil
	}

	newToken, ok := result["access_token"].(string)
	if !ok || newToken == "" {
		return &RefreshResult{
			Success: false,
			Error:   "No access_token in response",
		}, nil
	}

	// Get new token expiry
	newExpiry, _ := debugFacebookToken(newToken, appID, appSecret)

	// Update .env file
	envPath := filepath.Join(getProjectPath(project), ".env")
	if err := updateEnvValue(envPath, "FACEBOOK_ACCESS_TOKEN", newToken); err != nil {
		return &RefreshResult{
			Success:          true,
			OldExpiresInDays: oldExpiry,
			NewExpiresInDays: newExpiry,
			Message:          "Token refreshed but failed to update .env: " + err.Error(),
		}, nil
	}

	return &RefreshResult{
		Success:          true,
		OldExpiresInDays: oldExpiry,
		NewExpiresInDays: newExpiry,
		Message:          "Token refreshed successfully. Updated .env file.",
	}, nil
}

// postToFacebook posts a message to the Facebook Page
func postToFacebook(project, message string) (string, error) {
	env := loadEnv(project)

	pageID := env["FACEBOOK_PAGE_ID"]
	accessToken := env["FACEBOOK_ACCESS_TOKEN"]

	if pageID == "" || accessToken == "" {
		return "", fmt.Errorf("FACEBOOK_PAGE_ID and FACEBOOK_ACCESS_TOKEN required in .env")
	}

	endpoint := fmt.Sprintf("%s/%s/feed", graphAPIBase, pageID)
	params := url.Values{}
	params.Set("access_token", accessToken)

	formData := url.Values{}
	formData.Set("message", message)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(endpoint+"?"+params.Encode(), formData)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if postID, ok := result["id"].(string); ok {
		return postID, nil
	}

	return "", fmt.Errorf("unexpected response format")
}
