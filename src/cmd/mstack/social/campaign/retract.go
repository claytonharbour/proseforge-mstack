package campaign

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewRetractCmd() *cobra.Command {
	var project string
	var campaignName string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "retract [uuid]",
		Short: "Retract a campaign item",
		Long:  "Delete a posted campaign item from all platforms.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}
			if campaignName == "" {
				campaignName = "launch"
			}

			postID := args[0]

			if dryRun {
				fmt.Fprintf(os.Stderr, "Dry run - no posts retracted\n")
				fmt.Fprintf(os.Stderr, "Would retract campaign item: %s\n", postID)
				return nil
			}

			// Retract not yet implemented in service
			result := map[string]string{"status": "retract not yet implemented", "post_id": postID}
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")
	cmd.Flags().StringVar(&campaignName, "campaign", "launch", "Campaign name (default: launch)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without retracting")

	return cmd
}
