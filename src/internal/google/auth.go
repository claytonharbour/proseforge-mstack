package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// RunAuthFlow runs the full OAuth2 authentication flow for all Google services
// This opens a browser for user consent and captures the callback
func RunAuthFlow(ctx context.Context, clientSecretPath string, project string, callbackPort int) (*AuthConfig, error) {
	// Load OAuth2 config with all scopes
	config, err := LoadOAuthConfig(clientSecretPath, AllScopes())
	if err != nil {
		return nil, fmt.Errorf("failed to load client secret: %w", err)
	}

	// Set redirect URL with configured port
	config.RedirectURL = fmt.Sprintf("http://localhost:%d/callback", callbackPort)

	// Channel to receive auth code
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Start local server to receive callback
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", callbackPort),
		Handler: mux,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code in callback")
			fmt.Fprintf(w, "Error: No authorization code received")
			return
		}

		codeChan <- code
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Authorization Successful</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1>Authorization Successful!</h1>
<p>You can close this window and return to the terminal.</p>
</body>
</html>`)
	})

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Generate auth URL with offline access and force approval
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	fmt.Printf("\nOpening browser for Google authorization...\n")
	fmt.Printf("This will grant access to: %s\n\n", strings.Join(ServiceNames(), ", "))
	fmt.Printf("If browser doesn't open, visit this URL:\n\n%s\n\n", authURL)

	// Try to open browser
	openBrowser(authURL)

	// Wait for code or error
	var code string
	select {
	case code = <-codeChan:
		// Got the code
	case err := <-errChan:
		server.Shutdown(ctx)
		return nil, err
	case <-time.After(5 * time.Minute):
		server.Shutdown(ctx)
		return nil, fmt.Errorf("timeout waiting for authorization")
	}

	// Shutdown server
	server.Shutdown(ctx)

	// Exchange code for token
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	// Create google directory for the project
	googleDir := GetGoogleDir(project)
	if err := os.MkdirAll(googleDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create google directory: %w", err)
	}

	// Build auth config for this project
	authConfig := AuthConfigForProject(project)
	authConfig.CallbackPort = callbackPort

	// Copy client_secret.json to project directory if not already there
	if clientSecretPath != authConfig.ClientSecretPath {
		secretData, err := os.ReadFile(clientSecretPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read client secret: %w", err)
		}
		if err := os.WriteFile(authConfig.ClientSecretPath, secretData, 0600); err != nil {
			return nil, fmt.Errorf("failed to copy client secret: %w", err)
		}
	}

	// Save token
	if err := SaveToken(authConfig.TokenFilePath, token); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Printf("\nAuthentication successful!\n")
	fmt.Printf("Credentials saved to: %s\n", googleDir)

	return &authConfig, nil
}

// CheckToken checks if a valid token exists and reports its status
func CheckToken(authConfig AuthConfig) *AuthStatus {
	status := &AuthStatus{
		Project:       authConfig.Project,
		Authenticated: false,
	}

	token, err := LoadToken(authConfig.TokenFilePath)
	if err != nil {
		status.Status = fmt.Sprintf("No token found: %v", err)
		return status
	}

	if token.Expiry.Before(time.Now()) {
		if token.RefreshToken != "" {
			status.Authenticated = true
			status.Status = "Token expired but has refresh token (will auto-refresh)"
			status.TokenExpiry = token.Expiry.Format(time.RFC3339)
		} else {
			status.Status = "Token expired and no refresh token available"
		}
		return status
	}

	status.Authenticated = true
	status.Status = "Token valid"
	status.TokenExpiry = token.Expiry.Format(time.RFC3339)
	return status
}

// LoadOAuthConfig loads OAuth2 configuration from client_secret.json with specified scopes
func LoadOAuthConfig(clientSecretPath string, scopes []string) (*oauth2.Config, error) {
	data, err := os.ReadFile(clientSecretPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", clientSecretPath, err)
	}

	config, err := google.ConfigFromJSON(data, scopes...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse client secret: %w", err)
	}

	return config, nil
}

// LoadToken loads OAuth2 token from file
func LoadToken(tokenPath string) (*oauth2.Token, error) {
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	return &token, nil
}

// SaveToken saves OAuth2 token to file
func SaveToken(tokenPath string, token *oauth2.Token) error {
	// Ensure directory exists
	dir := filepath.Dir(tokenPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	if err := os.WriteFile(tokenPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write token: %w", err)
	}

	return nil
}

// openBrowser opens URL in default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}
