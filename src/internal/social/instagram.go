package social

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// getInstagramAccountID gets Instagram Business Account ID linked to Facebook Page
func getInstagramAccountID(pageID, accessToken string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s", graphAPIBase, pageID)
	params := url.Values{}
	params.Set("fields", "instagram_business_account")
	params.Set("access_token", accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if igAccount, ok := result["instagram_business_account"].(map[string]interface{}); ok {
		if id, ok := igAccount["id"].(string); ok {
			return id, nil
		}
	}

	return "", fmt.Errorf("instagram_business_account not found")
}

// getInstagramProfileInfo gets Instagram profile information
func getInstagramProfileInfo(igUserID, accessToken string) (map[string]interface{}, error) {
	fields := "id,username,name,biography,followers_count,follows_count,media_count,profile_picture_url,website"
	endpoint := fmt.Sprintf("%s/%s", graphAPIBase, igUserID)
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

// showInstagramProfile shows Instagram profile information
func showInstagramProfile(project string) (map[string]interface{}, error) {
	env := loadEnv(project)

	igUserID := env["INSTAGRAM_USER_ID"]
	accessToken := env["INSTAGRAM_ACCESS_TOKEN"]
	if accessToken == "" {
		accessToken = env["FACEBOOK_ACCESS_TOKEN"]
	}

	if igUserID == "" {
		pageID := env["FACEBOOK_PAGE_ID"]
		if pageID != "" && accessToken != "" {
			id, err := getInstagramAccountID(pageID, accessToken)
			if err == nil && id != "" {
				igUserID = id
			}
		}
	}

	if igUserID == "" {
		return nil, fmt.Errorf("INSTAGRAM_USER_ID not set and couldn't find via Facebook Page")
	}

	if accessToken == "" {
		return nil, fmt.Errorf("INSTAGRAM_ACCESS_TOKEN (or FACEBOOK_ACCESS_TOKEN) not set")
	}

	return getInstagramProfileInfo(igUserID, accessToken)
}

// updateInstagramProfile updates Instagram profile (not supported via API)
func updateInstagramProfile(project string, updates map[string]string) error {
	return fmt.Errorf("Instagram bio/profile cannot be updated via API. Use the Instagram app to update your profile")
}

// postToInstagram posts an image to Instagram
func postToInstagram(project, caption, imageURL string) (string, error) {
	env := loadEnv(project)

	igUserID := env["INSTAGRAM_USER_ID"]
	accessToken := env["INSTAGRAM_ACCESS_TOKEN"]
	if accessToken == "" {
		accessToken = env["FACEBOOK_ACCESS_TOKEN"]
	}

	if igUserID == "" || accessToken == "" {
		return "", fmt.Errorf("INSTAGRAM_USER_ID and access token required")
	}

	// Step 1: Create media container
	endpoint := fmt.Sprintf("%s/%s/media", graphAPIBase, igUserID)
	params := url.Values{}
	params.Set("access_token", accessToken)

	formData := url.Values{}
	formData.Set("image_url", imageURL)
	formData.Set("caption", caption)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(endpoint+"?"+params.Encode(), formData)
	if err != nil {
		return "", fmt.Errorf("failed to create media container: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read container response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to create media container: %d - %s", resp.StatusCode, string(body))
	}

	var container map[string]interface{}
	if err := json.Unmarshal(body, &container); err != nil {
		return "", fmt.Errorf("failed to parse container response: %w", err)
	}

	containerID, ok := container["id"].(string)
	if !ok {
		return "", fmt.Errorf("failed to get container ID")
	}

	// Step 2: Publish the container
	endpoint = fmt.Sprintf("%s/%s/media_publish", graphAPIBase, igUserID)
	formData = url.Values{}
	formData.Set("creation_id", containerID)

	resp, err = client.PostForm(endpoint+"?"+params.Encode(), formData)
	if err != nil {
		return "", fmt.Errorf("failed to publish: %w", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read publish response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to publish: %d - %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse publish response: %w", err)
	}

	if mediaID, ok := result["id"].(string); ok {
		return mediaID, nil
	}

	return "", fmt.Errorf("unexpected response format")
}
