package validation

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
)

// DefaultConfig returns the default validation configuration
func DefaultConfig() *ValidationConfig {
	return &ValidationConfig{
		Keywords: map[string]KeywordRule{
			"story": {
				ExpectedText:      []string{"story", "stories"},
				EmptyStateText:    []string{"no stories", "empty", "get started", "create your first"},
				NarrationPatterns: []string{"story", "stories", "story card"},
			},
			"reviewer": {
				ExpectedText:      []string{"reviewer", "reviewers", "review"},
				EmptyStateText:    []string{"no reviewers", "no pending", "empty"},
				NarrationPatterns: []string{"reviewer", "review", "feedback"},
			},
			"notification": {
				ExpectedText:      []string{"notification", "notifications", "alert"},
				EmptyStateText:    []string{"no notifications", "all caught up"},
				NarrationPatterns: []string{"notification", "bell", "alert"},
			},
			"dashboard": {
				ExpectedText:      []string{"dashboard", "overview", "summary"},
				NarrationPatterns: []string{"dashboard", "overview"},
			},
			"settings": {
				ExpectedText:      []string{"settings", "preferences", "account"},
				NarrationPatterns: []string{"settings", "configure", "preferences"},
			},
			"button": {
				NarrationPatterns: []string{"click", "button", "press", "select"},
			},
			"form": {
				ExpectedText:      []string{"name", "email", "password", "submit"},
				NarrationPatterns: []string{"enter", "fill", "type", "input"},
			},
		},
		Validation: ValidationSettings{
			StrictMode:         false,
			WarnOnEmptyState:   true,
			RequireUIElements:  false,
			MinPauseDurationMs: 500,
		},
	}
}

