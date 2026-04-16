package validation

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
)

// tagPattern matches backtick-wrapped JSON: `{"action":"click",...}`
var tagPattern = regexp.MustCompile("`({[^`]+})`")

// ParseNarrationTag extracts a NarrationTag from narration text
// Returns the tag (or nil if none) and the clean text without the tag
func ParseNarrationTag(text string) (*NarrationTag, string) {
	matches := tagPattern.FindStringSubmatch(text)
	if len(matches) < 2 {
		return nil, text
	}

	jsonStr := matches[1]
	var tag NarrationTag
	if err := json.Unmarshal([]byte(jsonStr), &tag); err != nil {
		// Invalid JSON, return original text
		return nil, text
	}

	// Validate action type
	if !isValidAction(tag.Action) {
		return nil, text
	}

	// Set default timing if not specified
	if tag.Timing == "" {
		tag.Timing = "after"
	}

	// Remove the tag from the text
	cleanText := strings.TrimSpace(tagPattern.ReplaceAllString(text, ""))

	return &tag, cleanText
}

// isValidAction checks if the action type is supported
func isValidAction(action string) bool {
	for _, valid := range ValidActionTypes {
		if action == valid {
			return true
		}
	}
	return false
}

// ParseSegmentsWithTags converts segments to TaggedSegments, extracting any tags
func ParseSegmentsWithTags(segments []types.Segment) []TaggedSegment {
	tagged := make([]TaggedSegment, len(segments))

	for i, seg := range segments {
		tag, cleanText := ParseNarrationTag(seg.Text)

		tagged[i] = TaggedSegment{
			Index:       seg.Index,
			Timestamp:   seg.Timestamp,
			TimestampMs: seg.TimestampMs,
			Text:        cleanText,
			RawText:     seg.Text,
			Tag:         tag,
		}
	}

	return tagged
}

// EnrichWithDurations adds audio duration info to tagged segments
// This requires the audio files to exist and be analyzed
func EnrichWithDurations(segments []TaggedSegment, analysisResults []types.SegmentInfo) []TaggedSegment {
	// Create a map for quick lookup
	durationMap := make(map[int]types.SegmentInfo)
	for _, info := range analysisResults {
		durationMap[info.Segment] = info
	}

	for i := range segments {
		if info, ok := durationMap[segments[i].Index]; ok {
			segments[i].DurationMs = info.DurationMs
			segments[i].EndMs = info.EndMs
		}
	}

	return segments
}

// HasTag returns true if the segment has a parsed narration tag
func (ts *TaggedSegment) HasTag() bool {
	return ts.Tag != nil
}

// GetFrameOffsets returns the millisecond offsets for frame extraction
// based on the tag's timing setting
func (ts *TaggedSegment) GetFrameOffsets(frameOffset int) []int {
	if ts.Tag == nil {
		// No tag, just return single frame at start
		return []int{0}
	}

	switch ts.Tag.Timing {
	case "before":
		// Action happens before narration
		return []int{
			-1000,                    // Pre-action state (1s before narration)
			0,                        // Post-action (action already happened)
			ts.DurationMs,            // End of narration
		}
	case "during":
		// Action happens during narration
		return []int{
			0,                        // Start of narration
			ts.DurationMs / 2,        // Middle of narration
			ts.DurationMs,            // End of narration
		}
	case "after":
		fallthrough
	default:
		// Action happens after narration (default)
		return []int{
			0,                                 // Pre-action (target should be visible)
			ts.DurationMs + frameOffset,       // Post-action (quick changes)
			ts.DurationMs + frameOffset*2,     // Settled state (animations done)
		}
	}
}

// GetFramePositions returns position labels for each frame offset
func (ts *TaggedSegment) GetFramePositions() []string {
	if ts.Tag == nil {
		return []string{"single"}
	}

	switch ts.Tag.Timing {
	case "before":
		return []string{"pre_action", "post_action", "narration_end"}
	case "during":
		return []string{"narration_start", "narration_mid", "narration_end"}
	case "after":
		fallthrough
	default:
		return []string{"pre_action", "post_action", "settled"}
	}
}
