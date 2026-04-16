package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/claytonharbour/proseforge-mstack/src/internal/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// GetClient returns an authenticated HTTP client for YouTube API
// Uses the shared Google auth from the google package
func GetClient(ctx context.Context, authConfig AuthConfig) (*http.Client, error) {
	return google.GetClientForProject(ctx, authConfig.Project)
}

// GetYouTubeService returns an authenticated YouTube API service
func GetYouTubeService(ctx context.Context, authConfig AuthConfig) (*youtube.Service, error) {
	client, err := GetClient(ctx, authConfig)
	if err != nil {
		return nil, err
	}

	service, err := youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create YouTube service: %w", err)
	}

	return service, nil
}

// FetchAndCacheChannelInfo fetches channel info from YouTube API and caches it
func FetchAndCacheChannelInfo(ctx context.Context, authConfig AuthConfig) (*ChannelInfo, error) {
	service, err := GetYouTubeService(ctx, authConfig)
	if err != nil {
		return nil, err
	}

	// Fetch channel info
	channelInfo, err := fetchChannelInfo(service)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channel info: %w", err)
	}

	// Create youtube metadata directory
	youtubeDir := GetYouTubeDir(authConfig.Project)
	if err := os.MkdirAll(youtubeDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create youtube directory: %w", err)
	}

	// Save channel info
	channelInfoPath := GetChannelInfoPath(authConfig.Project, channelInfo.ID)
	if err := saveChannelInfo(channelInfoPath, channelInfo); err != nil {
		return nil, fmt.Errorf("failed to save channel info: %w", err)
	}

	return channelInfo, nil
}

// fetchChannelInfo gets the authenticated user's channel information
func fetchChannelInfo(service *youtube.Service) (*ChannelInfo, error) {
	call := service.Channels.List([]string{"snippet"}).Mine(true)
	response, err := call.Do()
	if err != nil {
		return nil, err
	}

	if len(response.Items) == 0 {
		return nil, fmt.Errorf("no channel found for authenticated user")
	}

	channel := response.Items[0]
	return &ChannelInfo{
		ID:        channel.Id,
		Title:     channel.Snippet.Title,
		CustomURL: channel.Snippet.CustomUrl,
	}, nil
}

// LoadChannelInfo loads cached channel info from disk
func LoadChannelInfo(authConfig AuthConfig) (*ChannelInfo, error) {
	if authConfig.ChannelInfoPath == "" {
		return nil, fmt.Errorf("no channel info path configured")
	}

	data, err := os.ReadFile(authConfig.ChannelInfoPath)
	if err != nil {
		return nil, err
	}

	var info ChannelInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// saveChannelInfo saves channel info to disk
func saveChannelInfo(path string, info *ChannelInfo) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// CheckToken checks if a valid token exists and reports its status
func CheckToken(authConfig AuthConfig) (bool, string) {
	token, err := google.LoadToken(authConfig.TokenFilePath)
	if err != nil {
		return false, fmt.Sprintf("No token found: %v", err)
	}

	if token.Expiry.Before(time.Now()) {
		if token.RefreshToken != "" {
			return true, "Token expired but has refresh token (will auto-refresh)"
		}
		return false, "Token expired and no refresh token available"
	}

	return true, fmt.Sprintf("Token valid until %s", token.Expiry.Format(time.RFC3339))
}

// CheckAuthStatus returns detailed auth status for a project
func CheckAuthStatus(ctx context.Context, project string) (*AuthStatus, error) {
	authConfig := AuthConfigForProject(project)

	// Check if token file exists
	if _, err := os.Stat(authConfig.TokenFilePath); os.IsNotExist(err) {
		return &AuthStatus{
			Authenticated: false,
			Status:        "No credentials found. Run 'mstack google auth' to authenticate.",
			Project:       project,
		}, nil
	}

	valid, status := CheckToken(authConfig)

	authStatus := &AuthStatus{
		Authenticated: valid,
		Status:        status,
		Project:       project,
	}

	// Try to get token expiry
	if token, err := google.LoadToken(authConfig.TokenFilePath); err == nil && !token.Expiry.IsZero() {
		authStatus.TokenExpiry = token.Expiry.Format(time.RFC3339)
	}

	return authStatus, nil
}

// AuthStatus represents the current authentication status
type AuthStatus struct {
	Authenticated bool   `json:"authenticated"`
	Status        string `json:"status"`
	TokenExpiry   string `json:"token_expiry,omitempty"`
	Project       string `json:"project"`
}
