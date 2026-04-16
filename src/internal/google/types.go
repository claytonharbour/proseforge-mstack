package google

import (
	"os"
	"path/filepath"
)

// OAuth2 scopes for all Google services
const (
	// Forms scopes
	FormsBodyScope              = "https://www.googleapis.com/auth/forms.body"
	FormsResponsesReadonlyScope = "https://www.googleapis.com/auth/forms.responses.readonly"

	// YouTube scopes
	YouTubeUploadScope = "https://www.googleapis.com/auth/youtube.upload"
	YouTubeScope       = "https://www.googleapis.com/auth/youtube"

	// Drive scope (needed to list forms)
	DriveReadonlyScope = "https://www.googleapis.com/auth/drive.readonly"

	// Cloud Platform scope (needed for Cloud TTS and Vertex AI)
	CloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"
)

// ScopeEntry pairs an OAuth scope URL with a human-readable service name.
type ScopeEntry struct {
	Scope   string
	Service string // human-readable, e.g. "Forms", "YouTube"
}

// AllScopeEntries is the single source of truth for OAuth scopes and their display names.
var AllScopeEntries = []ScopeEntry{
	{FormsBodyScope, "Forms"},
	{FormsResponsesReadonlyScope, "Forms (responses)"},
	{YouTubeUploadScope, "YouTube"},
	{YouTubeScope, "YouTube"},
	{DriveReadonlyScope, "Drive (read-only)"},
	{CloudPlatformScope, "Cloud TTS / Vertex AI"},
}

// AllScopes returns all OAuth scope URLs needed for Google services.
func AllScopes() []string {
	scopes := make([]string, len(AllScopeEntries))
	for i, e := range AllScopeEntries {
		scopes[i] = e.Scope
	}
	return scopes
}

// ServiceNames returns deduplicated human-readable service names.
func ServiceNames() []string {
	seen := make(map[string]bool)
	var names []string
	for _, e := range AllScopeEntries {
		if !seen[e.Service] {
			seen[e.Service] = true
			names = append(names, e.Service)
		}
	}
	return names
}

// AuthConfig holds paths for OAuth credentials
type AuthConfig struct {
	Project          string // Project name (e.g., "proseforge")
	ClientSecretPath string // Path to client_secret.json
	TokenFilePath    string // Path to token.json
	CallbackPort     int    // OAuth callback port (default 8080)
}

// AuthStatus represents the current authentication status
type AuthStatus struct {
	Authenticated bool   `json:"authenticated"`
	Status        string `json:"status"`
	TokenExpiry   string `json:"token_expiry,omitempty"`
	Project       string `json:"project"`
}

// GetMstackDir returns the path to ~/.mstack/.
// Honors MSTACK_CONFIG_DIR if set (useful for testing).
func GetMstackDir() string {
	if dir := os.Getenv("MSTACK_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".mstack"
	}
	return filepath.Join(home, ".mstack")
}

// GetProjectsDir returns the path to ~/.mstack/projects/
func GetProjectsDir() string {
	return filepath.Join(GetMstackDir(), "projects")
}

// GetProjectDir returns the path to ~/.mstack/projects/<project>/
func GetProjectDir(project string) string {
	return filepath.Join(GetProjectsDir(), project)
}

// GetGoogleDir returns the path to ~/.mstack/projects/<project>/google/
func GetGoogleDir(project string) string {
	return filepath.Join(GetProjectDir(project), "google")
}

// GetYouTubeDir returns the path to ~/.mstack/projects/<project>/google/youtube/
func GetYouTubeDir(project string) string {
	return filepath.Join(GetGoogleDir(project), "youtube")
}

// AuthConfigForProject returns auth configuration for a specific project
func AuthConfigForProject(project string) AuthConfig {
	googleDir := GetGoogleDir(project)
	return AuthConfig{
		Project:          project,
		ClientSecretPath: filepath.Join(googleDir, "client_secret.json"),
		TokenFilePath:    filepath.Join(googleDir, "token.json"),
		CallbackPort:     8080,
	}
}

// ListAuthenticatedProjects returns all projects with valid Google auth
func ListAuthenticatedProjects() ([]string, error) {
	projectsDir := GetProjectsDir()
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var projects []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Verify it has a token file
			tokenPath := filepath.Join(projectsDir, entry.Name(), "google", "token.json")
			if _, err := os.Stat(tokenPath); err == nil {
				projects = append(projects, entry.Name())
			}
		}
	}
	return projects, nil
}

// ResolveAuthConfig resolves which project to use based on flags/env/available projects
func ResolveAuthConfig(projectFlag string) (*AuthConfig, error) {
	// 1. Check explicit project flag
	if projectFlag != "" {
		config := AuthConfigForProject(projectFlag)
		return &config, nil
	}

	// 2. Check environment variable
	if envProject := os.Getenv("MSTACK_PROJECT"); envProject != "" {
		config := AuthConfigForProject(envProject)
		return &config, nil
	}

	// 3. Default to "proseforge"
	config := AuthConfigForProject("proseforge")
	return &config, nil
}
