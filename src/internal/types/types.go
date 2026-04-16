package types

// Segment represents a narration segment with timing information
type Segment struct {
	Index       int    `json:"index"`
	Timestamp   string `json:"timestamp"`
	TimestampMs int    `json:"timestamp_ms"`
	Text        string `json:"text"`
	AudioFile   string `json:"audio_file"`
}

// SegmentInfo contains analysis information for a segment
type SegmentInfo struct {
	Segment     int    `json:"segment"`
	Text        string `json:"text"`
	StartMs     int    `json:"start_ms"`
	DurationMs  int    `json:"duration_ms"`
	EndMs       int    `json:"end_ms"`
	NextStartMs int    `json:"next_start_ms"`
	GapMs       int    `json:"gap_ms"`
}

// Summary contains summary statistics
type Summary struct {
	Overlaps  int `json:"overlaps"`
	TightFits int `json:"tight_fits"`
	GoodFits  int `json:"good_fits"`
}

// AnalysisResults contains the full analysis
type AnalysisResults struct {
	TotalSegments int           `json:"total_segments"`
	Overlaps      []SegmentInfo `json:"overlaps"`
	TightFits     []SegmentInfo `json:"tight_fits"`
	GoodFits      []SegmentInfo `json:"good_fits"`
	Summary       Summary       `json:"summary"`
}
