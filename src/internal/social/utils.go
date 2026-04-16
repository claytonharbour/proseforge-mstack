package social

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// projectsRoot is the base path for project directories
// If empty, falls back to relative "projects/" path (legacy behavior)
var projectsRoot string

// SetProjectsRoot sets the base path for project directories
// Called from MCP server with MSTACK_PROJECTS_ROOT env var
func SetProjectsRoot(root string) {
	projectsRoot = root
}

// getProjectPath returns the full path to a project directory
func getProjectPath(project string) string {
	if projectsRoot != "" {
		return filepath.Join(projectsRoot, project)
	}
	// Legacy: relative path from working directory
	return filepath.Join("projects", project)
}

// log prints a formatted log message
func log(msg string, level string) {
	prefix := map[string]string{
		"info":  "\033[34m[INFO]\033[0m",
		"ok":    "\033[32m[OK]\033[0m",
		"warn":  "\033[33m[WARN]\033[0m",
		"error": "\033[31m[ERROR]\033[0m",
		"check": "\033[36m[CHECK]\033[0m",
	}[level]
	fmt.Printf("%s %s\n", prefix, msg)
}

// loadEnv loads environment variables from project .env file
func loadEnv(project string) map[string]string {
	envPath := filepath.Join(getProjectPath(project), ".env")
	envVars := make(map[string]string)

	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return envVars
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		return envVars
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, "="); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			envVars[key] = value
		}
	}

	return envVars
}

// updateEnvValue updates a single value in an .env file
func updateEnvValue(envPath, key, value string) error {
	data, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+"=") {
			lines[i] = fmt.Sprintf("%s=%s", key, value)
			found = true
			break
		}
	}

	if !found {
		// Add new key at the end
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	return os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0644)
}
