package main

import (
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/cmd/mstack/audio"
	"github.com/claytonharbour/proseforge-mstack/src/cmd/mstack/forms"
	"github.com/claytonharbour/proseforge-mstack/src/cmd/mstack/google"
	"github.com/claytonharbour/proseforge-mstack/src/cmd/mstack/secrets"
	"github.com/claytonharbour/proseforge-mstack/src/cmd/mstack/social"
	"github.com/claytonharbour/proseforge-mstack/src/cmd/mstack/video"
	"github.com/claytonharbour/proseforge-mstack/src/internal/config"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	config.Load()

	rootCmd := &cobra.Command{
		Use:   "mstack",
		Short: "Marketing stack tools",
		Long:  "Marketing stack tools for video narration, social media management, and secrets.",
	}

	// Add global flags
	rootCmd.PersistentFlags().String("project", "proseforge", "Project name")

	// Add top-level subcommands
	rootCmd.AddCommand(audio.NewAudioCmd())
	rootCmd.AddCommand(video.NewVideoCmd())
	rootCmd.AddCommand(social.NewSocialCmd())
	rootCmd.AddCommand(secrets.NewSecretsCmd())
	rootCmd.AddCommand(google.NewGoogleCmd())
	rootCmd.AddCommand(forms.NewFormsCmd())
	rootCmd.AddCommand(newVersionCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version info",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("mstack version", version)
		},
	}
}
