package tts

import (
	"github.com/spf13/cobra"
)

func NewTTSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tts",
		Short: "Generate TTS audio",
		Long:  "Generate text-to-speech audio using various engines.",
	}

	cmd.AddCommand(NewSayCmd())
	cmd.AddCommand(NewGeminiCmd())
	cmd.AddCommand(NewCloudTTSCmd())
	cmd.AddCommand(NewVertexCmd())

	return cmd
}
