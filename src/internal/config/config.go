package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Load reads ~/.mstack/conf and sets environment variables for each KEY=VALUE
// line where os.Getenv(key) is empty. It skips comments (#) and blank lines.
// It silently returns if the file is missing.
func Load() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	f, err := os.Open(filepath.Join(home, ".mstack", "conf"))
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}
