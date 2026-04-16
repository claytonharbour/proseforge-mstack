package forms

import (
	"context"
	"fmt"

	"github.com/claytonharbour/proseforge-mstack/src/internal/forms"
	"github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [form-id]",
		Short: "Get form details",
		Long: `Get detailed information about a Google Form including all questions.

Examples:
  # Get form by ID
  mstack forms get 1FAIpQLSf...

  # Get form with explicit project
  mstack forms get 1FAIpQLSf... --project=proseforge`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			formID := args[0]

			ctx := context.Background()
			svc := forms.NewService()

			form, err := svc.GetForm(ctx, project, formID)
			if err != nil {
				return fmt.Errorf("failed to get form: %w", err)
			}

			return printJSON(form)
		},
	}

	return cmd
}
