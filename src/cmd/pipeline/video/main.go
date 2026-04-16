package main

import (
	"github.com/claytonharbour/proseforge-mstack/src/cmd/pipeline/video/commands"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mstack-video",
		Short: "Video narration pipeline tools",
		Long:  "Tools for processing video narration, generating TTS audio, and building final videos.",
	}

	rootCmd.AddCommand(commands.NewParseCmd())
	rootCmd.AddCommand(commands.NewAnalyzeCmd())
	rootCmd.AddCommand(commands.NewBuildCmd())
	rootCmd.AddCommand(commands.NewGenerateCmd())
	rootCmd.AddCommand(commands.NewGenerateSayCmd())
	rootCmd.AddCommand(commands.NewProcessCmd())
	rootCmd.AddCommand(commands.NewProcessAllCmd())

	if err := rootCmd.Execute(); err != nil {
		// Error already printed by cobra
	}
}
