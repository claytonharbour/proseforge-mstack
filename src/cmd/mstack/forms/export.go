package forms

import (
	"context"
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/forms"
	"github.com/spf13/cobra"
)

func NewExportCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "export [form-id]",
		Short: "Export responses to JSON file",
		Long: `Export all form responses to a JSON file.

Default output: projects/<project>/forms/<form-title>/responses.json

Examples:
  # Export to default location
  mstack forms export 1FAIpQLSf...

  # Export to custom path
  mstack forms export 1FAIpQLSf... --output=/path/to/responses.json

  # Export for specific project
  mstack forms export 1FAIpQLSf... --project=proseforge`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			formID := args[0]

			ctx := context.Background()
			svc := forms.NewService()

			result, err := svc.ExportResponses(ctx, project, formID, outputPath)
			if err != nil {
				return fmt.Errorf("failed to export responses: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Exported %d responses to: %s\n", result.ResponseCount, result.OutputPath)

			return printJSON(result)
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path")

	return cmd
}
