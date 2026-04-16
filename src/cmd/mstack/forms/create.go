package forms

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/forms"
	"github.com/spf13/cobra"
)

func NewCreateCmd() *cobra.Command {
	var (
		title       string
		description string
		itemsFile   string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Google Form",
		Long: `Create a new Google Form programmatically.

Examples:
  # Create basic form
  mstack forms create --title="Beta Signup"

  # Create with description
  mstack forms create --title="Beta Signup" --description="Sign up for ProseForge beta"

  # Create from JSON spec file
  mstack forms create --items=beta-form.json

The items file should be JSON with this structure:
{
  "title": "Form Title",
  "description": "Form description",
  "items": [
    {"title": "Name", "type": "text", "required": true},
    {"title": "Email", "type": "text", "required": true},
    {"title": "Feedback", "type": "paragraph", "required": false},
    {"title": "Rating", "type": "choice", "required": true,
     "options": ["Excellent", "Good", "Fair", "Poor"]}
  ]
}

Supported item types: text, paragraph, choice, checkbox, dropdown, scale, date, time`,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")

			var params forms.CreateFormParams

			// If items file provided, load from file
			if itemsFile != "" {
				data, err := os.ReadFile(itemsFile)
				if err != nil {
					return fmt.Errorf("failed to read items file: %w", err)
				}

				if err := json.Unmarshal(data, &params); err != nil {
					return fmt.Errorf("failed to parse items file: %w", err)
				}

				// Override with flags if provided
				if title != "" {
					params.Title = title
				}
				if description != "" {
					params.Description = description
				}
			} else {
				// Build from flags
				if title == "" {
					return fmt.Errorf("--title is required (or provide --items file)")
				}
				params.Title = title
				params.Description = description
			}

			if params.Title == "" {
				return fmt.Errorf("form title is required")
			}

			ctx := context.Background()
			svc := forms.NewService()

			form, err := svc.CreateForm(ctx, project, params)
			if err != nil {
				return fmt.Errorf("failed to create form: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Form created successfully!\n")
			fmt.Fprintf(os.Stderr, "Responder URL: %s\n", form.ResponderURL)

			return printJSON(form)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Form title (required unless --items provided)")
	cmd.Flags().StringVar(&description, "description", "", "Form description")
	cmd.Flags().StringVar(&itemsFile, "items", "", "JSON file with form items/questions")

	return cmd
}
