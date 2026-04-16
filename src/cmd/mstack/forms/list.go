package forms

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/claytonharbour/proseforge-mstack/src/internal/forms"
	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List accessible Google Forms",
		Long: `List all Google Forms accessible by the authenticated account.

Uses Google Drive API to find forms owned by or shared with the user.

Examples:
  # List all forms
  mstack forms list

  # List forms for a specific project
  mstack forms list --project=proseforge`,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")

			ctx := context.Background()
			svc := forms.NewService()

			formsList, err := svc.ListForms(ctx, project)
			if err != nil {
				return fmt.Errorf("failed to list forms: %w", err)
			}

			return printJSON(formsList)
		},
	}

	return cmd
}

func printJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
