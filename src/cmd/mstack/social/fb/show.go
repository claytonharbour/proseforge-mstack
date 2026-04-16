package fb

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
		Short: "Show Facebook Page info",
		Long:  "Display current Facebook Page information.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			pageInfo, err := socialSvc.ShowFacebookPage(project)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(pageInfo)
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")

	return cmd
}
