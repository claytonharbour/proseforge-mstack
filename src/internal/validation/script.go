package validation

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// NarrationCall represents a narrator call found in a test script
type NarrationCall struct {
	LineNumber  int    `json:"line_number"`
	Text        string `json:"text"`
	Method      string `json:"method"` // "markAndPause" or "mark"
	CodeContext string `json:"code_context"`
}

// ParseTestScript parses a Playwright test file to extract narration calls
func ParseTestScript(scriptPath string) ([]NarrationCall, error) {
	file, err := os.Open(scriptPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var calls []NarrationCall
	scanner := bufio.NewScanner(file)

	// Patterns for narrator calls
	// Single-line: narrator.markAndPause(page, 'text', 3000) or narrator.mark('text')
	markAndPauseSingleLine := regexp.MustCompile(`narrator\.markAndPause\s*\(\s*(?:page\s*,\s*)?['"]([^'"]+)['"]`)
	markSingleLine := regexp.MustCompile(`narrator\.mark\s*\(\s*['"]([^'"]+)['"]`)

	// Multi-line start: narrator.markAndPause( with no text on same line
	markAndPauseStart := regexp.MustCompile(`narrator\.markAndPause\s*\(`)
	markStart := regexp.MustCompile(`narrator\.mark\s*\(`)

	// Text argument pattern (for multi-line continuation)
	textArgPattern := regexp.MustCompile(`^\s*(?:page\s*,\s*)?['"]([^'"]+)['"]`)

	// For multi-line strings, we need to track state
	var pendingCall *NarrationCall
	var multiLineBuffer strings.Builder
	var lookingForText bool
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check for multi-line text continuation (text that spans multiple lines)
		if pendingCall != nil && !lookingForText {
			multiLineBuffer.WriteString(" ")
			multiLineBuffer.WriteString(strings.TrimSpace(line))
			// Check if this line completes the string
			if strings.Contains(line, `',`) || strings.Contains(line, `",`) ||
				strings.Contains(line, `')`) || strings.Contains(line, `")`) {
				// Extract the text from the buffer
				fullText := multiLineBuffer.String()
				// Find the closing quote
				for _, quote := range []string{`',`, `",`, `')`, `")`} {
					if idx := strings.Index(fullText, quote); idx > 0 {
						pendingCall.Text = strings.TrimSpace(fullText[:idx])
						break
					}
				}
				if pendingCall.Text != "" {
					calls = append(calls, *pendingCall)
				}
				pendingCall = nil
				multiLineBuffer.Reset()
			}
			continue
		}

		// Check for multi-line call start (looking for text on subsequent line)
		if pendingCall != nil && lookingForText {
			// Look for the text argument on this line
			if matches := textArgPattern.FindStringSubmatch(line); len(matches) > 1 {
				text := matches[1]
				pendingCall.CodeContext = strings.TrimSpace(line)
				// Check if text is complete
				if isCompleteString(line, text) {
					pendingCall.Text = text
					calls = append(calls, *pendingCall)
					pendingCall = nil
					lookingForText = false
				} else {
					// Text continues on next line
					multiLineBuffer.WriteString(text)
					lookingForText = false
				}
			}
			// Skip lines that don't have the text (like "page,")
			continue
		}

		// Check for single-line markAndPause
		if matches := markAndPauseSingleLine.FindStringSubmatch(line); len(matches) > 1 {
			text := matches[1]
			if isCompleteString(line, text) {
				calls = append(calls, NarrationCall{
					LineNumber:  lineNum,
					Text:        text,
					Method:      "markAndPause",
					CodeContext: strings.TrimSpace(line),
				})
			} else {
				// Text continues on next line
				pendingCall = &NarrationCall{
					LineNumber:  lineNum,
					Method:      "markAndPause",
					CodeContext: strings.TrimSpace(line),
				}
				multiLineBuffer.WriteString(text)
			}
			continue
		}

		// Check for multi-line markAndPause start (no text on this line)
		if markAndPauseStart.MatchString(line) && !markAndPauseSingleLine.MatchString(line) {
			pendingCall = &NarrationCall{
				LineNumber:  lineNum,
				Method:      "markAndPause",
				CodeContext: strings.TrimSpace(line),
			}
			lookingForText = true
			continue
		}

		// Check for single-line mark
		if matches := markSingleLine.FindStringSubmatch(line); len(matches) > 1 {
			text := matches[1]
			if isCompleteString(line, text) {
				calls = append(calls, NarrationCall{
					LineNumber:  lineNum,
					Text:        text,
					Method:      "mark",
					CodeContext: strings.TrimSpace(line),
				})
			} else {
				pendingCall = &NarrationCall{
					LineNumber:  lineNum,
					Method:      "mark",
					CodeContext: strings.TrimSpace(line),
				}
				multiLineBuffer.WriteString(text)
			}
			continue
		}

		// Check for multi-line mark start
		if markStart.MatchString(line) && !markSingleLine.MatchString(line) {
			pendingCall = &NarrationCall{
				LineNumber:  lineNum,
				Method:      "mark",
				CodeContext: strings.TrimSpace(line),
			}
			lookingForText = true
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return calls, nil
}

// isCompleteString checks if the string in the line is complete (has closing quote)
func isCompleteString(line, text string) bool {
	// After the text, there should be a closing quote followed by comma or paren
	afterText := strings.Index(line, text)
	if afterText < 0 {
		return false
	}
	remaining := line[afterText+len(text):]
	return strings.Contains(remaining, `',`) || strings.Contains(remaining, `",`) ||
		strings.Contains(remaining, `')`) || strings.Contains(remaining, `")`)
}

// FindMatchingCall finds the NarrationCall that best matches the given narration text
func FindMatchingCall(calls []NarrationCall, narrationText string) *NarrationCall {
	normalizedTarget := NormalizeText(narrationText)

	// First try exact match
	for i := range calls {
		if NormalizeText(calls[i].Text) == normalizedTarget {
			return &calls[i]
		}
	}

	// Then try prefix match (narration might be truncated in segments)
	for i := range calls {
		normalizedCall := NormalizeText(calls[i].Text)
		if strings.HasPrefix(normalizedCall, normalizedTarget) ||
			strings.HasPrefix(normalizedTarget, normalizedCall) {
			return &calls[i]
		}
	}

	// Finally try substring match
	for i := range calls {
		normalizedCall := NormalizeText(calls[i].Text)
		if strings.Contains(normalizedCall, normalizedTarget) ||
			strings.Contains(normalizedTarget, normalizedCall) {
			return &calls[i]
		}
	}

	return nil
}

// FindCallByIndex finds a NarrationCall by its order in the file (1-based index)
func FindCallByIndex(calls []NarrationCall, index int) *NarrationCall {
	if index > 0 && index <= len(calls) {
		return &calls[index-1]
	}
	return nil
}
