package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
	"github.com/spf13/cobra"
)

func NewParseCmd() *cobra.Command {
	var videoSvc = video.NewService()

	return &cobra.Command{
		Use:   "parse [narration.md]",
		Short: "Parse narration markdown into JSON segments",
		Long:  "Parses a markdown narration file with timestamped segments into JSON format.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			segments, err := videoSvc.ParseNarrationMD(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			// Output JSON to stdout (pipeable)
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(segments)
		},
	}
}
