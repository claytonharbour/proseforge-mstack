package forms

import (
	"context"

	"github.com/claytonharbour/proseforge-mstack/src/internal/google"
)

// Service provides Google Forms operations
type Service interface {
	// CheckAuth checks if authenticated and returns status
	CheckAuth(ctx context.Context, project string) (*google.AuthStatus, error)

	// CreateForm creates a new form
	CreateForm(ctx context.Context, project string, params CreateFormParams) (*Form, error)

	// ListForms returns all forms accessible to the authenticated account
	// Note: Uses Drive API since Forms API has no list endpoint
	ListForms(ctx context.Context, project string) ([]FormSummary, error)

	// GetForm returns detailed form information
	GetForm(ctx context.Context, project string, formID string) (*Form, error)

	// GetResponses fetches responses for a form
	GetResponses(ctx context.Context, project string, formID string, params ResponseParams) (*ResponseList, error)

	// ExportResponses exports responses to a JSON file
	ExportResponses(ctx context.Context, project string, formID string, outputPath string) (*ExportResult, error)
}

type service struct{}

// NewService creates a new Forms service
func NewService() Service {
	return &service{}
}

// CheckAuth checks if authenticated and returns status
func (s *service) CheckAuth(ctx context.Context, project string) (*google.AuthStatus, error) {
	authConfig := google.AuthConfigForProject(project)
	status := google.CheckToken(authConfig)
	return status, nil
}

// CreateForm creates a new form
func (s *service) CreateForm(ctx context.Context, project string, params CreateFormParams) (*Form, error) {
	return createForm(ctx, project, params)
}

// ListForms returns all forms accessible to the authenticated account
func (s *service) ListForms(ctx context.Context, project string) ([]FormSummary, error) {
	return listForms(ctx, project)
}

// GetForm returns detailed form information
func (s *service) GetForm(ctx context.Context, project string, formID string) (*Form, error) {
	return getForm(ctx, project, formID)
}

// GetResponses fetches responses for a form
func (s *service) GetResponses(ctx context.Context, project string, formID string, params ResponseParams) (*ResponseList, error) {
	return getResponses(ctx, project, formID, params)
}

// ExportResponses exports responses to a JSON file
func (s *service) ExportResponses(ctx context.Context, project string, formID string, outputPath string) (*ExportResult, error) {
	return exportResponses(ctx, project, formID, outputPath)
}
