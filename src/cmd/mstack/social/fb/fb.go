package fb

import (
	"github.com/spf13/cobra"
)

func NewFBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fb",
		Short: "Facebook Page operations",
		Long:  "Manage Facebook Page and posts.",
	}

	cmd.AddCommand(NewShowCmd())
	cmd.AddCommand(NewPostCmd())
	cmd.AddCommand(NewDeleteCmd())
	cmd.AddCommand(NewUpdateCmd())

	return cmd
}
