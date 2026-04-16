package google

import (
	"github.com/spf13/cobra"
)

func NewGoogleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "google",
		Short: "Google API authentication",
		Long:  "Manage Google OAuth authentication for Forms, YouTube, and Drive services.",
	}

	cmd.AddCommand(NewAuthCmd())

	return cmd
}
