package x

import (
	"github.com/spf13/cobra"
)

func NewXCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "x",
		Short: "X (Twitter) operations",
		Long:  "Manage X (Twitter) profile and posts.",
	}

	cmd.AddCommand(NewShowCmd())
	cmd.AddCommand(NewPostCmd())
	cmd.AddCommand(NewDeleteCmd())
	cmd.AddCommand(NewUpdateCmd())

	return cmd
}
