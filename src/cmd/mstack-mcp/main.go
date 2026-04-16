package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/claytonharbour/proseforge-mstack/src/internal/config"
	"github.com/claytonharbour/proseforge-mstack/src/internal/forms"
	"github.com/claytonharbour/proseforge-mstack/src/internal/google"
	"github.com/claytonharbour/proseforge-mstack/src/internal/social"
	"github.com/claytonharbour/proseforge-mstack/src/internal/validation"
	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/claytonharbour/proseforge-mstack/src/internal/youtube"
)

var version = "dev"

// MCP Protocol types
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type MCPNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCP Server info
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      ServerInfo             `json:"serverInfo"`
}

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CallToolResult struct {
	Content []TextContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// Services
var videoSvc = video.NewService()
var socialSvc = social.NewService()
var validationSvc = validation.NewService()
var formsSvc = forms.NewService()

func init() {
	config.Load()

	// Configure projects root from environment variable (for social only)
	if root := os.Getenv("MSTACK_PROJECTS_ROOT"); root != "" {
		social.SetProjectsRoot(root)
	}
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			sendError(nil, -32700, "Parse error", err.Error())
			continue
		}

		handleRequest(req)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
	}
}

func handleRequest(req MCPRequest) {
	switch req.Method {
	case "initialize":
		handleInitialize(req)
	case "initialized":
		// Notification, no response needed
	case "tools/list":
		handleToolsList(req)
	case "tools/call":
		handleToolsCall(req)
	default:
		sendError(req.ID, -32601, "Method not found", req.Method)
	}
}

func handleInitialize(req MCPRequest) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		ServerInfo: ServerInfo{
			Name:    "mstack",
			Version: version,
		},
	}
	sendResult(req.ID, result)
}

