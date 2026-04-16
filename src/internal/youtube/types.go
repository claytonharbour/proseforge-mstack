package youtube

import (
	"os"
	"path/filepath"
	"time"

	"github.com/claytonharbour/proseforge-mstack/src/internal/google"
)

// Default values for uploads
const (
	DefaultPrivacy     = "unlisted"
	DefaultCategoryID  = "28" // Science & Technology
	DefaultMadeForKids = false
)

// ChannelInfo represents cached channel information after auth
type ChannelInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CustomURL string `json:"custom_url,omitempty"`
}

// YouTubeVideo represents a video from the YouTube API
type YouTubeVideo struct {
	ID           string    `json:"youtube_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description,omitempty"`
	PublishedAt  time.Time `json:"published_at,omitempty"`
	Privacy      string    `json:"privacy,omitempty"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
}

// UploadParams holds all parameters for uploading a video
type UploadParams struct {
	FilePath    string   // Required: absolute path to video file
	Title       string   // Required: video title
	Description string   // Optional: video description
	Tags        []string // Optional: video tags
	Privacy     string   // Optional: public, private, unlisted (default: unlisted)
	CategoryID  string   // Optional: YouTube category ID (default: 28)
	MadeForKids bool     // Optional: COPPA compliance (default: false)
	PlaylistID  string   // Optional: add to playlist after upload
	ReplaceID   string   // Optional: delete this video ID before uploading
}

// UploadResult represents the result of a YouTube upload
type UploadResult struct {
	Success         bool   `json:"success"`
	VideoID         string `json:"video_id,omitempty"`
	VideoURL        string `json:"video_url,omitempty"`
	Error           string `json:"error,omitempty"`
	DryRun          bool   `json:"dry_run,omitempty"`
	ReplacedVideoID string `json:"replaced_video_id,omitempty"`
}

// AuthConfig holds paths for OAuth credentials (uses shared google auth)
type AuthConfig struct {
	Project          string // Project name (e.g., "proseforge")
	ChannelID        string // Channel ID (for channel-specific metadata)
	ClientSecretPath string // Path to client_secret.json (shared with Forms)
	TokenFilePath    string // Path to token.json (shared with Forms)
	ChannelInfoPath  string // Path to channel.json (YouTube-specific)
	CallbackPort     int    // OAuth callback port (default 8080)
}

// GetYouTubeDir returns the path to ~/.mstack/projects/<project>/google/youtube/
// Channel metadata is stored here
func GetYouTubeDir(project string) string {
	return google.GetYouTubeDir(project)
}

// GetChannelInfoPath returns the path to channel info for a specific channel
// ~/.mstack/projects/<project>/google/youtube/<channel_id>.json
func GetChannelInfoPath(project, channelID string) string {
	return filepath.Join(GetYouTubeDir(project), channelID+".json")
}

// AuthConfigForProject returns auth configuration for a specific project
// Uses shared Google credentials with YouTube-specific channel metadata
func AuthConfigForProject(project string) AuthConfig {
	googleConfig := google.AuthConfigForProject(project)
	return AuthConfig{
		Project:          project,
		ClientSecretPath: googleConfig.ClientSecretPath,
		TokenFilePath:    googleConfig.TokenFilePath,
		CallbackPort:     googleConfig.CallbackPort,
	}
}

// AuthConfigForProjectAndChannel returns auth config with channel-specific paths
func AuthConfigForProjectAndChannel(project, channelID string) AuthConfig {
	config := AuthConfigForProject(project)
	config.ChannelID = channelID
	config.ChannelInfoPath = GetChannelInfoPath(project, channelID)
	return config
}

// ListChannels returns all channel IDs with cached info for a project
func ListChannels(project string) ([]string, error) {
	youtubeDir := GetYouTubeDir(project)
	entries, err := os.ReadDir(youtubeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var channels []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			// Extract channel ID from filename (e.g., "<channel-id>.json")
			channelID := entry.Name()[:len(entry.Name())-5]
			channels = append(channels, channelID)
		}
	}
	return channels, nil
}

// ResolveAuthConfig resolves which project/channel to use
func ResolveAuthConfig(project, channelFlag string) (*AuthConfig, error) {
	// Use provided project or default to proseforge
	if project == "" {
		project = "proseforge"
	}

	// Check if we have Google auth for this project
	googleConfig := google.AuthConfigForProject(project)
	if _, err := os.Stat(googleConfig.TokenFilePath); os.IsNotExist(err) {
		return nil, nil // No auth configured
	}

	config := AuthConfigForProject(project)

	// If channel specified, use it
	if channelFlag != "" {
		config.ChannelID = channelFlag
		config.ChannelInfoPath = GetChannelInfoPath(project, channelFlag)
		return &config, nil
	}

	// Check environment variable
	if envChannel := os.Getenv("MSTACK_YOUTUBE_CHANNEL"); envChannel != "" {
		config.ChannelID = envChannel
		config.ChannelInfoPath = GetChannelInfoPath(project, envChannel)
		return &config, nil
	}

	// Check available channels
	channels, err := ListChannels(project)
	if err != nil {
		return nil, err
	}

	if len(channels) == 0 {
		// No channel info cached yet, but we have auth - caller should fetch channel info
		return &config, nil
	}

	if len(channels) == 1 {
		config.ChannelID = channels[0]
		config.ChannelInfoPath = GetChannelInfoPath(project, channels[0])
		return &config, nil
	}

	// Multiple channels - caller needs to specify
	return nil, &MultipleChannelsError{Channels: channels}
}

// MultipleChannelsError is returned when multiple channels exist but none specified
type MultipleChannelsError struct {
	Channels []string
}

func (e *MultipleChannelsError) Error() string {
	return "multiple YouTube channels configured, specify with --channel flag"
}

// ---- Legacy support for migration ----

// GetLegacyYouTubeDir returns the old path ~/.mstack/youtube/
func GetLegacyYouTubeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".mstack/youtube"
	}
	return filepath.Join(home, ".mstack", "youtube")
}

// GetLegacyChannelDir returns the old path ~/.mstack/youtube/<channel_id>/
func GetLegacyChannelDir(channelID string) string {
	return filepath.Join(GetLegacyYouTubeDir(), channelID)
}

// HasLegacyCredentials checks if old-style credentials exist
func HasLegacyCredentials() bool {
	legacyDir := GetLegacyYouTubeDir()
	entries, err := os.ReadDir(legacyDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			tokenPath := filepath.Join(legacyDir, entry.Name(), "youtube.token.json")
			if _, err := os.Stat(tokenPath); err == nil {
				return true
			}
		}
	}
	return false
}
