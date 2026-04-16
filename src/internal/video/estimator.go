package video

import (
	"strings"
	"unicode"

	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
)

const (
	DefaultSayWPM    = 200
	DefaultGeminiWPM = 96 // measured conservative: ~1.6 WPS at 24kHz mono
)

// DurationEstimate represents estimated duration for a segment
type DurationEstimate struct {
	SegmentIndex int    `json:"segment_index"`
	Text         string `json:"text"`
	WordCount    int    `json:"word_count"`
	EstimatedMs  int    `json:"estimated_ms"`
	StartMs      int    `json:"start_ms"`
	EndMs        int    `json:"end_ms"`
	NextStartMs  int    `json:"next_start_ms,omitempty"`
	GapMs        int    `json:"gap_ms,omitempty"`
	WillOverlap  bool   `json:"will_overlap,omitempty"`
	OverlapMs    int    `json:"overlap_ms,omitempty"`
}

// EstimationResult contains all segment estimates and validation summary
type EstimationResult struct {
	Engine         string             `json:"engine"`
	WordsPerMinute int                `json:"words_per_minute"`
	TotalSegments  int                `json:"total_segments"`
	Segments       []DurationEstimate `json:"segments"`
	Summary        EstimationSummary  `json:"summary"`
}

// EstimationSummary provides aggregate info
type EstimationSummary struct {
	TotalDurationMs    int `json:"total_duration_ms"`
	TotalWordCount     int `json:"total_word_count"`
	PredictedOverlaps  int `json:"predicted_overlaps"`
	PredictedTightFits int `json:"predicted_tight_fits"`
	TotalOverlapMs     int `json:"total_overlap_ms"`
}

// EstimationParams configures the estimation
type EstimationParams struct {
	Engine         string // "say" or "gemini"
	WordsPerMinute int    // Override default WPM
	Validate       bool   // Check for timing conflicts
}

// SingleEstimate represents estimation for a single text
type SingleEstimate struct {
	Text           string  `json:"text"`
	WordCount      int     `json:"word_count"`
	EstimatedMs    int     `json:"estimated_ms"`
	EstimatedSec   float64 `json:"estimated_sec"`
	Engine         string  `json:"engine"`
	WordsPerMinute int     `json:"words_per_minute"`
}

// EstimateDuration estimates TTS duration for a single text
func EstimateDuration(text string, wpm int) int {
	wordCount := countWords(text)
	if wpm <= 0 {
		wpm = DefaultSayWPM
	}
	// duration_ms = (words / wpm) * 60 * 1000
	return (wordCount * 60 * 1000) / wpm
}

// EstimateSegments estimates duration for all segments
func EstimateSegments(segments []types.Segment, params EstimationParams) *EstimationResult {
	// Determine WPM
	wpm := params.WordsPerMinute
	if wpm <= 0 {
		if params.Engine == "gemini" {
			wpm = DefaultGeminiWPM
		} else {
			wpm = DefaultSayWPM
		}
	}

	engine := params.Engine
	if engine == "" {
		engine = "say"
	}

	result := &EstimationResult{
		Engine:         engine,
		WordsPerMinute: wpm,
		TotalSegments:  len(segments),
		Segments:       make([]DurationEstimate, 0, len(segments)),
	}

	totalDuration := 0
	totalWords := 0
	overlaps := 0
	tightFits := 0
	totalOverlap := 0

	for i, seg := range segments {
		wordCount := countWords(seg.Text)
		estimatedMs := EstimateDuration(seg.Text, wpm)
		startMs := seg.TimestampMs
		endMs := startMs + estimatedMs

		estimate := DurationEstimate{
			SegmentIndex: seg.Index,
			Text:         truncateText(seg.Text, 50),
			WordCount:    wordCount,
			EstimatedMs:  estimatedMs,
			StartMs:      startMs,
			EndMs:        endMs,
		}

		// Check against next segment if validating
		if params.Validate && i < len(segments)-1 {
			nextStart := segments[i+1].TimestampMs
			gap := nextStart - endMs
			estimate.NextStartMs = nextStart
			estimate.GapMs = gap

			if gap < 0 {
				estimate.WillOverlap = true
				estimate.OverlapMs = -gap
				overlaps++
				totalOverlap += -gap
			} else if gap < 500 {
				tightFits++
			}
		}

		result.Segments = append(result.Segments, estimate)
		totalDuration += estimatedMs
		totalWords += wordCount
	}

	result.Summary = EstimationSummary{
		TotalDurationMs:    totalDuration,
		TotalWordCount:     totalWords,
		PredictedOverlaps:  overlaps,
		PredictedTightFits: tightFits,
		TotalOverlapMs:     totalOverlap,
	}

	return result
}

// EstimateSingleText estimates duration for arbitrary text
func EstimateSingleText(text string, engine string, wpm int) *SingleEstimate {
	if wpm <= 0 {
		if engine == "gemini" {
			wpm = DefaultGeminiWPM
		} else {
			wpm = DefaultSayWPM
		}
	}

	if engine == "" {
		engine = "say"
	}

	wordCount := countWords(text)
	durationMs := EstimateDuration(text, wpm)

	return &SingleEstimate{
		Text:           text,
		WordCount:      wordCount,
		EstimatedMs:    durationMs,
		EstimatedSec:   float64(durationMs) / 1000.0,
		Engine:         engine,
		WordsPerMinute: wpm,
	}
}

// countWords counts words in text (handles contractions, hyphens)
func countWords(text string) int {
	// Split on whitespace and count non-empty parts
	words := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r)
	})
	return len(words)
}

// truncateText truncates text for display
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