func handleToolsList(req MCPRequest) {
	tools := []Tool{
		{
			Name:        "video_parse",
			Description: "Parse narration markdown into JSON segments",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to narration.md file",
					},
				},
				"required": []string{"file_path"},
			},
		},
		{
			Name:        "video_analyze",
			Description: "Analyze video for timing overlaps between narration segments. Parses narration.md internally.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"narration_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to narration.md file (will be parsed internally)",
					},
					"audio_dir": map[string]interface{}{
						"type":        "string",
						"description": "Path to audio directory containing segment_XXX.m4a files",
					},
				},
				"required": []string{"narration_path", "audio_dir"},
			},
		},
		{
			Name:        "social_x_show",
			Description: "Show X/Twitter profile information",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name (default: proseforge)",
					},
				},
			},
		},
		{
			Name:        "social_x_post",
			Description: "Post a tweet to X/Twitter",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Tweet content",
					},
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name (default: proseforge)",
					},
				},
				"required": []string{"text"},
			},
		},
		{
			Name:        "social_fb_show",
			Description: "Show Facebook Page information",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name (default: proseforge)",
					},
				},
			},
		},
		{
			Name:        "social_fb_post",
			Description: "Post to Facebook Page",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Post content",
					},
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name (default: proseforge)",
					},
				},
				"required": []string{"text"},
			},
		},
		{
			Name:        "social_ig_show",
			Description: "Show Instagram profile information",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name (default: proseforge)",
					},
				},
			},
		},
		{
			Name:        "social_campaign_list",
			Description: "List posts in a campaign",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"campaign": map[string]interface{}{
						"type":        "string",
						"description": "Campaign name (default: launch)",
					},
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name (default: proseforge)",
					},
				},
			},
		},
		{
			Name:        "secrets_diff",
			Description: "Compare .env secrets with Bitwarden vault",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name (default: proseforge)",
					},
				},
			},
		},
		{
			Name:        "video_validate",
			Description: "Validate video content matches narration using OCR. Supports inline JSON tags for semantic validation: `{\"action\":\"click\",\"target\":\"Settings\"}`. Action types: click, fill, navigate, wait, select, hover, scroll, assert.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to video.webm file",
					},
					"narration_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to narration.md file (will be parsed internally)",
					},
					"script_path": map[string]interface{}{
						"type":        "string",
						"description": "Optional: Path to Playwright test file (.showcase.ts) for line number mapping in issues",
					},
					"frames": map[string]interface{}{
						"type":        "number",
						"description": "Number of frames per tagged segment (default: 3)",
					},
					"frame_offset": map[string]interface{}{
						"type":        "number",
						"description": "Milliseconds offset for post-action frame extraction (default: 500)",
					},
				},
				"required": []string{"video_path", "narration_path"},
			},
		},
		{
			Name:        "video_extract_frame",
			Description: "Extract a single frame from a video at a specific timestamp. Useful for debugging validation issues.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to video file",
					},
					"timestamp_ms": map[string]interface{}{
						"type":        "number",
						"description": "Timestamp in milliseconds to extract frame at",
					},
					"output_path": map[string]interface{}{
						"type":        "string",
						"description": "Optional: Path to save extracted frame (default: temp file)",
					},
				},
				"required": []string{"video_path", "timestamp_ms"},
			},
		},
		{
			Name:        "video_ocr_frame",
			Description: "Perform OCR on an image file and return extracted text. Useful for debugging validation issues.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"image_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to image file (PNG, JPG, etc.)",
					},
				},
				"required": []string{"image_path"},
			},
		},
		{
			Name:        "social_token_check",
			Description: "Validate that social media tokens are working. Returns status for each platform including Facebook token expiry.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name",
					},
					"platforms": map[string]interface{}{
						"type":        "string",
						"description": "Comma-separated platforms to check: x,facebook,instagram (default: all)",
					},
				},
				"required": []string{"project"},
			},
		},
		{
			Name:        "social_fb_refresh_token",
			Description: "Refresh Facebook long-lived token before it expires. Requires FACEBOOK_APP_ID and FACEBOOK_APP_SECRET in .env.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name",
					},
				},
				"required": []string{"project"},
			},
		},
		{
			Name:        "social_campaign_post",
			Description: "Post a campaign item to all configured platforms (X, Facebook, Instagram).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name",
					},
					"campaign": map[string]interface{}{
						"type":        "string",
						"description": "Campaign name",
					},
					"post_id": map[string]interface{}{
						"type":        "string",
						"description": "UUID of the post to publish",
					},
					"dry_run": map[string]interface{}{
						"type":        "boolean",
						"description": "Preview without posting (default: false)",
					},
				},
				"required": []string{"project", "campaign", "post_id"},
			},
		},
		{
			Name:        "social_prelaunch",
			Description: "Run full prelaunch verification: check all credentials, API connections, and token expiry.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name",
					},
					"campaign": map[string]interface{}{
						"type":        "string",
						"description": "Campaign name to validate (optional)",
					},
				},
				"required": []string{"project"},
			},
		},
		{
			Name:        "video_build",
			Description: "Build narrated video: parses narration, generates TTS audio, analyzes overlaps, mixes onto video with FFmpeg. Supports multiple TTS engines including macOS say, Google Gemini, Cloud TTS, and Vertex AI.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"narration_path": map[string]interface{}{
						"type":        "string",
						"description": "Full path to narration.md file",
					},
					"video_path": map[string]interface{}{
						"type":        "string",
						"description": "Full path to source video file",
					},
					"output_path": map[string]interface{}{
						"type":        "string",
						"description": "Full path for output .mp4 file",
					},
					"working_dir": map[string]interface{}{
						"type":        "string",
						"description": "Directory for intermediates (segments.json, audio/). If omitted, uses temp dir and cleans up.",
					},
					"tts_engine": map[string]interface{}{
						"type":        "string",
						"description": "TTS engine: 'auto' (default), 'say', 'gemini', 'cloudtts', or 'vertex'. Auto-detects: cloudtts if OAuth token exists, gemini if API key set, otherwise say.",
					},
					"tts_model": map[string]interface{}{
						"type":        "string",
						"description": "TTS model name (default varies by engine). Examples: gemini-2.5-flash-preview-tts, gemini-2.5-pro-preview-tts",
					},
					"voice": map[string]interface{}{
						"type":        "string",
						"description": "Voice name (default: Karen for say, Kore for gemini/cloudtts/vertex)",
					},
					"words_per_minute": map[string]interface{}{
						"type":        "number",
						"description": "Words per minute (say: -r flag, gemini: prompt pacing directive)",
					},
					"force": map[string]interface{}{
						"type":        "boolean",
						"description": "Force regeneration of audio files (default: false)",
					},
					"tts_timeout": map[string]interface{}{
						"type":        "string",
						"description": "Overall TTS generation timeout (e.g. '10m', '30m'). Default: 10m",
					},
					"verbose": map[string]interface{}{
						"type":        "boolean",
						"description": "Log raw HTTP responses on TTS errors (429/500/503) to stderr (default: false)",
					},
				},
				"required": []string{"narration_path", "video_path", "output_path"},
			},
		},
		// YouTube tools
		{
			Name:        "youtube_upload",
			Description: "Upload a video to YouTube. Credentials loaded from ~/.mstack/youtube/<channel_id>/",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file": map[string]interface{}{
						"type":        "string",
						"description": "Full path to video file (required)",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Video title (required)",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Video description",
					},
					"tags": map[string]interface{}{
						"type":        "string",
						"description": "Comma-separated tags",
					},
					"privacy": map[string]interface{}{
						"type":        "string",
						"description": "Privacy: public, private, unlisted (default: unlisted)",
					},
					"category_id": map[string]interface{}{
						"type":        "string",
						"description": "YouTube category ID (default: 28 - Science & Technology)",
					},
					"made_for_kids": map[string]interface{}{
						"type":        "boolean",
						"description": "COPPA compliance - is this video made for kids? (default: false)",
					},
					"playlist_id": map[string]interface{}{
						"type":        "string",
						"description": "Add to this playlist after upload",
					},
					"replace_id": map[string]interface{}{
						"type":        "string",
						"description": "Delete this YouTube video ID before uploading (for re-uploads)",
					},
					"channel": map[string]interface{}{
						"type":        "string",
						"description": "Channel ID (required if multiple channels configured)",
					},
					"dry_run": map[string]interface{}{
						"type":        "boolean",
						"description": "Preview without uploading (default: false)",
					},
				},
				"required": []string{"file", "title"},
			},
		},
		{
			Name:        "youtube_list",
			Description: "List videos from the authenticated YouTube channel (queries YouTube API directly).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"max_results": map[string]interface{}{
						"type":        "number",
						"description": "Maximum videos to return (default: 50, max: 50)",
					},
					"search": map[string]interface{}{
						"type":        "string",
						"description": "Search query to filter videos by title",
					},
					"channel": map[string]interface{}{
						"type":        "string",
						"description": "Channel ID (required if multiple channels configured)",
					},
				},
			},
		},
		{
			Name:        "youtube_delete",
			Description: "Delete a video from YouTube by its video ID.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"youtube_id": map[string]interface{}{
						"type":        "string",
						"description": "YouTube video ID to delete (required)",
					},
					"channel": map[string]interface{}{
						"type":        "string",
						"description": "Channel ID (required if multiple channels configured)",
					},
				},
				"required": []string{"youtube_id"},
			},
		},
		{
			Name:        "youtube_auth_check",
			Description: "Check if YouTube OAuth token is valid. Credentials in ~/.mstack/youtube/<channel_id>/",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"channel": map[string]interface{}{
						"type":        "string",
						"description": "Channel ID (required if multiple channels configured)",
					},
				},
			},
		},
		{
			Name:        "youtube_channels",
			Description: "List authenticated YouTube channels. Use this to find channel IDs for other YouTube tools.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		// Thumbnail extraction tool
		{
			Name:        "video_thumbnail_extract",
			Description: "Extract thumbnail from video at timestamp, resized to YouTube optimal 1280x720. Supports auto-detection of interesting frames using scene change analysis.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to video file (required)",
					},
					"timestamp_ms": map[string]interface{}{
						"type":        "number",
						"description": "Timestamp in milliseconds (-1 for auto-detect)",
					},
					"output_path": map[string]interface{}{
						"type":        "string",
						"description": "Output path (default: <video>_thumbnail.jpg)",
					},
					"width": map[string]interface{}{
						"type":        "number",
						"description": "Output width (default: 1280)",
					},
					"height": map[string]interface{}{
						"type":        "number",
						"description": "Output height (default: 720)",
					},
					"quality": map[string]interface{}{
						"type":        "number",
						"description": "JPEG quality 1-100 (default: 90)",
					},
				},
				"required": []string{"video_path"},
			},
		},
		// Video split/join/trim tools
		{
			Name:        "video_split",
			Description: "Split video at timestamps into multiple files. Output files named <video>_001.<ext>, <video>_002.<ext>, etc.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to video file to split (required)",
					},
					"timestamps": map[string]interface{}{
						"type":        "string",
						"description": "Comma-separated timestamps to split at. Formats: seconds (30), MM:SS (00:30), HH:MM:SS (00:00:30)",
					},
					"output_dir": map[string]interface{}{
						"type":        "string",
						"description": "Output directory (default: same as source video)",
					},
					"lossless": map[string]interface{}{
						"type":        "boolean",
						"description": "Use -c copy for fast lossless splitting (default: true)",
					},
				},
				"required": []string{"video_path", "timestamps"},
			},
		},
		{
			Name:        "video_join",
			Description: "Concatenate multiple videos into one file. Uses audio sync correction by default when re-encoding to fix choppy audio at segment boundaries.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_paths": map[string]interface{}{
						"type":        "string",
						"description": "Comma-separated paths to videos to join (in order, minimum 2)",
					},
					"output_path": map[string]interface{}{
						"type":        "string",
						"description": "Output file path (required)",
					},
					"lossless": map[string]interface{}{
						"type":        "boolean",
						"description": "Use -c copy for fast lossless join (default: true)",
					},
					"audio_sync": map[string]interface{}{
						"type":        "boolean",
						"description": "Use TS-based audio sync method when re-encoding to fix choppy audio (default: true when lossless=false)",
					},
				},
				"required": []string{"video_paths", "output_path"},
			},
		},
		{
			Name:        "video_trim",
			Description: "Trim video start and/or end. Supports manual trimming or auto-detection of dead air (silence/black frames).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to video file (required)",
					},
					"start_ms": map[string]interface{}{
						"type":        "number",
						"description": "Milliseconds to trim from start",
					},
					"end_ms": map[string]interface{}{
						"type":        "number",
						"description": "Milliseconds to trim from end",
					},
					"auto": map[string]interface{}{
						"type":        "boolean",
						"description": "Auto-detect dead air (silence/black frames)",
					},
					"preview": map[string]interface{}{
						"type":        "boolean",
						"description": "Preview trim points without actually trimming",
					},
					"output_path": map[string]interface{}{
						"type":        "string",
						"description": "Output file path (default: <video>_trimmed.<ext>)",
					},
				},
				"required": []string{"video_path"},
			},
		},
		{
			Name:        "video_audio_check",
			Description: "Analyze video for audio sync issues. Detects duration mismatch, silence gaps, and timestamp discontinuities.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"video_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to video file (required)",
					},
					"silence_threshold_db": map[string]interface{}{
						"type":        "number",
						"description": "Silence detection threshold in dB (default: -50)",
					},
					"silence_min_duration_ms": map[string]interface{}{
						"type":        "number",
						"description": "Minimum silence duration to report in ms (default: 100)",
					},
					"drift_threshold_ms": map[string]interface{}{
						"type":        "number",
						"description": "Audio/video duration drift threshold in ms (default: 100)",
					},
				},
				"required": []string{"video_path"},
			},
		},
		{
			Name:        "youtube_batch_upload",
			Description: "Batch upload videos from a JSON manifest. Quota-aware and resumable. Saves progress after each upload.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"manifest_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to JSON manifest file with upload items (required)",
					},
					"dry_run": map[string]interface{}{
						"type":        "boolean",
						"description": "Preview quota cost without uploading",
					},
					"resume": map[string]interface{}{
						"type":        "boolean",
						"description": "Retry failed uploads",
					},
					"channel": map[string]interface{}{
						"type":        "string",
						"description": "Channel ID (required if multiple channels configured)",
					},
				},
				"required": []string{"manifest_path"},
			},
		},
		// Duration estimation tool
		{
			Name:        "narration_estimate_duration",
			Description: "Estimate TTS duration before generation. Use validate mode to predict segments that will overlap. Saves iteration time by identifying timing issues before generating audio.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Single text to estimate (use instead of narration_path)",
					},
					"narration_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to narration.md file (use instead of text)",
					},
					"engine": map[string]interface{}{
						"type":        "string",
						"description": "TTS engine: say or gemini (default: say)",
					},
					"wpm": map[string]interface{}{
						"type":        "number",
						"description": "Words per minute (default: 200 for say, 150 for gemini)",
					},
					"validate": map[string]interface{}{
						"type":        "boolean",
						"description": "Check for timing conflicts and predict overlaps (only for narration_path)",
					},
				},
			},
		},
		// Google Auth tool
		{
			Name:        "google_auth_check",
			Description: "Check Google authentication status for all services (Forms, YouTube, Drive).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name (default: proseforge)",
					},
				},
			},
		},
		// Google Forms tools
		{
			Name:        "forms_list",
			Description: "List accessible Google Forms.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name (default: proseforge)",
					},
				},
			},
		},
		{
			Name:        "forms_get",
			Description: "Get form details by form ID.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name (default: proseforge)",
					},
					"form_id": map[string]interface{}{
						"type":        "string",
						"description": "Google Form ID (required)",
					},
				},
				"required": []string{"form_id"},
			},
		},
		{
			Name:        "forms_create",
			Description: "Create a new Google Form.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name (default: proseforge)",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Form title (required)",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Form description",
					},
					"items": map[string]interface{}{
						"type":        "string",
						"description": "JSON string of form items/questions array",
					},
				},
				"required": []string{"title"},
			},
		},
		{
			Name:        "forms_responses",
			Description: "Get form responses.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name (default: proseforge)",
					},
					"form_id": map[string]interface{}{
						"type":        "string",
						"description": "Google Form ID (required)",
					},
					"page_size": map[string]interface{}{
						"type":        "number",
						"description": "Number of responses per page (default: 50)",
					},
					"page_token": map[string]interface{}{
						"type":        "string",
						"description": "Page token for pagination",
					},
				},
				"required": []string{"form_id"},
			},
		},
		{
			Name:        "forms_export",
			Description: "Export form responses to JSON file.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{
						"type":        "string",
						"description": "Project name (default: proseforge)",
					},
					"form_id": map[string]interface{}{
						"type":        "string",
						"description": "Google Form ID (required)",
					},
					"output_path": map[string]interface{}{
						"type":        "string",
						"description": "Output file path (default: projects/<project>/forms/<form-name>/responses.json)",
					},
				},
				"required": []string{"form_id"},
			},
		},
	}

	sendResult(req.ID, ToolsListResult{Tools: tools})
}

