package social

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// loadCampaign loads campaign JSON file
func loadCampaign(project, campaign string) (map[string]interface{}, error) {
	campaignPath := filepath.Join(getProjectPath(project), "campaigns", campaign+".json")

	if _, err := os.Stat(campaignPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("campaign file not found: %s", campaignPath)
	}

	data, err := os.ReadFile(campaignPath)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON in campaign file: %w", err)
	}

	return result, nil
}

// saveCampaign saves campaign JSON file
func saveCampaign(project, campaign string, data map[string]interface{}) error {
	campaignPath := filepath.Join(getProjectPath(project), "campaigns", campaign+".json")

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(campaignPath, append(jsonData, '\n'), 0644)
}

// findPostByID finds a post by UUID in campaign data
func findPostByID(campaignData map[string]interface{}, postID string) map[string]interface{} {
	posts, ok := campaignData["posts"].([]interface{})
	if !ok {
		return nil
	}

	for _, postInterface := range posts {
		post, ok := postInterface.(map[string]interface{})
		if !ok {
			continue
		}
		if id, ok := post["id"].(string); ok && (id == postID || strings.HasPrefix(id, postID)) {
			return post
		}
	}

	return nil
}

// listPosts lists all posts in a campaign
func listPosts(project, campaign string) error {
	data, err := loadCampaign(project, campaign)
	if err != nil {
		return err
	}

	posts, ok := data["posts"].([]interface{})
	if !ok || len(posts) == 0 {
		fmt.Println("No posts in campaign")
		return nil
	}

	fmt.Printf("\nCampaign: %v\n", data["campaign"])
	fmt.Println(strings.Repeat("-", 60))

	for _, postInterface := range posts {
		post, ok := postInterface.(map[string]interface{})
		if !ok {
			continue
		}

		status := "unknown"
		if s, ok := post["status"].(string); ok {
			status = s
		}

		fmt.Printf("\n  ID: %v\n", post["id"])
		if desc, ok := post["description"].(string); ok {
			fmt.Printf("  Description: %s\n", desc)
		}
		fmt.Printf("  Status: %s\n", status)

		if postedAt, ok := post["posted_at"].(string); ok && postedAt != "" {
			fmt.Printf("  Posted: %s\n", postedAt)
		}

		if platformIDs, ok := post["platform_ids"].(map[string]interface{}); ok && len(platformIDs) > 0 {
			fmt.Printf("  Platform IDs: %v\n", platformIDs)
		}
	}

	return nil
}

