package forms

import (
	"context"
	"fmt"

	"github.com/claytonharbour/proseforge-mstack/src/internal/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/forms/v1"
	"google.golang.org/api/option"
)

// getFormsService returns an authenticated Google Forms API service
func getFormsService(ctx context.Context, project string) (*forms.Service, error) {
	client, err := google.GetClientForProject(ctx, project)
	if err != nil {
		return nil, err
	}

	svc, err := forms.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create Forms service: %w", err)
	}

	return svc, nil
}

// getDriveService returns an authenticated Google Drive API service
func getDriveService(ctx context.Context, project string) (*drive.Service, error) {
	client, err := google.GetClientForProject(ctx, project)
	if err != nil {
		return nil, err
	}

	svc, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create Drive service: %w", err)
	}

	return svc, nil
}

// createForm creates a new Google Form
func createForm(ctx context.Context, project string, params CreateFormParams) (*Form, error) {
	svc, err := getFormsService(ctx, project)
	if err != nil {
		return nil, err
	}

	// Create the form with just the title first
	formReq := &forms.Form{
		Info: &forms.Info{
			Title:         params.Title,
			DocumentTitle: params.Title,
		},
	}

	created, err := svc.Forms.Create(formReq).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create form: %w", err)
	}

	// If we have items to add, use batch update
	if len(params.Items) > 0 {
		var requests []*forms.Request

		for i, item := range params.Items {
			createItemReq := buildCreateItemRequest(item, i)
			requests = append(requests, createItemReq)
		}

		if len(requests) > 0 {
			batchReq := &forms.BatchUpdateFormRequest{
				Requests: requests,
			}

			_, err = svc.Forms.BatchUpdate(created.FormId, batchReq).Context(ctx).Do()
			if err != nil {
				return nil, fmt.Errorf("failed to add form items: %w", err)
			}
		}
	}

	// If description provided, add it via batch update
	if params.Description != "" {
		descReq := &forms.BatchUpdateFormRequest{
			Requests: []*forms.Request{
				{
					UpdateFormInfo: &forms.UpdateFormInfoRequest{
						Info: &forms.Info{
							Description: params.Description,
						},
						UpdateMask: "description",
					},
				},
			},
		}

		_, err = svc.Forms.BatchUpdate(created.FormId, descReq).Context(ctx).Do()
		if err != nil {
			// Log but don't fail - form is created
			fmt.Printf("Warning: failed to set description: %v\n", err)
		}
	}

	// Fetch the complete form to return
	return getForm(ctx, project, created.FormId)
}

// buildCreateItemRequest builds a Forms API request for creating an item
func buildCreateItemRequest(item CreateFormItem, index int) *forms.Request {
	req := &forms.Request{
		CreateItem: &forms.CreateItemRequest{
			Item: &forms.Item{
				Title:       item.Title,
				Description: item.Description,
			},
			Location: &forms.Location{
				Index:           int64(index),
				ForceSendFields: []string{"Index"}, // Force sending index=0
			},
		},
	}

	// Set the question type based on item.Type
	switch item.Type {
	case FormItemTypeText:
		req.CreateItem.Item.QuestionItem = &forms.QuestionItem{
			Question: &forms.Question{
				Required: item.Required,
				TextQuestion: &forms.TextQuestion{
					Paragraph: false,
				},
			},
		}

	case FormItemTypeParagraph:
		req.CreateItem.Item.QuestionItem = &forms.QuestionItem{
			Question: &forms.Question{
				Required: item.Required,
				TextQuestion: &forms.TextQuestion{
					Paragraph: true,
				},
			},
		}

	case FormItemTypeChoice:
		options := make([]*forms.Option, len(item.Options))
		for i, opt := range item.Options {
			options[i] = &forms.Option{Value: opt}
		}
		req.CreateItem.Item.QuestionItem = &forms.QuestionItem{
			Question: &forms.Question{
				Required: item.Required,
				ChoiceQuestion: &forms.ChoiceQuestion{
					Type:    "RADIO",
					Options: options,
				},
			},
		}

	case FormItemTypeCheckbox:
		options := make([]*forms.Option, len(item.Options))
		for i, opt := range item.Options {
			options[i] = &forms.Option{Value: opt}
		}
		req.CreateItem.Item.QuestionItem = &forms.QuestionItem{
			Question: &forms.Question{
				Required: item.Required,
				ChoiceQuestion: &forms.ChoiceQuestion{
					Type:    "CHECKBOX",
					Options: options,
				},
			},
		}

	case FormItemTypeDropdown:
		options := make([]*forms.Option, len(item.Options))
		for i, opt := range item.Options {
			options[i] = &forms.Option{Value: opt}
		}
		req.CreateItem.Item.QuestionItem = &forms.QuestionItem{
			Question: &forms.Question{
				Required: item.Required,
				ChoiceQuestion: &forms.ChoiceQuestion{
					Type:    "DROP_DOWN",
					Options: options,
				},
			},
		}

	case FormItemTypeScale:
		req.CreateItem.Item.QuestionItem = &forms.QuestionItem{
			Question: &forms.Question{
				Required: item.Required,
				ScaleQuestion: &forms.ScaleQuestion{
					Low:      1,
					High:     5,
					LowLabel: "Low",
					HighLabel: "High",
				},
			},
		}

	case FormItemTypeDate:
		req.CreateItem.Item.QuestionItem = &forms.QuestionItem{
			Question: &forms.Question{
				Required: item.Required,
				DateQuestion: &forms.DateQuestion{
					IncludeTime: false,
					IncludeYear: true,
				},
			},
		}

	case FormItemTypeTime:
		req.CreateItem.Item.QuestionItem = &forms.QuestionItem{
			Question: &forms.Question{
				Required: item.Required,
				TimeQuestion: &forms.TimeQuestion{
					Duration: false,
				},
			},
		}

	default:
		// Default to short text
		req.CreateItem.Item.QuestionItem = &forms.QuestionItem{
			Question: &forms.Question{
				Required: item.Required,
				TextQuestion: &forms.TextQuestion{
					Paragraph: false,
				},
			},
		}
	}

	return req
}

