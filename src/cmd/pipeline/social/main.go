package main

import (
	"github.com/claytonharbour/proseforge-mstack/src/cmd/pipeline/social/commands"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mstack-social",
		Short: "Social media management tools",
		Long:  "Tools for managing social media profiles, posting content, and running campaigns.",
	}

	rootCmd.AddCommand(commands.NewXProfileCmd())
	rootCmd.AddCommand(commands.NewFacebookPageCmd())
	rootCmd.AddCommand(commands.NewInstagramProfileCmd())
	rootCmd.AddCommand(commands.NewCampaignPostCmd())
	rootCmd.AddCommand(commands.NewSyncSecretsCmd())
	rootCmd.AddCommand(commands.NewPrelaunchCheckCmd())

	if err := rootCmd.Execute(); err != nil {
		// Error already printed by cobra
	}
}
