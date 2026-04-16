package validation

import (
	"fmt"
	"os/exec"
	"strings"
)

// ExtractText performs OCR on an image and returns the extracted text
func ExtractText(imagePath string) (string, error) {
	// Use tesseract CLI directly for simplicity
	// tesseract <input> stdout -l eng
	cmd := exec.Command("tesseract", imagePath, "stdout", "-l", "eng")

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tesseract OCR failed: %w", err)
	}

	// Clean up the text
	text := strings.TrimSpace(string(output))
	return text, nil
}

// ExtractTextFromFrames performs OCR on all frames and updates FrameInfo
func ExtractTextFromFrames(frames []FrameInfo) ([]FrameInfo, error) {
	result := make([]FrameInfo, len(frames))
	copy(result, frames)

	for i := range result {
		text, err := ExtractText(result[i].FramePath)
		if err != nil {
			// Log warning but continue - some frames may fail OCR
			result[i].OCRText = fmt.Sprintf("[OCR Error: %v]", err)
			continue
		}
		result[i].OCRText = text
	}

	return result, nil
}

// NormalizeText normalizes text for comparison (lowercase, remove extra whitespace)
func NormalizeText(text string) string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Replace multiple whitespace with single space
	fields := strings.Fields(text)
	text = strings.Join(fields, " ")

	return text
}

// ContainsAny checks if text contains any of the search terms (case-insensitive)
func ContainsAny(text string, searchTerms []string) (bool, []string) {
	normalizedText := NormalizeText(text)
	found := []string{}

	for _, term := range searchTerms {
		normalizedTerm := NormalizeText(term)
		if strings.Contains(normalizedText, normalizedTerm) {
			found = append(found, term)
		}
	}

	return len(found) > 0, found
}