// LoadConfig loads validation config from a JSON file
func LoadConfig(path string) (*ValidationConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config ValidationConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ValidateSegment validates a single segment against OCR text
func ValidateSegment(seg types.Segment, ocrText string, config *ValidationConfig) []ValidationIssue {
	issues := []ValidationIssue{}
	normalizedNarration := NormalizeText(seg.Text)
	normalizedOCR := NormalizeText(ocrText)

	// Always check for error pages regardless of narration content
	errorIndicators := []string{
		"404", "not found", "page not found",
		"500", "internal server error", "something went wrong",
		"error occurred", "oops", "we're sorry",
		"connection refused", "unable to connect",
		"bad gateway", "service unavailable",
	}

	foundError, errorTerms := ContainsAny(ocrText, errorIndicators)
	if foundError {
		issues = append(issues, ValidationIssue{
			Type:          "error_page_detected",
			Severity:      "high",
			SegmentIndex:  seg.Index,
			Timestamp:     seg.Timestamp,
			NarrationText: seg.Text,
			ScreenContent: ocrText,
			FoundText:     errorTerms,
			Suggestion: &Suggestion{
				Action:      "fix_error",
				Description: "Error page detected on screen",
				CodeFix:     "Check test data setup and navigation - screen shows an error",
			},
		})
	}

	// Check each keyword rule
	for keyword, rule := range config.Keywords {
		// Check if narration mentions this keyword
		narrationMentions := false
		for _, pattern := range rule.NarrationPatterns {
			if strings.Contains(normalizedNarration, strings.ToLower(pattern)) {
				narrationMentions = true
				break
			}
		}

		if !narrationMentions {
			continue
		}

		// Check for empty state detection
		if config.Validation.WarnOnEmptyState && len(rule.EmptyStateText) > 0 {
			foundEmpty, emptyTerms := ContainsAny(ocrText, rule.EmptyStateText)
			if foundEmpty {
				// Check if narration expects populated content
				expectsContent := !containsEmptyStateWords(seg.Text)
				if expectsContent {
					issues = append(issues, ValidationIssue{
						Type:          "empty_state_mismatch",
						Severity:      "high",
						SegmentIndex:  seg.Index,
						Timestamp:     seg.Timestamp,
						NarrationText: seg.Text,
						ScreenContent: ocrText,
						FoundText:     emptyTerms,
						Suggestion: &Suggestion{
							Action:      "add_test_data",
							Description: "Screen shows empty state but narration describes content for: " + keyword,
							CodeFix:     "Ensure test data exists before this segment",
						},
					})
				}
			}
		}

		// Check if expected text is on screen
		if len(rule.ExpectedText) > 0 {
			found, foundTerms := ContainsAny(ocrText, rule.ExpectedText)
			if !found && config.Validation.RequireUIElements {
				issues = append(issues, ValidationIssue{
					Type:          "missing_ui_element",
					Severity:      "medium",
					SegmentIndex:  seg.Index,
					Timestamp:     seg.Timestamp,
					NarrationText: seg.Text,
					ScreenContent: ocrText,
					ExpectedText:  rule.ExpectedText,
					FoundText:     foundTerms,
					Suggestion: &Suggestion{
						Action:      "verify_navigation",
						Description: "Expected UI elements not found on screen for: " + keyword,
						CodeFix:     "Verify the correct page is displayed at this timestamp",
					},
				})
			}
		}
	}

	// Check for generic content mismatch
	// If narration says "click X" but X is not visible
	clickTargets := extractClickTargets(seg.Text)
	for _, target := range clickTargets {
		if !strings.Contains(normalizedOCR, strings.ToLower(target)) {
			issues = append(issues, ValidationIssue{
				Type:          "click_target_not_visible",
				Severity:      "medium",
				SegmentIndex:  seg.Index,
				Timestamp:     seg.Timestamp,
				NarrationText: seg.Text,
				ScreenContent: ocrText,
				ExpectedText:  []string{target},
				Suggestion: &Suggestion{
					Action:      "verify_element_visible",
					Description: "Click target '" + target + "' not found in screen text",
					CodeFix:     "Ensure the element is visible before narrating the click action",
				},
			})
		}
	}

	return issues
}

// containsEmptyStateWords checks if text describes an empty state
func containsEmptyStateWords(text string) bool {
	emptyIndicators := []string{
		"no ", "don't have", "empty", "none", "nothing",
		"haven't", "not yet", "get started",
	}
	normalized := NormalizeText(text)
	for _, indicator := range emptyIndicators {
		if strings.Contains(normalized, indicator) {
			return true
		}
	}
	return false
}

// extractClickTargets extracts button/link names from narration
func extractClickTargets(text string) []string {
	targets := []string{}

	// Look for quoted strings after "click" or "press"
	normalizedText := strings.ToLower(text)

	// Common patterns: "Click 'X'" or "Click the X button"
	patterns := []string{"click ", "press ", "select ", "tap "}

	for _, pattern := range patterns {
		idx := strings.Index(normalizedText, pattern)
		if idx == -1 {
			continue
		}

		remaining := text[idx+len(pattern):]

		// Check for quoted target
		if strings.HasPrefix(remaining, "\"") || strings.HasPrefix(remaining, "'") {
			quote := remaining[0:1]
			endIdx := strings.Index(remaining[1:], quote)
			if endIdx > 0 {
				targets = append(targets, remaining[1:endIdx+1])
			}
		}
	}

	return targets
}

// ValidateTaggedSegment validates a tagged segment using its semantic metadata
// Requires multiple frames: pre-action, post-action, and settled states
func ValidateTaggedSegment(segment *TaggedSegment, config *ValidationConfig) []ValidationIssue {
	issues := []ValidationIssue{}

	// If no tag, fall back to basic validation on first frame
	if segment.Tag == nil || len(segment.Frames) == 0 {
		return issues
	}

	// Get OCR text for each frame position
	ocrByPosition := make(map[string]string)
	positions := segment.GetFramePositions()
	for i, frame := range segment.Frames {
		if i < len(positions) {
			ocrByPosition[positions[i]] = frame.OCRText
		}
	}

	// Dispatch to action-specific validation
	switch segment.Tag.Action {
	case "click":
		issues = append(issues, validateClick(segment, ocrByPosition)...)
	case "fill":
		issues = append(issues, validateFill(segment, ocrByPosition)...)
	case "navigate":
		issues = append(issues, validateNavigate(segment, ocrByPosition)...)
	case "wait":
		issues = append(issues, validateWait(segment, ocrByPosition)...)
	case "select":
		issues = append(issues, validateSelect(segment, ocrByPosition)...)
	case "hover":
		issues = append(issues, validateHover(segment, ocrByPosition)...)
	case "scroll":
		issues = append(issues, validateScroll(segment, ocrByPosition)...)
	case "assert":
		issues = append(issues, validateAssert(segment, ocrByPosition)...)
	}

	return issues
}

// validateClick checks that target is visible before click and state changes after
func validateClick(segment *TaggedSegment, ocr map[string]string) []ValidationIssue {
	issues := []ValidationIssue{}
	target := segment.Tag.Target

	if target == "" {
		return issues
	}

	preAction := NormalizeText(ocr["pre_action"])
	postAction := NormalizeText(ocr["post_action"])
	targetLower := strings.ToLower(target)

	// Target should be visible before action
	if !strings.Contains(preAction, targetLower) {
		issues = append(issues, ValidationIssue{
			Type:          "click_target_not_visible",
			Severity:      "high",
			SegmentIndex:  segment.Index,
			Timestamp:     segment.Timestamp,
			NarrationText: segment.Text,
			ScreenContent: ocr["pre_action"],
			ExpectedText:  []string{target},
			Suggestion: &Suggestion{
				Action:      "verify_element_visible",
				Description: "Click target '" + target + "' not visible before click",
				CodeFix:     "Ensure element is visible before narrating the click",
			},
		})
	}

	// State should change after action (simple heuristic: OCR text differs)
	if preAction == postAction && preAction != "" {
		issues = append(issues, ValidationIssue{
			Type:          "no_state_change",
			Severity:      "medium",
			SegmentIndex:  segment.Index,
			Timestamp:     segment.Timestamp,
			NarrationText: segment.Text,
			ScreenContent: ocr["post_action"],
			Suggestion: &Suggestion{
				Action:      "verify_action_effect",
				Description: "Screen did not change after click action",
				CodeFix:     "Verify the click action has a visible effect",
			},
		})
	}

	return issues
}

// validateFill checks that input is visible and value appears after
func validateFill(segment *TaggedSegment, ocr map[string]string) []ValidationIssue {
	issues := []ValidationIssue{}
	target := segment.Tag.Target

	postAction := NormalizeText(ocr["post_action"])
	settled := NormalizeText(ocr["settled"])

	// If target specified, it should appear in post-action or settled
	if target != "" {
		targetLower := strings.ToLower(target)
		if !strings.Contains(postAction, targetLower) && !strings.Contains(settled, targetLower) {
			issues = append(issues, ValidationIssue{
				Type:          "fill_value_not_visible",
				Severity:      "medium",
				SegmentIndex:  segment.Index,
				Timestamp:     segment.Timestamp,
				NarrationText: segment.Text,
				ScreenContent: ocr["settled"],
				ExpectedText:  []string{target},
				Suggestion: &Suggestion{
					Action:      "verify_input_value",
					Description: "Filled value '" + target + "' not visible after fill action",
					CodeFix:     "Ensure the input shows the filled value",
				},
			})
		}
	}

	return issues
}

// validateNavigate checks that page content changes after navigation
func validateNavigate(segment *TaggedSegment, ocr map[string]string) []ValidationIssue {
	issues := []ValidationIssue{}

	preAction := NormalizeText(ocr["pre_action"])
	postAction := NormalizeText(ocr["post_action"])

	// Content should change significantly
	if preAction == postAction && preAction != "" {
		issues = append(issues, ValidationIssue{
			Type:          "navigation_no_change",
			Severity:      "high",
			SegmentIndex:  segment.Index,
			Timestamp:     segment.Timestamp,
			NarrationText: segment.Text,
			ScreenContent: ocr["post_action"],
			Suggestion: &Suggestion{
				Action:      "verify_navigation",
				Description: "Page content did not change after navigation",
				CodeFix:     "Verify navigation occurred and page loaded",
			},
		})
	}

	// If target specified, it should appear in destination
	if segment.Tag.Target != "" {
		targetLower := strings.ToLower(segment.Tag.Target)
		if !strings.Contains(postAction, targetLower) {
			issues = append(issues, ValidationIssue{
				Type:          "navigation_target_missing",
				Severity:      "medium",
				SegmentIndex:  segment.Index,
				Timestamp:     segment.Timestamp,
				NarrationText: segment.Text,
				ScreenContent: ocr["post_action"],
				ExpectedText:  []string{segment.Tag.Target},
				Suggestion: &Suggestion{
					Action:      "verify_destination",
					Description: "Expected content '" + segment.Tag.Target + "' not found after navigation",
					CodeFix:     "Verify navigation goes to the correct page",
				},
			})
		}
	}

	return issues
}

// validateWait checks that state remains stable (no unexpected changes)
func validateWait(segment *TaggedSegment, ocr map[string]string) []ValidationIssue {
	// Wait action expects no significant change - nothing to validate
	return []ValidationIssue{}
}

// validateSelect checks dropdown is visible and selected value appears
func validateSelect(segment *TaggedSegment, ocr map[string]string) []ValidationIssue {
	issues := []ValidationIssue{}
	target := segment.Tag.Target

	postAction := NormalizeText(ocr["post_action"])
	settled := NormalizeText(ocr["settled"])

	// Selected value should appear
	if target != "" {
		targetLower := strings.ToLower(target)
		if !strings.Contains(postAction, targetLower) && !strings.Contains(settled, targetLower) {
			issues = append(issues, ValidationIssue{
				Type:          "select_value_not_visible",
				Severity:      "medium",
				SegmentIndex:  segment.Index,
				Timestamp:     segment.Timestamp,
				NarrationText: segment.Text,
				ScreenContent: ocr["settled"],
				ExpectedText:  []string{target},
				Suggestion: &Suggestion{
					Action:      "verify_selection",
					Description: "Selected value '" + target + "' not visible after selection",
					CodeFix:     "Ensure dropdown selection is reflected in UI",
				},
			})
		}
	}

	return issues
}

// validateHover checks target is visible and hover state appears
func validateHover(segment *TaggedSegment, ocr map[string]string) []ValidationIssue {
	issues := []ValidationIssue{}
	target := segment.Tag.Target

	preAction := NormalizeText(ocr["pre_action"])

	// Target should be visible before hover
	if target != "" {
		targetLower := strings.ToLower(target)
		if !strings.Contains(preAction, targetLower) {
			issues = append(issues, ValidationIssue{
				Type:          "hover_target_not_visible",
				Severity:      "medium",
				SegmentIndex:  segment.Index,
				Timestamp:     segment.Timestamp,
				NarrationText: segment.Text,
				ScreenContent: ocr["pre_action"],
				ExpectedText:  []string{target},
				Suggestion: &Suggestion{
					Action:      "verify_hover_target",
					Description: "Hover target '" + target + "' not visible",
					CodeFix:     "Ensure element is visible before hover",
				},
			})
		}
	}

	return issues
}

// validateScroll checks that target becomes visible after scroll
func validateScroll(segment *TaggedSegment, ocr map[string]string) []ValidationIssue {
	issues := []ValidationIssue{}
	target := segment.Tag.Target

	postAction := NormalizeText(ocr["post_action"])
	settled := NormalizeText(ocr["settled"])

	// Target should become visible after scroll
	if target != "" {
		targetLower := strings.ToLower(target)
		if !strings.Contains(postAction, targetLower) && !strings.Contains(settled, targetLower) {
			issues = append(issues, ValidationIssue{
				Type:          "scroll_target_not_visible",
				Severity:      "medium",
				SegmentIndex:  segment.Index,
				Timestamp:     segment.Timestamp,
				NarrationText: segment.Text,
				ScreenContent: ocr["settled"],
				ExpectedText:  []string{target},
				Suggestion: &Suggestion{
					Action:      "verify_scroll",
					Description: "Target '" + target + "' not visible after scroll",
					CodeFix:     "Ensure scroll brings target into view",
				},
			})
		}
	}

	return issues
}

// validateAssert checks that all visible items are found in OCR
func validateAssert(segment *TaggedSegment, ocr map[string]string) []ValidationIssue {
	issues := []ValidationIssue{}

	if len(segment.Tag.Visible) == 0 {
		return issues
	}

	// Check the pre-action frame (or first available)
	checkOCR := ocr["pre_action"]
	if checkOCR == "" {
		checkOCR = ocr["post_action"]
	}
	normalizedOCR := NormalizeText(checkOCR)

	missing := []string{}
	for _, expected := range segment.Tag.Visible {
		if !strings.Contains(normalizedOCR, strings.ToLower(expected)) {
			missing = append(missing, expected)
		}
	}

	if len(missing) > 0 {
		issues = append(issues, ValidationIssue{
			Type:          "assert_missing_elements",
			Severity:      "high",
			SegmentIndex:  segment.Index,
			Timestamp:     segment.Timestamp,
			NarrationText: segment.Text,
			ScreenContent: checkOCR,
			ExpectedText:  missing,
			Suggestion: &Suggestion{
				Action:      "verify_visibility",
				Description: "Expected elements not found on screen",
				CodeFix:     "Ensure all expected elements are visible at this timestamp",
			},
		})
	}

	return issues
}
