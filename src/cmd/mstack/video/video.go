package video

import (
	"github.com/claytonharbour/proseforge-mstack/src/cmd/mstack/video/tts"
	"github.com/spf13/cobra"
)

func NewVideoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "video",
		Short: "Video narration pipeline tools",
		Long:  "Tools for processing video narration, generating TTS audio, and building final videos.",
	}

	cmd.AddCommand(NewParseCmd())
	cmd.AddCommand(NewAnalyzeCmd())
	cmd.AddCommand(NewBuildCmd())
	cmd.AddCommand(tts.NewTTSCmd())
	cmd.AddCommand(NewProcessCmd())
	cmd.AddCommand(NewProcessAllCmd())
	cmd.AddCommand(NewValidateCmd())
	cmd.AddCommand(NewExtractFrameCmd())
	cmd.AddCommand(NewYouTubeCmd())
	cmd.AddCommand(NewEstimateCmd())
	cmd.AddCommand(NewThumbnailCmd())
	cmd.AddCommand(NewSplitCmd())
	cmd.AddCommand(NewJoinCmd())
	cmd.AddCommand(NewTrimCmd())
	cmd.AddCommand(NewCheckCmd())

	return cmd
}