// postCampaignItemWithResult posts a campaign post to all platforms and returns detailed result
func postCampaignItemWithResult(project, campaign, postID string, dryRun bool) (*CampaignPostResult, error) {
	data, err := loadCampaign(project, campaign)
	if err != nil {
		return nil, err
	}

	post := findPostByID(data, postID)
	if post == nil {
		return nil, fmt.Errorf("post not found with ID: %s", postID)
	}

	status := "unknown"
	if s, ok := post["status"].(string); ok {
		status = s
	}

	if status != "draft" {
		return nil, fmt.Errorf("post status is '%s' - can only post drafts", status)
	}

	result := &CampaignPostResult{
		Success:  true,
		PostedTo: make(map[string]*PlatformPost),
		Errors:   make(map[string]string),
		DryRun:   dryRun,
	}

	if dryRun {
		result.Status = "dry_run"
		// Just validate what would be posted
		if twitter, ok := post["twitter"].(string); ok && twitter != "" {
			result.PostedTo["x"] = &PlatformPost{ID: "(dry run)"}
		}
		if facebook, ok := post["facebook"].(string); ok && facebook != "" {
			result.PostedTo["facebook"] = &PlatformPost{ID: "(dry run)"}
		}
		if instagramCaption, ok := post["instagram_caption"].(string); ok && instagramCaption != "" {
			if _, ok := post["instagram_image"].(string); ok {
				result.PostedTo["instagram"] = &PlatformPost{ID: "(dry run)"}
			}
		}
		return result, nil
	}

	platformIDs := make(map[string]string)

	// Post to Twitter/X
	if twitter, ok := post["twitter"].(string); ok && twitter != "" {
		tweetID, err := postToX(project, twitter)
		if err != nil {
			result.Errors["x"] = err.Error()
			result.Success = false
		} else {
			platformIDs["twitter"] = tweetID
			result.PostedTo["x"] = &PlatformPost{
				ID:  tweetID,
				URL: fmt.Sprintf("https://x.com/i/status/%s", tweetID),
			}
		}
	}

	// Post to Facebook
	if facebook, ok := post["facebook"].(string); ok && facebook != "" {
		fbPostID, err := postToFacebook(project, facebook)
		if err != nil {
			result.Errors["facebook"] = err.Error()
			result.Success = false
		} else {
			platformIDs["facebook"] = fbPostID
			result.PostedTo["facebook"] = &PlatformPost{
				ID:  fbPostID,
				URL: fmt.Sprintf("https://facebook.com/%s", fbPostID),
			}
		}
	}

	// Post to Instagram
	if instagramCaption, ok := post["instagram_caption"].(string); ok && instagramCaption != "" {
		if instagramImage, ok := post["instagram_image"].(string); ok && instagramImage != "" {
			mediaID, err := postToInstagram(project, instagramCaption, instagramImage)
			if err != nil {
				result.Errors["instagram"] = err.Error()
				result.Success = false
			} else {
				platformIDs["instagram"] = mediaID
				result.PostedTo["instagram"] = &PlatformPost{
					ID: mediaID,
				}
			}
		}
	}

	if len(result.PostedTo) == 0 && len(result.Errors) == 0 {
		return nil, fmt.Errorf("no platform content found in post")
	}

	// Update campaign file if at least one platform succeeded
	if len(result.PostedTo) > 0 {
		post["status"] = "posted"
		post["posted_at"] = time.Now().UTC().Format(time.RFC3339)
		post["platform_ids"] = platformIDs
		saveCampaign(project, campaign, data)
		result.Status = "posted"
	} else {
		result.Status = "failed"
	}

	return result, nil
}

// postCampaignItem posts a campaign post to all platforms
func postCampaignItem(project, campaign, postID string) error {
	data, err := loadCampaign(project, campaign)
	if err != nil {
		return err
	}

	post := findPostByID(data, postID)
	if post == nil {
		return fmt.Errorf("post not found with ID: %s", postID)
	}

	status := "unknown"
	if s, ok := post["status"].(string); ok {
		status = s
	}

	if status != "draft" {
		return fmt.Errorf("post status is '%s' - can only post drafts", status)
	}

	platformIDs := make(map[string]string)

	// Post to Twitter/X
	if twitter, ok := post["twitter"].(string); ok && twitter != "" {
		tweetID, err := postToX(project, twitter)
		if err == nil {
			platformIDs["twitter"] = tweetID
		}
	}

	// Post to Facebook
	if facebook, ok := post["facebook"].(string); ok && facebook != "" {
		postID, err := postToFacebook(project, facebook)
		if err == nil {
			platformIDs["facebook"] = postID
		}
	}

	// Post to Instagram
	if instagramCaption, ok := post["instagram_caption"].(string); ok && instagramCaption != "" {
		if instagramImage, ok := post["instagram_image"].(string); ok && instagramImage != "" {
			mediaID, err := postToInstagram(project, instagramCaption, instagramImage)
			if err == nil {
				platformIDs["instagram"] = mediaID
			}
		}
	}

	if len(platformIDs) == 0 {
		return fmt.Errorf("no platform content found in post")
	}

	// Update campaign file
	post["status"] = "posted"
	post["posted_at"] = time.Now().UTC().Format(time.RFC3339)
	post["platform_ids"] = platformIDs

	return saveCampaign(project, campaign, data)
}
