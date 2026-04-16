package google

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/claytonharbour/proseforge-mstack/src/internal/google"
	"github.com/spf13/cobra"
)

func NewAuthCmd() *cobra.Command {
	var (
		port         int
		clientSecret string
		check        bool
		project      string
	)

	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with Google APIs",
		Long: `Run OAuth flow to authenticate with all Google services.

Grants access to: ` + strings.Join(google.ServiceNames(), ", ") + `.

Credentials are stored in ~/.mstack/projects/<project>/google/

Examples:
  # Run OAuth flow (opens browser)
  mstack google auth --client-secret=/path/to/client_secret.json

  # Authenticate for a specific project
  mstack google auth --client-secret=/path/to/client_secret.json --project=proseforge

  # Use custom port (avoid 9090 if Prometheus is running)
  mstack google auth --client-secret=... --port=9000

  # Check token status without re-authenticating
  mstack google auth --check`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if check {
				// Check existing token
				authConfig, err := google.ResolveAuthConfig(project)
				if err != nil {
					return fmt.Errorf("failed to resolve credentials: %w", err)
				}

				status := google.CheckToken(*authConfig)

				result := map[string]interface{}{
					"authenticated": status.Authenticated,
					"status":        status.Status,
					"project":       status.Project,
					"paths": map[string]string{
						"client_secret": authConfig.ClientSecretPath,
						"token_file":    authConfig.TokenFilePath,
					},
				}
				if status.TokenExpiry != "" {
					result["token_expiry"] = status.TokenExpiry
				}

				return printJSON(result)
			}

			// Run OAuth flow
			clientSecret = expandHome(clientSecret)
			if clientSecret == "" {
				return fmt.Errorf("--client-secret is required for authentication\n\nTo get credentials:\n1. Go to https://console.cloud.google.com/apis/credentials\n2. Create OAuth 2.0 credentials (Desktop app)\n3. Download client_secret.json\n4. Enable APIs: Forms, YouTube, Drive")
			}

			// Use default project if not specified
			if project == "" {
				project = "proseforge"
			}

			ctx := context.Background()
			authConfig, err := google.RunAuthFlow(ctx, clientSecret, project, port)
			if err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}

			services := google.ServiceNames()
			fmt.Fprintf(os.Stderr, "\nGoogle authentication successful for project: %s\n", project)
			fmt.Fprintf(os.Stderr, "Services enabled: %s\n", strings.Join(services, ", "))

			return printJSON(map[string]interface{}{
				"success": true,
				"project": project,
				"paths": map[string]string{
					"client_secret": authConfig.ClientSecretPath,
					"token_file":    authConfig.TokenFilePath,
				},
				"services": services,
			})
		},
	}

	cmd.Flags().IntVar(&port, "port", 8080, "OAuth callback port")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "Path to client_secret.json (required for new auth)")
	cmd.Flags().BoolVar(&check, "check", false, "Check token status without re-authenticating")
	cmd.Flags().StringVar(&project, "project", "proseforge", "Project name")

	return cmd
}

// expandHome replaces a leading "~/" with the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func printJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
