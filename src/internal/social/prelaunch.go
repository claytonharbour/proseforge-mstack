package social

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// checkCredentials checks that all required API credentials are present
func checkCredentials(project string) (bool, []string) {
	env := loadEnv(project)
	issues := []string{}

	// X/Twitter credentials
	xRequired := []string{"X_API_KEY", "X_API_SECRET", "X_ACCESS_TOKEN", "X_ACCESS_SECRET"}
	xMissing := []string{}
	for _, k := range xRequired {
		if env[k] == "" {
			xMissing = append(xMissing, k)
		}
	}
	if len(xMissing) > 0 {
		issues = append(issues, fmt.Sprintf("X/Twitter: Missing %s", strings.Join(xMissing, ", ")))
	}

	// Facebook credentials
	fbRequired := []string{"FACEBOOK_PAGE_ID", "FACEBOOK_ACCESS_TOKEN"}
	fbMissing := []string{}
	for _, k := range fbRequired {
		if env[k] == "" {
			fbMissing = append(fbMissing, k)
		}
	}
	if len(fbMissing) > 0 {
		issues = append(issues, fmt.Sprintf("Facebook: Missing %s", strings.Join(fbMissing, ", ")))
	}

	// Instagram credentials
	igUserID := env["INSTAGRAM_USER_ID"]
	igToken := env["INSTAGRAM_ACCESS_TOKEN"]
	if igToken == "" {
		igToken = env["FACEBOOK_ACCESS_TOKEN"]
	}
	if igUserID == "" {
		issues = append(issues, "Instagram: Missing INSTAGRAM_USER_ID")
	}
	if igToken == "" {
		issues = append(issues, "Instagram: Missing access token")
	}

	// YouTube
	ytChannel := env["YOUTUBE_CHANNEL_ID"]
	if ytChannel == "" {
		issues = append(issues, "YouTube: Missing YOUTUBE_CHANNEL_ID")
	}

	return len(issues) == 0, issues
}

// checkSite checks that the website returns 200 OK
func checkSite(siteURL string) (bool, string) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(siteURL)
	if err != nil {
		return false, fmt.Sprintf("Site unreachable: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		return true, fmt.Sprintf("Site returns 200 OK (%d bytes)", len(body))
	}
	return false, fmt.Sprintf("Site returns %d", resp.StatusCode)
}

// checkXAPI checks X/Twitter API connection
func checkXAPI(project string) (bool, string) {
	env := loadEnv(project)

	if env["X_API_KEY"] == "" || env["X_API_SECRET"] == "" || env["X_ACCESS_TOKEN"] == "" || env["X_ACCESS_SECRET"] == "" {
		return false, "Missing credentials"
	}

	return true, "Credentials present"
}

