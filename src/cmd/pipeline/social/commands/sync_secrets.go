package commands

import (
	"fmt"
	"os"

	"github.com/claytonharbour/proseforge-mstack/src/internal/social"
	"github.com/spf13/cobra"
)

func NewSyncSecretsCmd() *cobra.Command {
	var socialSvc = social.NewService()
	var project string
	var push bool
	var pull bool
	var list bool
	var diff bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "sync-secrets",
		Short: "Synchronize secrets with Bitwarden",
		Long:  "Push .env to Bitwarden, pull from Bitwarden to .env, list secrets, or diff local vs remote.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			actionCount := 0
			if push {
				actionCount++
			}
			if pull {
				actionCount++
			}
			if list {
				actionCount++
			}
			if diff {
				actionCount++
			}

			if actionCount != 1 {
				return fmt.Errorf("exactly one action must be specified: --push, --pull, --list, or --diff")
			}

			var action string
			if push {
				action = "push"
			} else if pull {
				action = "pull"
			} else if list {
				action = "list"
			} else if diff {
				action = "diff"
			}

			if dryRun && (action == "push" || action == "pull") {
				fmt.Fprintf(os.Stderr, "Dry run mode\n")
			}

			err := socialSvc.SyncSecrets(project, action)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")
	cmd.Flags().BoolVar(&push, "push", false, "Push .env to Bitwarden")
	cmd.Flags().BoolVar(&pull, "pull", false, "Pull from Bitwarden to .env")
	cmd.Flags().BoolVar(&list, "list", false, "List Bitwarden items")
	cmd.Flags().BoolVar(&diff, "diff", false, "Compare .env vs Bitwarden")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without changes")

	return cmd
}
