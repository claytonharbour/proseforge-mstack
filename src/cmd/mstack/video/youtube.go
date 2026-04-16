package video

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/claytonharbour/proseforge-mstack/src/internal/google"
	"github.com/claytonharbour/proseforge-mstack/src/internal/youtube"
	"github.com/spf13/cobra"
)

func NewYouTubeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "youtube",
		Short: "YouTube video management",
		Long:  "Upload videos to YouTube and manage video metadata. Uses shared Google credentials from ~/.mstack/projects/<project>/google/",
	}

	cmd.PersistentFlags().Bool("dry-run", false, "Preview without making changes")
	cmd.PersistentFlags().String("channel", "", "Channel ID (required if multiple channels configured)")

	cmd.AddCommand(newYouTubeUploadCmd())
	cmd.AddCommand(newYouTubeListCmd())
	cmd.AddCommand(newYouTubeAuthCmd())
	cmd.AddCommand(newYouTubeDeleteCmd())
	cmd.AddCommand(NewBatchUploadCmd())
	cmd.AddCommand(newYouTubeChannelsCmd())

	return cmd
}

func newYouTubeUploadCmd() *cobra.Command {
	var (
		filePath    string
		title       string
		description string
		tags        string
		privacy     string
		categoryID  string
		madeForKids bool
		playlistID  string
		replaceID   string
	)

	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload video to YouTube",
		Long: `Upload a video to YouTube.

Examples:
  # Upload a video
  mstack video youtube upload --file=/path/to/video.mp4 --title="My Video"

  # Upload with all options
  mstack video youtube upload \
    --file=/path/to/video.mp4 \
    --title="Tutorial Video" \
    --description="A helpful tutorial" \
    --tags="tutorial,demo" \
    --privacy=public \
    --playlist=PLxxxx

  # Replace an existing video
  mstack video youtube upload \
    --file=/path/to/new-video.mp4 \
    --title="Updated Tutorial" \
    --replace-id=oaxKOkvAu8A

  # Dry run to preview
  mstack video youtube upload --file=video.mp4 --title="Test" --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			channelFlag, _ := cmd.Flags().GetString("channel")
			project, _ := cmd.Flags().GetString("project")

			// Resolve auth config
			authConfig, err := youtube.ResolveAuthConfig(project, channelFlag)
			if err != nil {
				if multiErr, ok := err.(*youtube.MultipleChannelsError); ok {
					return fmt.Errorf("multiple channels configured: %v\nSpecify with --channel flag", multiErr.Channels)
				}
				return fmt.Errorf("failed to resolve credentials: %w", err)
			}
			if authConfig == nil {
				return fmt.Errorf("no Google auth configured. Run 'mstack google auth' first")
			}

			// Validate required fields
			if filePath == "" {
				return fmt.Errorf("--file is required")
			}
			if title == "" {
				return fmt.Errorf("--title is required")
			}

			// Build upload params
			params := youtube.UploadParams{
				FilePath:    filePath,
				Title:       title,
				Description: description,
				Privacy:     privacy,
				CategoryID:  categoryID,
				MadeForKids: madeForKids,
				PlaylistID:  playlistID,
				ReplaceID:   replaceID,
			}

			if tags != "" {
				for _, t := range strings.Split(tags, ",") {
					params.Tags = append(params.Tags, strings.TrimSpace(t))
				}
			}

			ctx := context.Background()
			result, err := youtube.UploadVideo(ctx, *authConfig, params, dryRun)
			if err != nil {
				return err
			}

			return printJSON(result)
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "Path to video file (required)")
	cmd.Flags().StringVar(&title, "title", "", "Video title (required)")
	cmd.Flags().StringVar(&description, "description", "", "Video description")
	cmd.Flags().StringVar(&tags, "tags", "", "Comma-separated tags")
	cmd.Flags().StringVar(&privacy, "privacy", "", "Privacy: public, private, unlisted (default: unlisted)")
	cmd.Flags().StringVar(&categoryID, "category", "", "YouTube category ID (default: 28 - Science & Technology)")
	cmd.Flags().BoolVar(&madeForKids, "made-for-kids", false, "COPPA compliance - is this video made for kids?")
	cmd.Flags().StringVar(&playlistID, "playlist", "", "Add to this playlist after upload")
	cmd.Flags().StringVar(&replaceID, "replace-id", "", "Delete this YouTube video ID before uploading")

	return cmd
}

func newYouTubeListCmd() *cobra.Command {
	var (
		maxResults int64
		search     string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List videos from YouTube channel",
		Long: `List videos from the authenticated YouTube channel (queries YouTube API directly).

Examples:
  # List recent videos
  mstack video youtube list

  # Limit results
  mstack video youtube list --max-results=10

  # Search by title
  mstack video youtube list --search="tutorial"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			channelFlag, _ := cmd.Flags().GetString("channel")
			project, _ := cmd.Flags().GetString("project")

			// Resolve auth config
			authConfig, err := youtube.ResolveAuthConfig(project, channelFlag)
			if err != nil {
				if multiErr, ok := err.(*youtube.MultipleChannelsError); ok {
					return fmt.Errorf("multiple channels configured: %v\nSpecify with --channel flag", multiErr.Channels)
				}
				return fmt.Errorf("failed to resolve credentials: %w", err)
			}
			if authConfig == nil {
				return fmt.Errorf("no Google auth configured. Run 'mstack google auth' first")
			}

			ctx := context.Background()
			var videos []youtube.YouTubeVideo

			if search != "" {
				videos, err = youtube.SearchChannelVideos(ctx, *authConfig, search, maxResults)
			} else {
				videos, err = youtube.ListChannelVideos(ctx, *authConfig, maxResults)
			}

			if err != nil {
				return fmt.Errorf("failed to list videos: %w", err)
			}

			return printJSON(videos)
		},
	}

	cmd.Flags().Int64Var(&maxResults, "max-results", 50, "Maximum videos to return (max: 50)")
	cmd.Flags().StringVar(&search, "search", "", "Search query to filter videos by title")

	return cmd
}

