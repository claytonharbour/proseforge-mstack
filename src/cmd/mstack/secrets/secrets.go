package secrets

import (
	"github.com/spf13/cobra"
)

func NewSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Secret management",
		Long:  "Synchronize secrets between .env and Bitwarden.",
	}

	cmd.AddCommand(NewPushCmd())
	cmd.AddCommand(NewPullCmd())
	cmd.AddCommand(NewListCmd())
	cmd.AddCommand(NewDiffCmd())

	return cmd
}