// checkFacebookAPI checks Facebook API connection
func checkFacebookAPI(project string) (bool, string) {
	env := loadEnv(project)

	pageID := env["FACEBOOK_PAGE_ID"]
	accessToken := env["FACEBOOK_ACCESS_TOKEN"]

	if pageID == "" || accessToken == "" {
		return false, "Missing credentials"
	}

	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s", pageID)
	params := fmt.Sprintf("fields=name&access_token=%s", accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url + "?" + params)
	if err != nil {
		return false, fmt.Sprintf("API error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("API error: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Sprintf("API error: %v", err)
	}

	if name, ok := result["name"].(string); ok {
		return true, fmt.Sprintf("Connected to page: %s", name)
	}

	return true, "Connected"
}

// checkInstagramAPI checks Instagram API connection
func checkInstagramAPI(project string) (bool, string) {
	env := loadEnv(project)

	igUserID := env["INSTAGRAM_USER_ID"]
	accessToken := env["INSTAGRAM_ACCESS_TOKEN"]
	if accessToken == "" {
		accessToken = env["FACEBOOK_ACCESS_TOKEN"]
	}

	if igUserID == "" || accessToken == "" {
		return false, "Missing credentials"
	}

	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s", igUserID)
	params := fmt.Sprintf("fields=username&access_token=%s", accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url + "?" + params)
	if err != nil {
		return false, fmt.Sprintf("API error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("API error: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Sprintf("API error: %v", err)
	}

	if username, ok := result["username"].(string); ok {
		return true, fmt.Sprintf("Connected as @%s", username)
	}

	return true, "Connected"
}

// checkTokens validates tokens for specified platforms
func checkTokens(project string, platforms []string) (*TokenStatus, error) {
	status := &TokenStatus{}

	// Default to all platforms if none specified
	checkAll := len(platforms) == 0
	checkX := checkAll || contains(platforms, "x") || contains(platforms, "twitter")
	checkFB := checkAll || contains(platforms, "facebook") || contains(platforms, "fb")
	checkIG := checkAll || contains(platforms, "instagram") || contains(platforms, "ig")

	// Check X/Twitter
	if checkX {
		status.X = &PlatformTokenStatus{}
		profile, err := showXProfile(project)
		if err != nil {
			status.X.Valid = false
			status.X.Error = err.Error()
		} else {
			status.X.Valid = true
			if username, ok := profile["username"].(string); ok {
				status.X.Handle = "@" + username
			}
		}
	}

	// Check Facebook
	if checkFB {
		status.Facebook = &PlatformTokenStatus{}
		env := loadEnv(project)
		profile, err := showFacebookPage(project)
		if err != nil {
			status.Facebook.Valid = false
			status.Facebook.Error = err.Error()
		} else {
			status.Facebook.Valid = true
			if name, ok := profile["name"].(string); ok {
				status.Facebook.Page = name
			}

			// Try to get token expiry
			appID := env["FACEBOOK_APP_ID"]
			appSecret := env["FACEBOOK_APP_SECRET"]
			accessToken := env["FACEBOOK_ACCESS_TOKEN"]
			if appID != "" && appSecret != "" && accessToken != "" {
				if days, err := debugFacebookToken(accessToken, appID, appSecret); err == nil {
					status.Facebook.ExpiresInDays = days
				}
			}
		}
	}

	// Check Instagram
	if checkIG {
		status.Instagram = &PlatformTokenStatus{}
		profile, err := showInstagramProfile(project)
		if err != nil {
			status.Instagram.Valid = false
			status.Instagram.Error = err.Error()
		} else {
			status.Instagram.Valid = true
			if username, ok := profile["username"].(string); ok {
				status.Instagram.Handle = "@" + username
			}
		}
	}

	return status, nil
}

// runPrelaunchCheck runs comprehensive prelaunch checks
func runPrelaunchCheck(project, campaign string) (*PrelaunchResult, error) {
	result := &PrelaunchResult{
		Passed: true,
		Checks: make(map[string]*CheckResult),
	}

	env := loadEnv(project)

	// Check X credentials
	xOk, xMsg := checkXAPI(project)
	if xOk {
		result.Checks["x_credentials"] = &CheckResult{Status: "pass", Message: xMsg}
	} else {
		result.Checks["x_credentials"] = &CheckResult{Status: "fail", Message: xMsg}
		result.Errors++
		result.Passed = false
	}

	// Check X API (actually try to connect)
	if xOk {
		_, err := showXProfile(project)
		if err != nil {
			result.Checks["x_api"] = &CheckResult{Status: "fail", Message: err.Error()}
			result.Errors++
			result.Passed = false
		} else {
			result.Checks["x_api"] = &CheckResult{Status: "pass", Message: "API connection verified"}
		}
	}

	// Check Facebook credentials
	fbOk, fbMsg := checkFacebookAPI(project)
	if fbOk {
		result.Checks["facebook_credentials"] = &CheckResult{Status: "pass", Message: fbMsg}
	} else {
		result.Checks["facebook_credentials"] = &CheckResult{Status: "fail", Message: fbMsg}
		result.Errors++
		result.Passed = false
	}

	// Check Facebook API
	if fbOk {
		_, err := showFacebookPage(project)
		if err != nil {
			result.Checks["facebook_api"] = &CheckResult{Status: "fail", Message: err.Error()}
			result.Errors++
			result.Passed = false
		} else {
			result.Checks["facebook_api"] = &CheckResult{Status: "pass", Message: "API connection verified"}
		}
	}

	// Check Facebook token expiry
	appID := env["FACEBOOK_APP_ID"]
	appSecret := env["FACEBOOK_APP_SECRET"]
	accessToken := env["FACEBOOK_ACCESS_TOKEN"]
	if appID != "" && appSecret != "" && accessToken != "" {
		days, err := debugFacebookToken(accessToken, appID, appSecret)
		if err != nil {
			result.Checks["facebook_token_expiry"] = &CheckResult{Status: "warn", Message: "Could not check token expiry"}
			result.Warnings++
		} else if days < 0 {
			result.Checks["facebook_token_expiry"] = &CheckResult{Status: "pass", Message: "Token does not expire"}
		} else if days <= 7 {
			result.Checks["facebook_token_expiry"] = &CheckResult{Status: "warn", Message: fmt.Sprintf("Token expires in %d days - refresh soon!", days)}
			result.Warnings++
		} else if days <= 30 {
			result.Checks["facebook_token_expiry"] = &CheckResult{Status: "warn", Message: fmt.Sprintf("Token expires in %d days", days)}
			result.Warnings++
		} else {
			result.Checks["facebook_token_expiry"] = &CheckResult{Status: "pass", Message: fmt.Sprintf("Token valid for %d days", days)}
		}
	}

	// Check Instagram credentials
	igOk, igMsg := checkInstagramAPI(project)
	if igOk {
		result.Checks["instagram_credentials"] = &CheckResult{Status: "pass", Message: igMsg}
	} else {
		result.Checks["instagram_credentials"] = &CheckResult{Status: "fail", Message: igMsg}
		result.Errors++
		result.Passed = false
	}

	// Check Instagram API
	if igOk {
		_, err := showInstagramProfile(project)
		if err != nil {
			result.Checks["instagram_api"] = &CheckResult{Status: "fail", Message: err.Error()}
			result.Errors++
			result.Passed = false
		} else {
			result.Checks["instagram_api"] = &CheckResult{Status: "pass", Message: "API connection verified"}
		}
	}

	// Check campaign if specified
	if campaign != "" {
		campOk, campIssues := checkCampaign(project, campaign)
		if campOk {
			result.Checks["campaign_structure"] = &CheckResult{Status: "pass", Message: "Campaign validated"}
		} else {
			result.Checks["campaign_structure"] = &CheckResult{Status: "fail", Message: strings.Join(campIssues, "; ")}
			result.Errors++
			result.Passed = false
		}
	}

	return result, nil
}

// contains checks if a slice contains a string
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if strings.EqualFold(item, s) {
			return true
		}
	}
	return false
}

