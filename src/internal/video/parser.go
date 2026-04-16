package video

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
)

// parseTimestamp converts MM:SS to milliseconds
func parseTimestamp(ts string) (int, error) {
	ts = strings.TrimSpace(ts)
	parts := strings.Split(ts, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid timestamp format: %s (expected MM:SS)", ts)
	}

	minutes, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid minutes in timestamp: %s", ts)
	}

	seconds, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid seconds in timestamp: %s", ts)
	}

	return (minutes*60 + seconds) * 1000, nil
}

// parseNarrationMD parses markdown table format and returns segments
func parseNarrationMD(filepath string) ([]types.Segment, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Match table rows: | 00:01 | Text here |
	// Pattern matches lines like: | 00:01 | Enter your email address to sign in |
	pattern := regexp.MustCompile(`\|\s*(\d{2}:\d{2})\s*\|\s*(.+?)\s*\|`)
	matches := pattern.FindAllStringSubmatch(string(content), -1)

	segments := make([]types.Segment, 0, len(matches))
	for i, match := range matches {
		if len(match) < 3 {
			continue
		}

		timestamp := match[1]
		text := strings.TrimSpace(match[2])

		timestampMs, err := parseTimestamp(timestamp)
		if err != nil {
			return nil, fmt.Errorf("error parsing timestamp %s: %w", timestamp, err)
		}

		segments = append(segments, types.Segment{
			Index:       i + 1,
			Timestamp:   timestamp,
			TimestampMs: timestampMs,
			Text:        text,
			AudioFile:   fmt.Sprintf("segment_%03d.m4a", i+1),
		})
	}

	return segments, nil
}
