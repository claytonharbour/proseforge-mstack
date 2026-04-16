package forms

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// getResponses fetches responses for a form
func getResponses(ctx context.Context, project string, formID string, params ResponseParams) (*ResponseList, error) {
	svc, err := getFormsService(ctx, project)
	if err != nil {
		return nil, err
	}

	// Set default page size
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 5000 {
		pageSize = 5000 // API limit
	}

	call := svc.Forms.Responses.List(formID).PageSize(int64(pageSize))

	if params.PageToken != "" {
		call = call.PageToken(params.PageToken)
	}

	result, err := call.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list responses: %w", err)
	}

	// Get form details for question titles
	form, err := getForm(ctx, project, formID)
	if err != nil {
		// Continue without question titles
		fmt.Fprintf(os.Stderr, "Warning: could not fetch form details: %v\n", err)
	}

	// Build question ID to title map
	questionTitles := make(map[string]string)
	if form != nil {
		for _, item := range form.Items {
			questionTitles[item.ItemID] = item.Title
		}
	}

	responseList := &ResponseList{
		TotalCount:    len(result.Responses),
		NextPageToken: result.NextPageToken,
	}

	for _, resp := range result.Responses {
		formResp := FormResponse{
			ResponseID: resp.ResponseId,
			Answers:    make(map[string]Answer),
		}

		// Parse timestamps
		if resp.CreateTime != "" {
			if t, err := time.Parse(time.RFC3339, resp.CreateTime); err == nil {
				formResp.CreateTime = t
			}
		}
		if resp.LastSubmittedTime != "" {
			if t, err := time.Parse(time.RFC3339, resp.LastSubmittedTime); err == nil {
				formResp.LastSubmittedAt = t
			}
		}

		formResp.RespondentEmail = resp.RespondentEmail

		// Convert answers
		for qID, answer := range resp.Answers {
			ans := Answer{
				QuestionID: qID,
				Question:   questionTitles[qID],
			}

			// Extract text answers
			if answer.TextAnswers != nil {
				for _, textAns := range answer.TextAnswers.Answers {
					ans.Values = append(ans.Values, textAns.Value)
				}
			}

			formResp.Answers[qID] = ans
		}

		responseList.Responses = append(responseList.Responses, formResp)
	}

	return responseList, nil
}

// exportResponses exports all responses to a JSON file
func exportResponses(ctx context.Context, project string, formID string, outputPath string) (*ExportResult, error) {
	result := &ExportResult{
		Success: false,
	}

	// Get form details
	form, err := getForm(ctx, project, formID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get form: %v", err)
		return result, err
	}

	result.FormTitle = form.Title

	// Fetch all responses (paginated)
	var allResponses []FormResponse
	pageToken := ""

	for {
		params := ResponseParams{
			PageSize:  5000, // Max per page
			PageToken: pageToken,
		}

		respList, err := getResponses(ctx, project, formID, params)
		if err != nil {
			result.Error = fmt.Sprintf("failed to fetch responses: %v", err)
			return result, err
		}

		allResponses = append(allResponses, respList.Responses...)

		pageToken = respList.NextPageToken
		if pageToken == "" {
			break
		}
	}

	result.ResponseCount = len(allResponses)

	// Build export data
	exportData := ExportedData{
		Form: FormSummary{
			FormID:       form.FormID,
			Title:        form.Title,
			ResponderURL: form.ResponderURL,
		},
		Responses:  allResponses,
		ExportedAt: time.Now(),
	}

	// Determine output path
	if outputPath == "" {
		// Default path: projects/<project>/forms/<form-title>/responses.json
		projectsRoot := os.Getenv("MSTACK_PROJECTS_ROOT")
		if projectsRoot == "" {
			projectsRoot = "projects"
		}

		// Sanitize form title for filename
		safeTitle := sanitizeFilename(form.Title)
		outputPath = filepath.Join(projectsRoot, project, "forms", safeTitle, "responses.json")
	}

	result.OutputPath = outputPath

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Error = fmt.Sprintf("failed to create directory: %v", err)
		return result, err
	}

	// Write JSON
	data, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		result.Error = fmt.Sprintf("failed to marshal JSON: %v", err)
		return result, err
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		result.Error = fmt.Sprintf("failed to write file: %v", err)
		return result, err
	}

	result.Success = true
	return result, nil
}

// sanitizeFilename removes or replaces characters that aren't safe for filenames
func sanitizeFilename(name string) string {
	// Replace problematic characters with underscores
	result := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			result = append(result, r)
		case r >= 'A' && r <= 'Z':
			result = append(result, r)
		case r >= '0' && r <= '9':
			result = append(result, r)
		case r == '-' || r == '_' || r == '.':
			result = append(result, r)
		case r == ' ':
			result = append(result, '-')
		default:
			// Skip other characters
		}
	}

	s := string(result)
	if s == "" {
		s = "form"
	}

	// Truncate if too long
	if len(s) > 100 {
		s = s[:100]
	}

	return s
}
