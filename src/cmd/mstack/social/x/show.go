package x

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/social"
	"github.com/spf13/cobra"
)

func NewShowCmd() *cobra.Command {
	var socialSvc = social.NewService()
	var project string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show X profile",
		Long:  "Display current X (Twitter) profile information.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			profile, err := socialSvc.ShowXProfile(project)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(profile)
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")

	return cmd
}
