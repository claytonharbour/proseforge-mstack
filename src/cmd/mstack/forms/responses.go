package forms

import (
	"context"
	"fmt"

	"github.com/claytonharbour/proseforge-mstack/src/internal/forms"
	"github.com/spf13/cobra"
)

func NewResponsesCmd() *cobra.Command {
	var (
		pageSize  int
		pageToken string
	)

	cmd := &cobra.Command{
		Use:   "responses [form-id]",
		Short: "Fetch form responses",
		Long: `Fetch responses submitted to a Google Form.

Examples:
  # Get all responses
  mstack forms responses 1FAIpQLSf...

  # Limit results
  mstack forms responses 1FAIpQLSf... --page-size=10

  # Paginate through results
  mstack forms responses 1FAIpQLSf... --page-token=xxx`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			formID := args[0]

			ctx := context.Background()
			svc := forms.NewService()

			params := forms.ResponseParams{
				PageSize:  pageSize,
				PageToken: pageToken,
			}

			responses, err := svc.GetResponses(ctx, project, formID, params)
			if err != nil {
				return fmt.Errorf("failed to get responses: %w", err)
			}

			return printJSON(responses)
		},
	}

	cmd.Flags().IntVar(&pageSize, "page-size", 50, "Number of responses per page (max: 5000)")
	cmd.Flags().StringVar(&pageToken, "page-token", "", "Page token for pagination")

	return cmd
}
