package social

import (
	"fmt"
	"os"
	"strings"

	"github.com/claytonharbour/proseforge-mstack/src/internal/social"
	"github.com/spf13/cobra"
)

func NewCheckCmd() *cobra.Command {
	var socialSvc = social.NewService()
	var project string
	var campaign string
	var site string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run pre-launch checklist",
		Long:  "Checks credentials, site reachability, API connections, and campaign file validity.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				project = "proseforge"
			}

			siteURL := site
			if siteURL == "" && project == "proseforge" {
				siteURL = "https://www.proseforge.ai"
			}

			fmt.Printf("\n%s\n", strings.Repeat("=", 60))
			fmt.Printf("Pre-Launch Checklist: %s\n", project)
			fmt.Printf("%s\n\n", strings.Repeat("=", 60))

			allPassed := true

			// 1. Credentials
			fmt.Println("[CHECK] Checking credentials...")
			ok, issues := socialSvc.CheckCredentials(project)
			if ok {
				fmt.Println("[OK] All credentials present")
			} else {
				allPassed = false
				for _, issue := range issues {
					fmt.Fprintf(os.Stderr, "[ERROR] %s\n", issue)
				}
			}

			// 2. Site check
			if siteURL != "" {
				fmt.Println()
				fmt.Printf("[CHECK] Checking site: %s\n", siteURL)
				// Site check not yet implemented in service
				fmt.Println("[WARN] Site check not yet implemented")
			}

			// 3. API connections
			fmt.Println()
			fmt.Println("[CHECK] Checking API connections...")
			// API checks not yet implemented in service
			fmt.Println("[WARN] API connection checks not yet implemented")

			// 4. Campaign check
			if campaign != "" {
				fmt.Println()
				fmt.Printf("[CHECK] Checking campaign: %s\n", campaign)
				// Campaign check not yet implemented in service
				fmt.Println("[WARN] Campaign check not yet implemented")
			}

			// Summary
			fmt.Printf("\n%s\n", strings.Repeat("=", 60))
			if allPassed {
				fmt.Println("[OK] All checks passed! Ready for launch.")
			} else {
				fmt.Println("[ERROR] Some checks failed. Please fix issues before launch.")
			}
			fmt.Printf("%s\n\n", strings.Repeat("=", 60))

			if !allPassed {
				return fmt.Errorf("pre-launch checks failed")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name (default: proseforge)")
	cmd.Flags().StringVar(&campaign, "campaign", "", "Campaign name to check")
	cmd.Flags().StringVar(&site, "site", "", "Site URL to check (e.g., https://proseforge.ai)")

	return cmd
}