func handleToolsCall(req MCPRequest) {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	project := "proseforge"
	if p, ok := params.Arguments["project"].(string); ok && p != "" {
		project = p
	}

	var result string
	var isError bool

	switch params.Name {
	case "video_parse":
		filePath, ok := params.Arguments["file_path"].(string)
		if !ok {
			sendToolError(req.ID, "file_path is required")
			return
		}
		segments, err := videoSvc.ParseNarrationMD(filePath)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(segments, "", "  ")
			result = string(jsonBytes)
		}

	case "video_analyze":
		narrationPath, ok := params.Arguments["narration_path"].(string)
		if !ok || narrationPath == "" {
			sendToolError(req.ID, "narration_path is required")
			return
		}

		audioDir, ok := params.Arguments["audio_dir"].(string)
		if !ok || audioDir == "" {
			sendToolError(req.ID, "audio_dir is required")
			return
		}

		// Parse narration.md internally
		segments, err := videoSvc.ParseNarrationMD(narrationPath)
		if err != nil {
			result = fmt.Sprintf("Error parsing narration: %v", err)
			isError = true
			break
		}

		// Analyze with parsed segments
		analysis, err := videoSvc.AnalyzeOverlapWithSegments(segments, audioDir)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(analysis, "", "  ")
			result = string(jsonBytes)
		}

	case "social_x_show":
		profile, err := socialSvc.ShowXProfile(project)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(profile, "", "  ")
			result = string(jsonBytes)
		}

	case "social_x_post":
		text, ok := params.Arguments["text"].(string)
		if !ok {
			sendToolError(req.ID, "text is required")
			return
		}
		tweetID, err := socialSvc.PostToX(project, text)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			result = fmt.Sprintf("Tweet posted successfully. ID: %s", tweetID)
		}

	case "social_fb_show":
		pageInfo, err := socialSvc.ShowFacebookPage(project)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(pageInfo, "", "  ")
			result = string(jsonBytes)
		}

	case "social_fb_post":
		text, ok := params.Arguments["text"].(string)
		if !ok {
			sendToolError(req.ID, "text is required")
			return
		}
		postID, err := socialSvc.PostToFacebook(project, text)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			result = fmt.Sprintf("Post created successfully. ID: %s", postID)
		}

	case "social_ig_show":
		profile, err := socialSvc.ShowInstagramProfile(project)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(profile, "", "  ")
			result = string(jsonBytes)
		}

	case "social_campaign_list":
		campaign := "launch"
		if c, ok := params.Arguments["campaign"].(string); ok && c != "" {
			campaign = c
		}
		err := socialSvc.ListPosts(project, campaign)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			result = "Campaign posts listed above"
		}

	case "secrets_diff":
		err := socialSvc.SyncSecrets(project, "diff")
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			result = "Secrets diff completed"
		}

	case "video_validate":
		videoPath, ok := params.Arguments["video_path"].(string)
		if !ok || videoPath == "" {
			sendToolError(req.ID, "video_path is required")
			return
		}

		narrationPath, ok := params.Arguments["narration_path"].(string)
		if !ok || narrationPath == "" {
			sendToolError(req.ID, "narration_path is required")
			return
		}

		// Extract optional parameters
		scriptPath, _ := params.Arguments["script_path"].(string)

		frameOffset := 500
		if fo, ok := params.Arguments["frame_offset"].(float64); ok {
			frameOffset = int(fo)
		}

		// Record frame metric if explicitly passed
		if _, ok := params.Arguments["frames"].(float64); ok {
			frames := int(params.Arguments["frames"].(float64))
			if err := validation.RecordFrameSample(frames); err != nil {
				// Log but don't fail
				fmt.Fprintf(os.Stderr, "Warning: could not record frame sample: %v\n", err)
			}
		}

		// Parse narration.md internally
		segments, err := videoSvc.ParseNarrationMD(narrationPath)
		if err != nil {
			result = fmt.Sprintf("Error parsing narration: %v", err)
			isError = true
			break
		}

		// Use directory containing narration.md for output (frames will go there)
		outputDir := filepath.Dir(narrationPath)

		// Parse segments with inline tags
		taggedSegments := validation.ParseSegmentsWithTags(segments)
		hasTaggedSegments := false
		for _, ts := range taggedSegments {
			if ts.HasTag() {
				hasTaggedSegments = true
				break
			}
		}

		var validationResult *validation.ValidationResult
		if hasTaggedSegments {
			// Use tagged segment validation
			validationResult, err = validationSvc.ValidateTaggedVideo(videoPath, taggedSegments, outputDir, nil, scriptPath, frameOffset)
		} else {
			// Use standard validation
			validationResult, err = validationSvc.ValidateVideoWithScript(videoPath, segments, outputDir, nil, scriptPath)
		}

		if err != nil {
			result = fmt.Sprintf("Validation error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(validationResult, "", "  ")
			result = string(jsonBytes)
		}

	case "video_extract_frame":
		videoPath, ok := params.Arguments["video_path"].(string)
		if !ok || videoPath == "" {
			sendToolError(req.ID, "video_path is required")
			return
		}

		timestampMs, ok := params.Arguments["timestamp_ms"].(float64)
		if !ok {
			sendToolError(req.ID, "timestamp_ms is required")
			return
		}

		outputPath, _ := params.Arguments["output_path"].(string)
		if outputPath == "" {
			outputPath = filepath.Join(os.TempDir(), fmt.Sprintf("frame_%d.png", int(timestampMs)))
		}

		err := validation.ExtractSingleFrame(videoPath, int(timestampMs), outputPath)
		if err != nil {
			result = fmt.Sprintf("Error extracting frame: %v", err)
			isError = true
		} else {
			result = fmt.Sprintf("Frame extracted to: %s", outputPath)
		}

	case "video_ocr_frame":
		imagePath, ok := params.Arguments["image_path"].(string)
		if !ok || imagePath == "" {
			sendToolError(req.ID, "image_path is required")
			return
		}

		text, err := validation.ExtractText(imagePath)
		if err != nil {
			result = fmt.Sprintf("OCR error: %v", err)
			isError = true
		} else {
			result = text
		}

	case "social_token_check":
		platforms := []string{}
		if p, ok := params.Arguments["platforms"].(string); ok && p != "" {
			// Split comma-separated platforms
			for _, plat := range splitPlatforms(p) {
				platforms = append(platforms, plat)
			}
		}

		status, err := socialSvc.CheckTokens(project, platforms)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(status, "", "  ")
			result = string(jsonBytes)
		}

	case "social_fb_refresh_token":
		refreshResult, err := socialSvc.RefreshFacebookToken(project)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(refreshResult, "", "  ")
			result = string(jsonBytes)
		}

	case "social_campaign_post":
		campaign, ok := params.Arguments["campaign"].(string)
		if !ok || campaign == "" {
			sendToolError(req.ID, "campaign is required")
			return
		}

		postID, ok := params.Arguments["post_id"].(string)
		if !ok || postID == "" {
			sendToolError(req.ID, "post_id is required")
			return
		}

		dryRun := false
		if d, ok := params.Arguments["dry_run"].(bool); ok {
			dryRun = d
		}

		postResult, err := socialSvc.PostCampaignItemWithResult(project, campaign, postID, dryRun)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(postResult, "", "  ")
			result = string(jsonBytes)
		}

	case "social_prelaunch":
		campaign, _ := params.Arguments["campaign"].(string)

		prelaunchResult, err := socialSvc.RunPrelaunchCheck(project, campaign)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(prelaunchResult, "", "  ")
			result = string(jsonBytes)
		}

	case "video_build":
		narrationPath, ok := params.Arguments["narration_path"].(string)
		if !ok || narrationPath == "" {
			sendToolError(req.ID, "narration_path is required")
			return
		}

		videoPath, ok := params.Arguments["video_path"].(string)
		if !ok || videoPath == "" {
			sendToolError(req.ID, "video_path is required")
			return
		}

		outputPath, ok := params.Arguments["output_path"].(string)
		if !ok || outputPath == "" {
			sendToolError(req.ID, "output_path is required")
			return
		}

		opts := video.PipelineOpts{
			NarrationPath: narrationPath,
			VideoPath:     videoPath,
			OutputPath:    outputPath,
		}

		if wd, ok := params.Arguments["working_dir"].(string); ok && wd != "" {
			opts.WorkingDir = wd
		}
		if e, ok := params.Arguments["tts_engine"].(string); ok && e != "" {
			opts.TTSEngine = e
		}
		if m, ok := params.Arguments["tts_model"].(string); ok && m != "" {
			opts.TTSModel = m
		}
		if v, ok := params.Arguments["voice"].(string); ok && v != "" {
			opts.Voice = v
		}
		if r, ok := params.Arguments["words_per_minute"].(float64); ok {
			opts.WordsPerMinute = int(r)
		}
		if f, ok := params.Arguments["force"].(bool); ok {
			opts.Force = f
		}
		if v, ok := params.Arguments["verbose"].(bool); ok {
			opts.Verbose = v
		}
		if t, ok := params.Arguments["tts_timeout"].(string); ok && t != "" {
			if d, err := time.ParseDuration(t); err == nil {
				opts.TTSTimeout = d
			} else {
				sendToolError(req.ID, fmt.Sprintf("invalid tts_timeout: %v", err))
				return
			}
		}

		pipelineResult, err := videoSvc.RunPipeline(opts)
		if err != nil {
			var rateLimitErr *video.TTSRateLimitError
			var authErr *video.TTSAuthError
			var ffmpegErr *video.FFmpegError

			switch {
			case errors.As(err, &rateLimitErr):
				result = fmt.Sprintf("TTS rate limited — all %d models exhausted (last: %s). Wait and retry later.", rateLimitErr.ModelsTriedCount, rateLimitErr.Model)
			case errors.As(err, &authErr):
				result = fmt.Sprintf("TTS auth failed: %s. Check GOOGLE_AI_API_KEY env var.", authErr.Detail)
			case errors.As(err, &ffmpegErr):
				msg := fmt.Sprintf("FFmpeg failed: %v", ffmpegErr.Cause)
				if opts.WorkingDir != "" {
					msg += fmt.Sprintf(". Working dir for debugging: %s", opts.WorkingDir)
				}
				result = msg
			default:
				result = fmt.Sprintf("Pipeline error: %v", err)
			}
			isError = true
			break
		}

		jsonBytes, _ := json.MarshalIndent(pipelineResult, "", "  ")
		result = string(jsonBytes)

	case "youtube_upload":
		// Resolve auth config (uses shared Google auth)
		project, _ := params.Arguments["project"].(string)
		channelFlag, _ := params.Arguments["channel"].(string)
		authConfig, err := youtube.ResolveAuthConfig(project, channelFlag)
		if err != nil {
			if _, ok := err.(*youtube.MultipleChannelsError); ok {
				result = "Multiple channels configured. Use youtube_channels to list them, then specify with 'channel' parameter."
			} else {
				result = fmt.Sprintf("Error resolving credentials: %v", err)
			}
			isError = true
			break
		}
		if authConfig == nil {
			result = "No YouTube channel configured. Run 'mstack youtube auth' first."
			isError = true
			break
		}

		// Get required parameters
		filePath, ok := params.Arguments["file"].(string)
		if !ok || filePath == "" {
			sendToolError(req.ID, "file is required")
			return
		}

		title, ok := params.Arguments["title"].(string)
		if !ok || title == "" {
			sendToolError(req.ID, "title is required")
			return
		}

		// Build upload params
		uploadParams := youtube.UploadParams{
			FilePath:    filePath,
			Title:       title,
			MadeForKids: youtube.DefaultMadeForKids,
		}

		if desc, ok := params.Arguments["description"].(string); ok {
			uploadParams.Description = desc
		}
		if privacy, ok := params.Arguments["privacy"].(string); ok && privacy != "" {
			uploadParams.Privacy = privacy
		}
		if catID, ok := params.Arguments["category_id"].(string); ok && catID != "" {
			uploadParams.CategoryID = catID
		}
		if mfk, ok := params.Arguments["made_for_kids"].(bool); ok {
			uploadParams.MadeForKids = mfk
		}
		if playlistID, ok := params.Arguments["playlist_id"].(string); ok {
			uploadParams.PlaylistID = playlistID
		}
		if replaceID, ok := params.Arguments["replace_id"].(string); ok {
			uploadParams.ReplaceID = replaceID
		}
		if tagsStr, ok := params.Arguments["tags"].(string); ok && tagsStr != "" {
			tags := strings.Split(tagsStr, ",")
			for i := range tags {
				tags[i] = strings.TrimSpace(tags[i])
			}
			uploadParams.Tags = tags
		}

		dryRun := false
		if d, ok := params.Arguments["dry_run"].(bool); ok {
			dryRun = d
		}

		ctx := context.Background()
		uploadResult, err := youtube.UploadVideo(ctx, *authConfig, uploadParams, dryRun)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(uploadResult, "", "  ")
			result = string(jsonBytes)
		}

	case "youtube_list":
		// Resolve auth config
		project, _ := params.Arguments["project"].(string)
		channelFlag, _ := params.Arguments["channel"].(string)
		authConfig, err := youtube.ResolveAuthConfig(project, channelFlag)
		if err != nil {
			if _, ok := err.(*youtube.MultipleChannelsError); ok {
				result = "Multiple channels configured. Use youtube_channels to list them, then specify with 'channel' parameter."
			} else {
				result = fmt.Sprintf("Error resolving credentials: %v", err)
			}
			isError = true
			break
		}
		if authConfig == nil {
			result = "No YouTube channel configured. Run 'mstack youtube auth' first."
			isError = true
			break
		}

		maxResults := int64(50)
		if mr, ok := params.Arguments["max_results"].(float64); ok {
			maxResults = int64(mr)
		}

		ctx := context.Background()

		// Check if search query provided
		var videos []youtube.YouTubeVideo
		if searchQuery, ok := params.Arguments["search"].(string); ok && searchQuery != "" {
			videos, err = youtube.SearchChannelVideos(ctx, *authConfig, searchQuery, maxResults)
		} else {
			videos, err = youtube.ListChannelVideos(ctx, *authConfig, maxResults)
		}

		if err != nil {
			result = fmt.Sprintf("Error listing videos: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(videos, "", "  ")
			result = string(jsonBytes)
		}

	case "youtube_delete":
		// Resolve auth config
		project, _ := params.Arguments["project"].(string)
		channelFlag, _ := params.Arguments["channel"].(string)
		authConfig, err := youtube.ResolveAuthConfig(project, channelFlag)
		if err != nil {
			if _, ok := err.(*youtube.MultipleChannelsError); ok {
				result = "Multiple channels configured. Use youtube_channels to list them, then specify with 'channel' parameter."
			} else {
				result = fmt.Sprintf("Error resolving credentials: %v", err)
			}
			isError = true
			break
		}
		if authConfig == nil {
			result = "No YouTube channel configured. Run 'mstack youtube auth' first."
			isError = true
			break
		}

		youtubeID, ok := params.Arguments["youtube_id"].(string)
		if !ok || youtubeID == "" {
			sendToolError(req.ID, "youtube_id is required")
			return
		}

		ctx := context.Background()
		err = youtube.DeleteVideo(ctx, *authConfig, youtubeID)
		if err != nil {
			result = fmt.Sprintf("Error deleting video: %v", err)
			isError = true
		} else {
			result = fmt.Sprintf("Video %s deleted successfully", youtubeID)
		}

	case "youtube_auth_check":
		// Resolve auth config
		project, _ := params.Arguments["project"].(string)
		channelFlag, _ := params.Arguments["channel"].(string)
		authConfig, err := youtube.ResolveAuthConfig(project, channelFlag)
		if err != nil {
			if multiErr, ok := err.(*youtube.MultipleChannelsError); ok {
				result = fmt.Sprintf("Multiple channels configured: %v. Use youtube_channels to list them, then specify with 'channel' parameter.", multiErr.Channels)
			} else {
				result = fmt.Sprintf("Error resolving credentials: %v", err)
			}
			isError = true
			break
		}
		if authConfig == nil {
			result = "No Google auth configured. Run 'mstack google auth' first."
			isError = true
			break
		}

		valid, status := youtube.CheckToken(*authConfig)

		// Try to load or fetch channel info for display
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
			}
		}

		checkResult := map[string]interface{}{
			"valid":   valid,
			"status":  status,
			"project": authConfig.Project,
			"paths": map[string]string{
				"client_secret": authConfig.ClientSecretPath,
				"token_file":    authConfig.TokenFilePath,
			},
		}
		if channelInfo != nil {
			checkResult["channel_id"] = channelInfo.ID
			checkResult["channel_title"] = channelInfo.Title
			checkResult["channel_url"] = channelInfo.CustomURL
		}

		jsonBytes, _ := json.MarshalIndent(checkResult, "", "  ")
		result = string(jsonBytes)

	case "youtube_channels":
		// List all authenticated channels for a project
		project, _ := params.Arguments["project"].(string)
		if project == "" {
			project = "proseforge"
		}
		channelIDs, err := youtube.ListChannels(project)
		if err != nil {
			result = fmt.Sprintf("Error listing channels: %v", err)
			isError = true
			break
		}

		if len(channelIDs) == 0 {
			result = "No YouTube channels configured. Run 'mstack google auth' first, then 'mstack video youtube auth' to cache channel info."
			isError = true
			break
		}

		var channels []map[string]interface{}
		for _, channelID := range channelIDs {
			authConfig := youtube.AuthConfigForProjectAndChannel(project, channelID)
			channelData := map[string]interface{}{
				"channel_id": channelID,
			}

			// Load channel info if available
			if info, err := youtube.LoadChannelInfo(authConfig); err == nil {
				channelData["title"] = info.Title
				channelData["custom_url"] = info.CustomURL
			}

			// Check token status
			valid, status := youtube.CheckToken(authConfig)
			channelData["token_valid"] = valid
			channelData["token_status"] = status

			channels = append(channels, channelData)
		}

		jsonBytes, _ := json.MarshalIndent(channels, "", "  ")
		result = string(jsonBytes)

	case "video_thumbnail_extract":
		videoPath, ok := params.Arguments["video_path"].(string)
		if !ok || videoPath == "" {
			sendToolError(req.ID, "video_path is required")
			return
		}

		thumbnailParams := video.ThumbnailParams{
			VideoPath:   videoPath,
			TimestampMs: -1, // Default: auto-detect
		}

		if ts, ok := params.Arguments["timestamp_ms"].(float64); ok {
			thumbnailParams.TimestampMs = int(ts)
		}
		if op, ok := params.Arguments["output_path"].(string); ok {
			thumbnailParams.OutputPath = op
		}
		if w, ok := params.Arguments["width"].(float64); ok {
			thumbnailParams.Width = int(w)
		}
		if h, ok := params.Arguments["height"].(float64); ok {
			thumbnailParams.Height = int(h)
		}
		if q, ok := params.Arguments["quality"].(float64); ok {
			thumbnailParams.Quality = int(q)
		}

		thumbnailResult, err := video.ExtractThumbnail(thumbnailParams)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(thumbnailResult, "", "  ")
			result = string(jsonBytes)
		}

	case "video_split":
		videoPath, ok := params.Arguments["video_path"].(string)
		if !ok || videoPath == "" {
			sendToolError(req.ID, "video_path is required")
			return
		}

		timestamps, ok := params.Arguments["timestamps"].(string)
		if !ok || timestamps == "" {
			sendToolError(req.ID, "timestamps is required")
			return
		}

		parsedTimestamps, err := video.ParseTimestamps(timestamps)
		if err != nil {
			result = fmt.Sprintf("Error parsing timestamps: %v", err)
			isError = true
			break
		}

		splitParams := video.SplitParams{
			VideoPath:  videoPath,
			Timestamps: parsedTimestamps,
			Lossless:   true,
		}

		if od, ok := params.Arguments["output_dir"].(string); ok {
			splitParams.OutputDir = od
		}
		if lossless, ok := params.Arguments["lossless"].(bool); ok {
			splitParams.Lossless = lossless
		}

		splitResult, err := video.SplitVideo(splitParams)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(splitResult, "", "  ")
			result = string(jsonBytes)
		}

	case "video_join":
		videoPaths, ok := params.Arguments["video_paths"].(string)
		if !ok || videoPaths == "" {
			sendToolError(req.ID, "video_paths is required")
			return
		}

		outputPath, ok := params.Arguments["output_path"].(string)
		if !ok || outputPath == "" {
			sendToolError(req.ID, "output_path is required")
			return
		}

		parsedPaths, err := video.ParseVideoPaths(videoPaths)
		if err != nil {
			result = fmt.Sprintf("Error parsing video paths: %v", err)
			isError = true
			break
		}

		joinParams := video.JoinParams{
			VideoPaths: parsedPaths,
			OutputPath: outputPath,
			Lossless:   true,
			AudioSync:  true, // Default to audio sync when re-encoding
		}

		if lossless, ok := params.Arguments["lossless"].(bool); ok {
			joinParams.Lossless = lossless
		}
		if audioSync, ok := params.Arguments["audio_sync"].(bool); ok {
			joinParams.AudioSync = audioSync
		}

		joinResult, err := video.JoinVideos(joinParams)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(joinResult, "", "  ")
			result = string(jsonBytes)
		}

	case "video_trim":
		videoPath, ok := params.Arguments["video_path"].(string)
		if !ok || videoPath == "" {
			sendToolError(req.ID, "video_path is required")
			return
		}

		trimParams := video.TrimParams{
			VideoPath: videoPath,
			Lossless:  true,
		}

		if startMs, ok := params.Arguments["start_ms"].(float64); ok {
			trimParams.StartMs = int(startMs)
		}
		if endMs, ok := params.Arguments["end_ms"].(float64); ok {
			trimParams.EndMs = int(endMs)
		}
		if auto, ok := params.Arguments["auto"].(bool); ok {
			trimParams.Auto = auto
		}
		if preview, ok := params.Arguments["preview"].(bool); ok {
			trimParams.PreviewOnly = preview
		}
		if op, ok := params.Arguments["output_path"].(string); ok {
			trimParams.OutputPath = op
		}

		// Validate at least one trim option is set
		if !trimParams.Auto && trimParams.StartMs == 0 && trimParams.EndMs == 0 {
			sendToolError(req.ID, "Either 'auto' or 'start_ms'/'end_ms' is required")
			return
		}

		trimResult, err := video.TrimVideo(trimParams)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(trimResult, "", "  ")
			result = string(jsonBytes)
		}

	case "video_audio_check":
		videoPath, ok := params.Arguments["video_path"].(string)
		if !ok || videoPath == "" {
			sendToolError(req.ID, "video_path is required")
			return
		}

		checkParams := video.AudioCheckParams{
			VideoPath: videoPath,
		}

		if threshold, ok := params.Arguments["silence_threshold_db"].(float64); ok {
			checkParams.SilenceThresholdDB = int(threshold)
		}
		if minDuration, ok := params.Arguments["silence_min_duration_ms"].(float64); ok {
			checkParams.SilenceMinDurationMs = int(minDuration)
		}
		if drift, ok := params.Arguments["drift_threshold_ms"].(float64); ok {
			checkParams.DriftThresholdMs = int(drift)
		}

		checkResult, err := video.CheckAudioSync(checkParams)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(checkResult, "", "  ")
			result = string(jsonBytes)
		}

	case "youtube_batch_upload":
		manifestPath, ok := params.Arguments["manifest_path"].(string)
		if !ok || manifestPath == "" {
			sendToolError(req.ID, "manifest_path is required")
			return
		}

		// Resolve auth config
		project, _ := params.Arguments["project"].(string)
		channelFlag, _ := params.Arguments["channel"].(string)
		authConfig, err := youtube.ResolveAuthConfig(project, channelFlag)
		if err != nil {
			if _, ok := err.(*youtube.MultipleChannelsError); ok {
				result = "Multiple channels configured. Use youtube_channels to list them, then specify with 'channel' parameter."
			} else {
				result = fmt.Sprintf("Error resolving credentials: %v", err)
			}
			isError = true
			break
		}
		if authConfig == nil {
			result = "No Google auth configured. Run 'mstack google auth' first."
			isError = true
			break
		}

		batchParams := youtube.BatchParams{
			ManifestPath: manifestPath,
		}

		if dryRun, ok := params.Arguments["dry_run"].(bool); ok {
			batchParams.DryRun = dryRun
		}
		if resume, ok := params.Arguments["resume"].(bool); ok {
			batchParams.Resume = resume
		}

		ctx := context.Background()
		batchResult, err := youtube.BatchUpload(ctx, *authConfig, batchParams)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(batchResult, "", "  ")
			result = string(jsonBytes)
		}

	case "narration_estimate_duration":
		// Check if single text or full narration
		if text, ok := params.Arguments["text"].(string); ok && text != "" {
			engine := "say"
			if e, ok := params.Arguments["engine"].(string); ok && e != "" {
				engine = e
			}
			wpm := 0
			if w, ok := params.Arguments["wpm"].(float64); ok {
				wpm = int(w)
			}

			estimateResult := video.EstimateSingleText(text, engine, wpm)
			jsonBytes, _ := json.MarshalIndent(estimateResult, "", "  ")
			result = string(jsonBytes)
		} else if narrationPath, ok := params.Arguments["narration_path"].(string); ok && narrationPath != "" {
			segments, err := videoSvc.ParseNarrationMD(narrationPath)
			if err != nil {
				result = fmt.Sprintf("Error parsing narration: %v", err)
				isError = true
				break
			}

			engine := "say"
			if e, ok := params.Arguments["engine"].(string); ok && e != "" {
				engine = e
			}
			wpm := 0
			if w, ok := params.Arguments["wpm"].(float64); ok {
				wpm = int(w)
			}
			validate := false
			if v, ok := params.Arguments["validate"].(bool); ok {
				validate = v
			}

			estimateParams := video.EstimationParams{
				Engine:         engine,
				WordsPerMinute: wpm,
				Validate:       validate,
			}

			estimateResult := video.EstimateSegments(segments, estimateParams)
			jsonBytes, _ := json.MarshalIndent(estimateResult, "", "  ")
			result = string(jsonBytes)
		} else {
			sendToolError(req.ID, "Either 'text' or 'narration_path' is required")
			return
		}

	case "google_auth_check":
		authConfig := google.AuthConfigForProject(project)
		status := google.CheckToken(authConfig)
		jsonBytes, _ := json.MarshalIndent(status, "", "  ")
		result = string(jsonBytes)

	case "forms_list":
		ctx := context.Background()
		formsList, err := formsSvc.ListForms(ctx, project)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(formsList, "", "  ")
			result = string(jsonBytes)
		}

	case "forms_get":
		formID, ok := params.Arguments["form_id"].(string)
		if !ok || formID == "" {
			sendToolError(req.ID, "form_id is required")
			return
		}
		ctx := context.Background()
		form, err := formsSvc.GetForm(ctx, project, formID)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(form, "", "  ")
			result = string(jsonBytes)
		}

	case "forms_create":
		title, ok := params.Arguments["title"].(string)
		if !ok || title == "" {
			sendToolError(req.ID, "title is required")
			return
		}

		createParams := forms.CreateFormParams{
			Title: title,
		}

		if desc, ok := params.Arguments["description"].(string); ok {
			createParams.Description = desc
		}

		// Parse items JSON if provided
		if itemsJSON, ok := params.Arguments["items"].(string); ok && itemsJSON != "" {
			var items []forms.CreateFormItem
			if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
				sendToolError(req.ID, fmt.Sprintf("Invalid items JSON: %v", err))
				return
			}
			createParams.Items = items
		}

		ctx := context.Background()
		form, err := formsSvc.CreateForm(ctx, project, createParams)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(form, "", "  ")
			result = string(jsonBytes)
		}

	case "forms_responses":
		formID, ok := params.Arguments["form_id"].(string)
		if !ok || formID == "" {
			sendToolError(req.ID, "form_id is required")
			return
		}

		respParams := forms.ResponseParams{}
		if pageSize, ok := params.Arguments["page_size"].(float64); ok {
			respParams.PageSize = int(pageSize)
		}
		if pageToken, ok := params.Arguments["page_token"].(string); ok {
			respParams.PageToken = pageToken
		}

		ctx := context.Background()
		responses, err := formsSvc.GetResponses(ctx, project, formID, respParams)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(responses, "", "  ")
			result = string(jsonBytes)
		}

	case "forms_export":
		formID, ok := params.Arguments["form_id"].(string)
		if !ok || formID == "" {
			sendToolError(req.ID, "form_id is required")
			return
		}

		outputPath := ""
		if op, ok := params.Arguments["output_path"].(string); ok {
			outputPath = op
		}

		ctx := context.Background()
		exportResult, err := formsSvc.ExportResponses(ctx, project, formID, outputPath)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			jsonBytes, _ := json.MarshalIndent(exportResult, "", "  ")
			result = string(jsonBytes)
		}

	default:
		sendToolError(req.ID, fmt.Sprintf("Unknown tool: %s", params.Name))
		return
	}

	sendResult(req.ID, CallToolResult{
		Content: []TextContent{{Type: "text", Text: result}},
		IsError: isError,
	})
}

func sendResult(id interface{}, result interface{}) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func sendError(id interface{}, code int, message string, data interface{}) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	data2, _ := json.Marshal(resp)
	fmt.Println(string(data2))
}

func sendToolError(id interface{}, message string) {
	sendResult(id, CallToolResult{
		Content: []TextContent{{Type: "text", Text: message}},
		IsError: true,
	})
}

// splitPlatforms splits a comma-separated list of platforms
func splitPlatforms(s string) []string {
	var result []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
