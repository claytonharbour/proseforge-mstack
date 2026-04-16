package ig

import (
	"github.com/spf13/cobra"
)

func NewIGCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ig",
		Short: "Instagram operations",
		Long:  "Manage Instagram profile and posts.",
	}

	cmd.AddCommand(NewShowCmd())
	cmd.AddCommand(NewPostCmd())
	cmd.AddCommand(NewDeleteCmd())

	return cmd
}