func newYouTubeAuthCmd() *cobra.Command {
	var check bool

	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Check YouTube authentication status",
		Long: `Check YouTube authentication status. YouTube uses shared Google credentials.

To authenticate, run: mstack google auth --client-secret=/path/to/client_secret.json

Examples:
  # Check auth status
  mstack video youtube auth --check

  # Check for specific project
  mstack video youtube auth --check --project=proseforge`,
		RunE: func(cmd *cobra.Command, args []string) error {
			channelFlag, _ := cmd.Flags().GetString("channel")
			project, _ := cmd.Flags().GetString("project")
			if project == "" {
				project = "proseforge"
			}

			// Check existing token
			authConfig, err := youtube.ResolveAuthConfig(project, channelFlag)
			if err != nil {
				if multiErr, ok := err.(*youtube.MultipleChannelsError); ok {
					return fmt.Errorf("multiple channels configured: %v\nSpecify with --channel flag", multiErr.Channels)
				}
				return fmt.Errorf("failed to resolve credentials: %w", err)
			}
			if authConfig == nil {
				fmt.Fprintf(os.Stderr, "No Google auth configured for project '%s'.\n", project)
				fmt.Fprintf(os.Stderr, "Run: mstack google auth --client-secret=/path/to/client_secret.json --project=%s\n", project)
				return printJSON(map[string]interface{}{
					"valid":   false,
					"status":  "No credentials configured",
					"project": project,
				})
			}

			valid, status := youtube.CheckToken(*authConfig)

			// Try to load or fetch channel info
			var channelInfo *youtube.ChannelInfo
			if authConfig.ChannelInfoPath != "" {
				if info, err := youtube.LoadChannelInfo(*authConfig); err == nil {
					channelInfo = info
				}
			}

			// If no cached channel info but we have valid auth, try to fetch it
			if channelInfo == nil && valid {
				ctx := context.Background()
				if info, err := youtube.FetchAndCacheChannelInfo(ctx, *authConfig); err == nil {
					channelInfo = info
					fmt.Fprintf(os.Stderr, "Cached channel info for: %s\n", info.Title)
				}
			}

			result := map[string]interface{}{
				"valid":   valid,
				"status":  status,
				"project": project,
				"paths": map[string]string{
					"client_secret": authConfig.ClientSecretPath,
					"token_file":    authConfig.TokenFilePath,
				},
			}
			if channelInfo != nil {
				result["channel_id"] = channelInfo.ID
				result["channel_title"] = channelInfo.Title
				if channelInfo.CustomURL != "" {
					result["channel_url"] = channelInfo.CustomURL
				}
			}

			return printJSON(result)
		},
	}

	cmd.Flags().BoolVar(&check, "check", false, "Check token status (default behavior)")

	return cmd
}

func newYouTubeDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [video-id]",
		Short: "Delete a video from YouTube",
		Long: `Delete a video from YouTube by video ID.

Examples:
  # Delete by YouTube video ID
  mstack video youtube delete oaxKOkvAu8A

  # Dry run to preview
  mstack video youtube delete oaxKOkvAu8A --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			channelFlag, _ := cmd.Flags().GetString("channel")
			project, _ := cmd.Flags().GetString("project")

			// Resolve auth config
			authConfig, err := youtube.ResolveAuthConfig(project, channelFlag)
			if err != nil {
				if multiErr, ok := err.(*youtube.MultipleChannelsError); ok {
					return fmt.Errorf("multiple channels configured: %v\nSpecify with --channel flag", multiErr.Channels)
				}
				return fmt.Errorf("failed to resolve credentials: %w", err)
			}
			if authConfig == nil {
				return fmt.Errorf("no Google auth configured. Run 'mstack google auth' first")
			}

			videoID := args[0]

			if dryRun {
				fmt.Printf("Would delete video: %s\n", videoID)
				return nil
			}

			ctx := context.Background()
			fmt.Printf("Deleting video: %s\n", videoID)
			if err := youtube.DeleteVideo(ctx, *authConfig, videoID); err != nil {
				return err
			}

			fmt.Println("Video deleted successfully")
			return nil
		},
	}

	return cmd
}

func newYouTubeChannelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "List authenticated YouTube channels",
		Long: `List YouTube channels that have cached info for this project.

Examples:
  # List channels
  mstack video youtube channels

  # List for specific project
  mstack video youtube channels --project=proseforge`,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			if project == "" {
				project = "proseforge"
			}

			channels, err := youtube.ListChannels(project)
			if err != nil {
				return fmt.Errorf("failed to list channels: %w", err)
			}

			if len(channels) == 0 {
				// Check if we have auth but no cached channels
				googleConfig := google.AuthConfigForProject(project)
				if _, err := os.Stat(googleConfig.TokenFilePath); err == nil {
					fmt.Fprintf(os.Stderr, "Google auth found but no channel info cached.\n")
					fmt.Fprintf(os.Stderr, "Run 'mstack video youtube auth' to fetch channel info.\n")
				} else {
					fmt.Fprintf(os.Stderr, "No Google auth configured for project '%s'.\n", project)
					fmt.Fprintf(os.Stderr, "Run: mstack google auth --client-secret=/path/to/client_secret.json\n")
				}
				return printJSON([]string{})
			}

			// Load full channel info for each
			var results []map[string]string
			for _, channelID := range channels {
				config := youtube.AuthConfigForProjectAndChannel(project, channelID)
				info, _ := youtube.LoadChannelInfo(config)
				entry := map[string]string{
					"channel_id": channelID,
				}
				if info != nil {
					entry["title"] = info.Title
					if info.CustomURL != "" {
						entry["url"] = info.CustomURL
					}
				}
				results = append(results, entry)
			}

			return printJSON(results)
		},
	}

	return cmd
}

func printJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