// checkCampaign checks campaign file exists and is valid
func checkCampaign(project, campaign string) (bool, []string) {
	campaignPath := filepath.Join("projects", project, "campaigns", campaign+".json")
	issues := []string{}

	if _, err := os.Stat(campaignPath); os.IsNotExist(err) {
		return false, []string{fmt.Sprintf("Campaign file not found: %s", campaignPath)}
	}

	data, err := os.ReadFile(campaignPath)
	if err != nil {
		return false, []string{fmt.Sprintf("Failed to read campaign file: %v", err)}
	}

	var campaignData map[string]interface{}
	if err := json.Unmarshal(data, &campaignData); err != nil {
		return false, []string{fmt.Sprintf("Invalid JSON: %v", err)}
	}

	posts, ok := campaignData["posts"].([]interface{})
	if !ok || len(posts) == 0 {
		issues = append(issues, "No posts in campaign")
	}

	draftCount := 0
	for _, postInterface := range posts {
		post, ok := postInterface.(map[string]interface{})
		if !ok {
			continue
		}

		if id, ok := post["id"].(string); !ok || id == "" {
			issues = append(issues, "Post missing UUID")
		}

		if status, ok := post["status"].(string); ok && status == "draft" {
			draftCount++
			hasContent := false
			if twitter, ok := post["twitter"].(string); ok && twitter != "" {
				hasContent = true
			}
			if facebook, ok := post["facebook"].(string); ok && facebook != "" {
				hasContent = true
			}
			if instagram, ok := post["instagram_caption"].(string); ok && instagram != "" {
				hasContent = true
			}
			if !hasContent {
				id := "?"
				if postID, ok := post["id"].(string); ok {
					if len(postID) > 8 {
						id = postID[:8]
					} else {
						id = postID
					}
				}
				issues = append(issues, fmt.Sprintf("Post %s has no content", id))
			}
		}
	}

	if draftCount == 0 {
		issues = append(issues, "No draft posts ready to publish")
	}

	return len(issues) == 0, issues
}
