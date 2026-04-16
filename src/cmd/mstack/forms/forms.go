package forms

import (
	"github.com/spf13/cobra"
)

func NewFormsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forms",
		Short: "Google Forms management",
		Long:  "Create, manage, and export responses from Google Forms.",
	}

	cmd.PersistentFlags().String("project", "proseforge", "Project name")

	cmd.AddCommand(NewListCmd())
	cmd.AddCommand(NewGetCmd())
	cmd.AddCommand(NewCreateCmd())
	cmd.AddCommand(NewResponsesCmd())
	cmd.AddCommand(NewExportCmd())

	return cmd
}