// listForms lists all Google Forms accessible to the user via Drive API
func listForms(ctx context.Context, project string) ([]FormSummary, error) {
	driveSvc, err := getDriveService(ctx, project)
	if err != nil {
		return nil, err
	}

	// Query for Google Forms files
	// mimeType for Google Forms is 'application/vnd.google-apps.form'
	query := "mimeType='application/vnd.google-apps.form' and trashed=false"

	var allForms []FormSummary
	pageToken := ""

	for {
		call := driveSvc.Files.List().
			Q(query).
			Fields("nextPageToken, files(id, name, webViewLink)").
			PageSize(100).
			OrderBy("modifiedTime desc")

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		result, err := call.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list forms: %w", err)
		}

		for _, file := range result.Files {
			allForms = append(allForms, FormSummary{
				FormID:       file.Id,
				Title:        file.Name,
				ResponderURL: fmt.Sprintf("https://docs.google.com/forms/d/e/%s/viewform", file.Id),
			})
		}

		pageToken = result.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return allForms, nil
}

// getForm retrieves detailed information about a form
func getForm(ctx context.Context, project string, formID string) (*Form, error) {
	svc, err := getFormsService(ctx, project)
	if err != nil {
		return nil, err
	}

	apiForm, err := svc.Forms.Get(formID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get form: %w", err)
	}

	form := &Form{
		FormID:       apiForm.FormId,
		Title:        apiForm.Info.Title,
		Description:  apiForm.Info.Description,
		ResponderURL: apiForm.ResponderUri,
		LinkedSheet:  apiForm.LinkedSheetId,
	}

	// Convert items
	for _, item := range apiForm.Items {
		formItem := convertAPIItem(item)
		if formItem != nil {
			form.Items = append(form.Items, *formItem)
		}
	}

	return form, nil
}

// convertAPIItem converts a Google Forms API item to our FormItem type
func convertAPIItem(item *forms.Item) *FormItem {
	if item.QuestionItem == nil {
		return nil // Skip non-question items (like page breaks, images, etc.)
	}

	formItem := &FormItem{
		ItemID:      item.ItemId,
		Title:       item.Title,
		Description: item.Description,
	}

	q := item.QuestionItem.Question
	if q == nil {
		return nil
	}

	formItem.Required = q.Required

	// Determine type and extract options
	switch {
	case q.TextQuestion != nil:
		if q.TextQuestion.Paragraph {
			formItem.Type = FormItemTypeParagraph
		} else {
			formItem.Type = FormItemTypeText
		}

	case q.ChoiceQuestion != nil:
		switch q.ChoiceQuestion.Type {
		case "RADIO":
			formItem.Type = FormItemTypeChoice
		case "CHECKBOX":
			formItem.Type = FormItemTypeCheckbox
		case "DROP_DOWN":
			formItem.Type = FormItemTypeDropdown
		default:
			formItem.Type = FormItemTypeChoice
		}
		for _, opt := range q.ChoiceQuestion.Options {
			formItem.Options = append(formItem.Options, opt.Value)
		}

	case q.ScaleQuestion != nil:
		formItem.Type = FormItemTypeScale

	case q.DateQuestion != nil:
		formItem.Type = FormItemTypeDate

	case q.TimeQuestion != nil:
		formItem.Type = FormItemTypeTime

	case q.FileUploadQuestion != nil:
		formItem.Type = FormItemTypeFileUpload

	default:
		formItem.Type = FormItemTypeText
	}

	return formItem
}
