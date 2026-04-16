package video

import (
	"time"

	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
)

// TTSOpts configures a TTS generation run across all segments.
type TTSOpts struct {
	Segments       []types.Segment
	OutputDir      string
	Engine         string        // "say", "gemini", "cloudtts", or "vertex"
	Voice          string
	WordsPerMinute int
	Model          string        // model name (ignored for say)
	UpdateJSON     bool
	SegmentsFile   string        // path to segments.json (for extension updates)
	Timeout        time.Duration // API timeout (ignored for say)
	Verbose        bool          // log raw HTTP on errors
	Project        string        // project name for OAuth (cloudtts, vertex)
}

// SpeechOpts configures a single Gemini TTS API call.
type SpeechOpts struct {
	Text           string
	OutputPath     string
	Voice          string
	Model          string
	WordsPerMinute int
	MaxRetries     int
	Verbose        bool
}
