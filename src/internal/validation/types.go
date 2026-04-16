package validation

// ValidationResult represents the complete validation report
type ValidationResult struct {
	VideoName   string            `json:"video_name"`
	Timestamp   string            `json:"validation_timestamp"`
	FramesDir   string            `json:"frames_dir"`
	ScriptPath  string            `json:"script_path,omitempty"`
	Issues      []ValidationIssue `json:"issues"`
	Summary     ValidationSummary `json:"summary"`
}

// ValidationIssue represents a single validation problem
type ValidationIssue struct {
	Type           string      `json:"type"`
	Severity       string      `json:"severity"` // high, medium, low
	SegmentIndex   int         `json:"segment_index"`
	Timestamp      string      `json:"timestamp"`
	NarrationText  string      `json:"narration_text"`
	ScreenContent  string      `json:"screen_content,omitempty"`
	ExpectedText   []string    `json:"expected_text,omitempty"`
	FoundText      []string    `json:"found_text,omitempty"`
	TestFile       string      `json:"test_file,omitempty"`
	TestLine       int         `json:"test_line,omitempty"`
	CodeContext    string      `json:"code_context,omitempty"`
	Suggestion     *Suggestion `json:"suggestion,omitempty"`
}

// Suggestion provides actionable fix recommendations
type Suggestion struct {
	Action      string `json:"action"`
	Description string `json:"description"`
	CodeFix     string `json:"code_fix,omitempty"`
}

// ValidationSummary provides aggregate counts
type ValidationSummary struct {
	TotalSegments  int `json:"total_segments"`
	SegmentsChecked int `json:"segments_checked"`
	TotalIssues    int `json:"total_issues"`
	HighSeverity   int `json:"high_severity"`
	MediumSeverity int `json:"medium_severity"`
	LowSeverity    int `json:"low_severity"`
}

// FrameInfo represents extracted frame metadata
type FrameInfo struct {
	SegmentIndex int    `json:"segment_index"`
	Timestamp    string `json:"timestamp"`
	TimestampMs  int    `json:"timestamp_ms"`
	FramePath    string `json:"frame_path"`
	OCRText      string `json:"ocr_text,omitempty"`
}

// ValidationConfig defines validation rules
type ValidationConfig struct {
	Keywords   map[string]KeywordRule `json:"keywords"`
	Validation ValidationSettings     `json:"validation"`
}

// KeywordRule defines expected elements for a keyword
type KeywordRule struct {
	ExpectedElements  []string `json:"expected_elements,omitempty"`
	ExpectedText      []string `json:"expected_text,omitempty"`
	EmptyStateText    []string `json:"empty_state_text,omitempty"`
	NarrationPatterns []string `json:"narration_patterns,omitempty"`
}

// ValidationSettings defines global validation behavior
type ValidationSettings struct {
	StrictMode         bool `json:"strict_mode"`
	WarnOnEmptyState   bool `json:"warn_on_empty_state"`
	RequireUIElements  bool `json:"require_ui_elements"`
	MinPauseDurationMs int  `json:"min_pause_duration_ms"`
}

// NarrationTag represents semantic metadata embedded in narration text
// Format in narration.md: `{"action":"click","target":"Settings"}`
type NarrationTag struct {
	Action  string   `json:"action"`            // click, fill, navigate, wait, select, hover, scroll, assert
	Target  string   `json:"target,omitempty"`  // element text or identifier
	Timing  string   `json:"timing,omitempty"`  // before, after, during (default: after)
	Visible []string `json:"visible,omitempty"` // elements that should be visible
}

// ValidActionTypes defines the supported action types for validation
var ValidActionTypes = []string{
	"click",    // Click an element - target visible at start, state changed after
	"fill",     // Fill an input - input visible, value appears after
	"navigate", // Navigate to page - URL/content changes after
	"wait",     // Wait for state - no state change expected
	"select",   // Select from dropdown - dropdown visible, selected value appears
	"hover",    // Hover over element - target visible, hover state appears
	"scroll",   // Scroll to element - target becomes visible after
	"assert",   // Assert visibility - all visible items found in OCR
}

// TaggedSegment combines a segment with its parsed narration tag and timing info
type TaggedSegment struct {
	Index          int          `json:"index"`
	Timestamp      string       `json:"timestamp"`
	TimestampMs    int          `json:"timestamp_ms"`
	Text           string       `json:"text"`            // Original narration text (without tag)
	RawText        string       `json:"raw_text"`        // Full text including tag
	Tag            *NarrationTag `json:"tag,omitempty"`  // Parsed tag, nil if no tag present
	DurationMs     int          `json:"duration_ms"`     // Audio duration
	EndMs          int          `json:"end_ms"`          // TimestampMs + DurationMs
	Frames         []FrameInfo  `json:"frames"`          // Multiple frames for this segment
}

// TaggedFrameInfo extends FrameInfo with position context
type TaggedFrameInfo struct {
	FrameInfo
	Position string `json:"position"` // "pre_action", "post_action", "settled"
	OffsetMs int    `json:"offset_ms"` // Offset from narration start
}
