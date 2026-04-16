package youtube

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/api/youtube/v3"
)

// UploadVideo uploads a video to YouTube using the YouTube Data API v3
// If ReplaceID is provided, deletes that video AFTER successful upload (to preserve original if upload fails)
func UploadVideo(ctx context.Context, authConfig AuthConfig, params UploadParams, dryRun bool) (*UploadResult, error) {
	// Validate required fields
	if params.FilePath == "" {
		return nil, fmt.Errorf("file path is required")
	}
	if params.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	// Check file exists
	if _, err := os.Stat(params.FilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("video file not found: %s", params.FilePath)
	}

	// Apply defaults
	privacy := params.Privacy
	if privacy == "" {
		privacy = DefaultPrivacy
	}
	categoryID := params.CategoryID
	if categoryID == "" {
		categoryID = DefaultCategoryID
	}

	if dryRun {
		result := &UploadResult{
			Success: true,
			DryRun:  true,
		}
		if params.ReplaceID != "" {
			result.ReplacedVideoID = params.ReplaceID
		}
		return result, nil
	}

	// Get authenticated YouTube service
	service, err := GetYouTubeService(ctx, authConfig)
	if err != nil {
		return &UploadResult{
			Success: false,
			Error:   fmt.Sprintf("authentication failed: %v", err),
		}, nil
	}

	// Open video file
	file, err := os.Open(params.FilePath)
	if err != nil {
		return &UploadResult{
			Success: false,
			Error:   fmt.Sprintf("failed to open video file: %v", err),
		}, nil
	}
	defer file.Close()

	// Build Video resource
	upload := &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:       params.Title,
			Description: params.Description,
			Tags:        params.Tags,
			CategoryId:  categoryID,
		},
		Status: &youtube.VideoStatus{
			PrivacyStatus:           privacy,
			SelfDeclaredMadeForKids: params.MadeForKids,
		},
	}

	// Create insert call
	call := service.Videos.Insert([]string{"snippet", "status"}, upload)
	call.Media(file)

	// Execute upload FIRST (before any delete)
	response, err := call.Do()
	if err != nil {
		return &UploadResult{
			Success: false,
			Error:   fmt.Sprintf("upload failed: %v", err),
		}, nil
	}

	result := &UploadResult{
		Success:  true,
		VideoID:  response.Id,
		VideoURL: fmt.Sprintf("https://youtube.com/watch?v=%s", response.Id),
	}

	// Handle replace_id - delete old video AFTER successful upload
	// This ensures we don't lose the original if upload fails (e.g., quota exceeded)
	if params.ReplaceID != "" {
		err := service.Videos.Delete(params.ReplaceID).Do()
		if err != nil {
			// Log warning but don't fail - upload succeeded
			fmt.Fprintf(os.Stderr, "Warning: failed to delete old video %s: %v\n", params.ReplaceID, err)
		} else {
			result.ReplacedVideoID = params.ReplaceID
		}
	}

	// Add to playlist if specified
	if params.PlaylistID != "" {
		if err := AddToPlaylist(ctx, authConfig, response.Id, params.PlaylistID); err != nil {
			// Log warning but don't fail the upload
			fmt.Fprintf(os.Stderr, "Warning: failed to add to playlist: %v\n", err)
		}
	}

	return result, nil
}

// DeleteVideo deletes a video from YouTube by its ID
func DeleteVideo(ctx context.Context, authConfig AuthConfig, videoID string) error {
	service, err := GetYouTubeService(ctx, authConfig)
	if err != nil {
		return fmt.Errorf("failed to get YouTube service: %w", err)
	}

	if err := service.Videos.Delete(videoID).Do(); err != nil {
		return fmt.Errorf("failed to delete video %s: %w", videoID, err)
	}

	return nil
}

// AddToPlaylist adds an uploaded video to a playlist
func AddToPlaylist(ctx context.Context, authConfig AuthConfig, videoID, playlistID string) error {
	if playlistID == "" {
		return nil // No playlist specified
	}

	service, err := GetYouTubeService(ctx, authConfig)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	playlistItem := &youtube.PlaylistItem{
		Snippet: &youtube.PlaylistItemSnippet{
			PlaylistId: playlistID,
			ResourceId: &youtube.ResourceId{
				Kind:    "youtube#video",
				VideoId: videoID,
			},
		},
	}

	_, err = service.PlaylistItems.Insert([]string{"snippet"}, playlistItem).Do()
	if err != nil {
		return fmt.Errorf("failed to add to playlist: %w", err)
	}

	return nil
}
