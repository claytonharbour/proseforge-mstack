package audio

import (
	"github.com/spf13/cobra"
)

// NewAudioCmd creates the "audio" parent command.
func NewAudioCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audio",
		Short: "Audio narration tools",
		Long:  "Tools for generating long-form audiobook-style narration from markdown.",
	}

	cmd.AddCommand(NewNarrateCmd())

	return cmd
}
