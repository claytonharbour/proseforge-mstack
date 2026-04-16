package forms

import (
	"time"
)

// Form represents a Google Form
type Form struct {
	FormID       string     `json:"form_id"`
	Title        string     `json:"title"`
	Description  string     `json:"description,omitempty"`
	DocumentURL  string     `json:"document_url,omitempty"`  // URL to edit form
	ResponderURL string     `json:"responder_url,omitempty"` // URL to fill out form
	LinkedSheet  string     `json:"linked_sheet,omitempty"`  // Linked Google Sheet ID
	Items        []FormItem `json:"items,omitempty"`
}

// FormItem represents a question or section in a form
type FormItem struct {
	ItemID      string       `json:"item_id"`
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Type        FormItemType `json:"type"`
	Required    bool         `json:"required"`
	Options     []string     `json:"options,omitempty"` // For choice/checkbox/dropdown
}

// FormItemType represents the type of form question
type FormItemType string

const (
	FormItemTypeText       FormItemType = "text"
	FormItemTypeParagraph  FormItemType = "paragraph"
	FormItemTypeChoice     FormItemType = "choice"     // Multiple choice (radio)
	FormItemTypeCheckbox   FormItemType = "checkbox"   // Checkboxes
	FormItemTypeDropdown   FormItemType = "dropdown"   // Dropdown
	FormItemTypeScale      FormItemType = "scale"      // Linear scale
	FormItemTypeDate       FormItemType = "date"       // Date
	FormItemTypeTime       FormItemType = "time"       // Time
	FormItemTypeFileUpload FormItemType = "fileUpload" // File upload
	FormItemTypeGrid       FormItemType = "grid"       // Grid (multiple choice)
)

// FormSummary is a minimal form representation for listings
type FormSummary struct {
	FormID       string `json:"form_id"`
	Title        string `json:"title"`
	ResponderURL string `json:"responder_url"`
}

// FormResponse represents a single form submission
type FormResponse struct {
	ResponseID      string            `json:"response_id"`
	CreateTime      time.Time         `json:"create_time"`
	LastSubmittedAt time.Time         `json:"last_submitted_at"`
	RespondentEmail string            `json:"respondent_email,omitempty"`
	Answers         map[string]Answer `json:"answers"` // keyed by question ID
}

// Answer represents a response to a single question
type Answer struct {
	QuestionID string   `json:"question_id"`
	Question   string   `json:"question,omitempty"` // Question title for readability
	Values     []string `json:"values"`
}

// ResponseList holds paginated form responses
type ResponseList struct {
	Responses     []FormResponse `json:"responses"`
	TotalCount    int            `json:"total_count"`
	NextPageToken string         `json:"next_page_token,omitempty"`
}

// CreateFormParams holds parameters for creating a form
type CreateFormParams struct {
	Title       string             `json:"title"`
	Description string             `json:"description,omitempty"`
	Items       []CreateFormItem   `json:"items,omitempty"`
}

// CreateFormItem specifies a form item to create
type CreateFormItem struct {
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Type        FormItemType `json:"type"`
	Required    bool         `json:"required"`
	Options     []string     `json:"options,omitempty"` // For choice/checkbox/dropdown
}

// ResponseParams configures response fetching
type ResponseParams struct {
	PageSize  int    `json:"page_size,omitempty"`  // Default: 50
	PageToken string `json:"page_token,omitempty"` // For pagination
}

// ExportResult holds the result of an export operation
type ExportResult struct {
	Success       bool   `json:"success"`
	OutputPath    string `json:"output_path"`
	ResponseCount int    `json:"response_count"`
	FormTitle     string `json:"form_title,omitempty"`
	Error         string `json:"error,omitempty"`
}

// ExportedData represents the full export structure written to JSON
type ExportedData struct {
	Form      FormSummary    `json:"form"`
	Responses []FormResponse `json:"responses"`
	ExportedAt time.Time     `json:"exported_at"`
}
