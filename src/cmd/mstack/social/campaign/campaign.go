package campaign

import (
	"github.com/spf13/cobra"
)

func NewCampaignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "campaign",
		Short: "Campaign management",
		Long:  "Manage social media campaigns - list, post, and retract campaign items.",
	}

	cmd.AddCommand(NewListCmd())
	cmd.AddCommand(NewPostCmd())
	cmd.AddCommand(NewRetractCmd())

	return cmd
}
