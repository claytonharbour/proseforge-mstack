package google

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

// GetClient returns an authenticated HTTP client for Google APIs
// It loads existing token, refreshes if needed
func GetClient(ctx context.Context, authConfig AuthConfig) (*http.Client, error) {
	// Load OAuth2 config from client_secret.json
	config, err := LoadOAuthConfig(authConfig.ClientSecretPath, AllScopes())
	if err != nil {
		return nil, fmt.Errorf("failed to load client secret: %w", err)
	}

	// Update redirect URL with configured port
	config.RedirectURL = fmt.Sprintf("http://localhost:%d/callback", authConfig.CallbackPort)

	// Try to load existing token
	token, err := LoadToken(authConfig.TokenFilePath)
	if err != nil {
		return nil, fmt.Errorf("no valid token found, run 'mstack google auth' first: %w", err)
	}

	// Check if token needs refresh
	if token.Expiry.Before(time.Now()) && token.RefreshToken != "" {
		// Token expired but we have refresh token - let oauth2 handle refresh
		tokenSource := config.TokenSource(ctx, token)
		newToken, err := tokenSource.Token()
		if err != nil {
			return nil, fmt.Errorf("failed to refresh token, run 'mstack google auth': %w", err)
		}

		// Save refreshed token
		if err := SaveToken(authConfig.TokenFilePath, newToken); err != nil {
			// Log but don't fail - we still have a valid token
			fmt.Fprintf(os.Stderr, "Warning: failed to save refreshed token: %v\n", err)
		}

		return config.Client(ctx, newToken), nil
	}

	return config.Client(ctx, token), nil
}

// GetClientForProject is a convenience function that creates an auth config for a project
// and returns an authenticated HTTP client
func GetClientForProject(ctx context.Context, project string) (*http.Client, error) {
	authConfig := AuthConfigForProject(project)
	return GetClient(ctx, authConfig)
}

// MustGetClient returns an authenticated HTTP client or panics
// Use this in contexts where authentication is required and failure is fatal
func MustGetClient(ctx context.Context, authConfig AuthConfig) *http.Client {
	client, err := GetClient(ctx, authConfig)
	if err != nil {
		panic(fmt.Sprintf("failed to get Google client: %v", err))
	}
	return client
}
