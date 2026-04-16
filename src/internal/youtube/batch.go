package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// BatchItem represents a single video in a batch upload manifest
type BatchItem struct {
	// Input fields (user-provided)
	File        string   `json:"file"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Privacy     string   `json:"privacy,omitempty"`
	PlaylistID  string   `json:"playlist_id,omitempty"`
	CategoryID  string   `json:"category_id,omitempty"`
	MadeForKids bool     `json:"made_for_kids,omitempty"`
	ReplaceID   string   `json:"replace_id,omitempty"`

	// Status fields (set by batch processor)
	Status     string `json:"status"` // pending, uploading, completed, failed, skipped
	YouTubeID  string `json:"youtube_id,omitempty"`
	YouTubeURL string `json:"youtube_url,omitempty"`
	Error      string `json:"error,omitempty"`
	UploadedAt string `json:"uploaded_at,omitempty"`
}

// BatchManifest represents a batch upload manifest file
type BatchManifest struct {
	Items []BatchItem `json:"items"`
}

// BatchResult contains the result of a batch upload operation
type BatchResult struct {
	ManifestPath string      `json:"manifest_path"`
	TotalItems   int         `json:"total_items"`
	Pending      int         `json:"pending"`
	Completed    int         `json:"completed"`
	Failed       int         `json:"failed"`
	Skipped      int         `json:"skipped"`
	DryRun       bool        `json:"dry_run"`
	Items        []BatchItem `json:"items"`
	Warning      string      `json:"warning,omitempty"`
}

// BatchParams contains options for batch upload
type BatchParams struct {
	ManifestPath string // Path to manifest JSON file
	DryRun       bool   // Preview without uploading
	Resume       bool   // Resume from failed/pending items
}

// LoadBatchManifest loads a batch manifest from a JSON file
func LoadBatchManifest(path string) (*BatchManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest BatchManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		// Try parsing as array of items directly
		var items []BatchItem
		if err2 := json.Unmarshal(data, &items); err2 != nil {
			return nil, fmt.Errorf("failed to parse manifest: %w", err)
		}
		manifest.Items = items
	}

	// Initialize status for new items
	for i := range manifest.Items {
		if manifest.Items[i].Status == "" {
			manifest.Items[i].Status = "pending"
		}
	}

	return &manifest, nil
}

// SaveBatchManifest saves a batch manifest to a JSON file
func SaveBatchManifest(path string, manifest *BatchManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// BatchUpload executes a batch upload operation
func BatchUpload(ctx context.Context, authConfig AuthConfig, params BatchParams) (*BatchResult, error) {
	// Load manifest
	manifest, err := LoadBatchManifest(params.ManifestPath)
	if err != nil {
		return nil, err
	}

	// Count items by status
	result := &BatchResult{
		ManifestPath: params.ManifestPath,
		TotalItems:   len(manifest.Items),
		DryRun:       params.DryRun,
		Items:        manifest.Items,
	}

	// Count pending items
	pendingItems := 0
	for _, item := range manifest.Items {
		if item.Status == "pending" || (params.Resume && item.Status == "failed") {
			pendingItems++
		}
		switch item.Status {
		case "completed":
			result.Completed++
		case "failed":
			if params.Resume {
				// Will retry
			} else {
				result.Failed++
			}
		case "skipped":
			result.Skipped++
		}
	}

	// If dry run, return preview
	if params.DryRun {
		result.Pending = pendingItems
		return result, nil
	}

	// Process each pending item
	for i := range manifest.Items {
		item := &manifest.Items[i]

		// Skip non-pending items (unless resume mode)
		if item.Status == "completed" || item.Status == "skipped" {
			continue
		}
		if item.Status == "failed" && !params.Resume {
			continue
		}

		// Mark as uploading
		item.Status = "uploading"
		SaveBatchManifest(params.ManifestPath, manifest)

		// Build upload params
		uploadParams := UploadParams{
			FilePath:    item.File,
			Title:       item.Title,
			Description: item.Description,
			Tags:        item.Tags,
			Privacy:     item.Privacy,
			CategoryID:  item.CategoryID,
			MadeForKids: item.MadeForKids,
			PlaylistID:  item.PlaylistID,
			ReplaceID:   item.ReplaceID,
		}

		// Execute upload
		uploadResult, err := UploadVideo(ctx, authConfig, uploadParams, false)
		if err != nil {
			item.Status = "failed"
			item.Error = err.Error()
			result.Failed++
		} else if !uploadResult.Success {
			item.Status = "failed"
			item.Error = uploadResult.Error
			result.Failed++
		} else {
			item.Status = "completed"
			item.YouTubeID = uploadResult.VideoID
			item.YouTubeURL = uploadResult.VideoURL
			item.UploadedAt = time.Now().Format(time.RFC3339)
			item.Error = ""
			result.Completed++
		}

		// Save progress after each upload
		SaveBatchManifest(params.ManifestPath, manifest)
	}

	// Count final status
	result.Pending = 0
	for _, item := range manifest.Items {
		if item.Status == "pending" {
			result.Pending++
		}
	}
	result.Items = manifest.Items

	return result, nil
}

// CreateBatchManifest creates a new manifest from video files
func CreateBatchManifest(files []string, outputPath string) error {
	var items []BatchItem
	for _, file := range files {
		items = append(items, BatchItem{
			File:    file,
			Title:   "", // User needs to fill in
			Status:  "pending",
			Privacy: DefaultPrivacy,
		})
	}

	manifest := &BatchManifest{Items: items}
	return SaveBatchManifest(outputPath, manifest)
}
