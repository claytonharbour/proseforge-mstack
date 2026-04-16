package social

import (
	"github.com/claytonharbour/proseforge-mstack/src/cmd/mstack/social/campaign"
	"github.com/claytonharbour/proseforge-mstack/src/cmd/mstack/social/fb"
	"github.com/claytonharbour/proseforge-mstack/src/cmd/mstack/social/ig"
	"github.com/claytonharbour/proseforge-mstack/src/cmd/mstack/social/x"
	"github.com/spf13/cobra"
)

func NewSocialCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "social",
		Short: "Social media management tools",
		Long:  "Tools for managing social media profiles, posting content, and running campaigns.",
	}

	cmd.AddCommand(x.NewXCmd())
	cmd.AddCommand(fb.NewFBCmd())
	cmd.AddCommand(ig.NewIGCmd())
	cmd.AddCommand(campaign.NewCampaignCmd())
	cmd.AddCommand(NewCheckCmd())

	return cmd
}
