package youtube

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/youtube/v3"
)

// ListChannelVideos returns videos from the authenticated channel
func ListChannelVideos(ctx context.Context, authConfig AuthConfig, maxResults int64) ([]YouTubeVideo, error) {
	if maxResults <= 0 {
		maxResults = 50 // Default
	}
	if maxResults > 50 {
		maxResults = 50 // YouTube API max per page
	}

	service, err := GetYouTubeService(ctx, authConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get YouTube service: %w", err)
	}

	// First get the channel's uploads playlist
	channelsCall := service.Channels.List([]string{"contentDetails"}).Mine(true)
	channelsResponse, err := channelsCall.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get channel info: %w", err)
	}

	if len(channelsResponse.Items) == 0 {
		return nil, fmt.Errorf("no channel found for authenticated user")
	}

	uploadsPlaylistID := channelsResponse.Items[0].ContentDetails.RelatedPlaylists.Uploads

	// Get videos from uploads playlist
	playlistCall := service.PlaylistItems.List([]string{"snippet", "contentDetails"}).
		PlaylistId(uploadsPlaylistID).
		MaxResults(maxResults)

	playlistResponse, err := playlistCall.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get uploads: %w", err)
	}

	// Get video IDs for status lookup
	var videoIDs []string
	for _, item := range playlistResponse.Items {
		videoIDs = append(videoIDs, item.ContentDetails.VideoId)
	}

	// Get video details including status
	var privacyMap map[string]string
	if len(videoIDs) > 0 {
		privacyMap, err = getVideoPrivacy(ctx, service, videoIDs)
		if err != nil {
			// Log but continue - privacy info is optional
			fmt.Printf("Warning: could not get video privacy info: %v\n", err)
			privacyMap = make(map[string]string)
		}
	}

	// Convert to our type
	var videos []YouTubeVideo
	for _, item := range playlistResponse.Items {
		publishedAt, _ := time.Parse(time.RFC3339, item.Snippet.PublishedAt)

		video := YouTubeVideo{
			ID:          item.ContentDetails.VideoId,
			Title:       item.Snippet.Title,
			Description: item.Snippet.Description,
			PublishedAt: publishedAt,
			Privacy:     privacyMap[item.ContentDetails.VideoId],
		}

		// Get thumbnail (prefer medium, fall back to default)
		if item.Snippet.Thumbnails != nil {
			if item.Snippet.Thumbnails.Medium != nil {
				video.ThumbnailURL = item.Snippet.Thumbnails.Medium.Url
			} else if item.Snippet.Thumbnails.Default != nil {
				video.ThumbnailURL = item.Snippet.Thumbnails.Default.Url
			}
		}

		videos = append(videos, video)
	}

	return videos, nil
}

// getVideoPrivacy fetches privacy status for multiple videos
func getVideoPrivacy(ctx context.Context, service *youtube.Service, videoIDs []string) (map[string]string, error) {
	result := make(map[string]string)

	// YouTube API allows up to 50 IDs per request
	call := service.Videos.List([]string{"status"}).Id(videoIDs...)
	response, err := call.Do()
	if err != nil {
		return nil, err
	}

	for _, item := range response.Items {
		result[item.Id] = item.Status.PrivacyStatus
	}

	return result, nil
}

// SearchChannelVideos searches for videos by title in the authenticated channel
func SearchChannelVideos(ctx context.Context, authConfig AuthConfig, query string, maxResults int64) ([]YouTubeVideo, error) {
	if maxResults <= 0 {
		maxResults = 25
	}
	if maxResults > 50 {
		maxResults = 50
	}

	service, err := GetYouTubeService(ctx, authConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get YouTube service: %w", err)
	}

	// Search for videos
	searchCall := service.Search.List([]string{"snippet"}).
		ForMine(true).
		Type("video").
		Q(query).
		MaxResults(maxResults)

	searchResponse, err := searchCall.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to search videos: %w", err)
	}

	// Get video IDs for status lookup
	var videoIDs []string
	for _, item := range searchResponse.Items {
		videoIDs = append(videoIDs, item.Id.VideoId)
	}

	// Get privacy status
	var privacyMap map[string]string
	if len(videoIDs) > 0 {
		privacyMap, err = getVideoPrivacy(ctx, service, videoIDs)
		if err != nil {
			privacyMap = make(map[string]string)
		}
	}

	// Convert to our type
	var videos []YouTubeVideo
	for _, item := range searchResponse.Items {
		publishedAt, _ := time.Parse(time.RFC3339, item.Snippet.PublishedAt)

		video := YouTubeVideo{
			ID:          item.Id.VideoId,
			Title:       item.Snippet.Title,
			Description: item.Snippet.Description,
			PublishedAt: publishedAt,
			Privacy:     privacyMap[item.Id.VideoId],
		}

		if item.Snippet.Thumbnails != nil && item.Snippet.Thumbnails.Medium != nil {
			video.ThumbnailURL = item.Snippet.Thumbnails.Medium.Url
		}

		videos = append(videos, video)
	}

	return videos, nil
}
