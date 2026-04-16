package social

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SecretGroups groups .env keys into logical Bitwarden items
var secretGroups = map[string][]string{
	"X (Twitter)": {"X_"},
	"Facebook":    {"FACEBOOK_"},
	"Instagram":   {"INSTAGRAM_"},
	"YouTube":     {"YOUTUBE_"},
	"LinkedIn":    {"LINKEDIN_"},
	"Google AI":   {"GOOGLE_AI_", "TTS_"},
	"Account":     {"ACCOUNT_"},
}

// runBW runs Bitwarden CLI command
func runBW(args []string, capture bool) (int, string, error) {
	cmd := exec.Command("bw", args...)
	if capture {
		output, err := cmd.Output()
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				return exitError.ExitCode(), string(output), nil
			}
			return 1, "", err
		}
		return 0, strings.TrimSpace(string(output)), nil
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.ExitCode(), "", nil
		}
		return 1, "", err
	}
	return 0, "", nil
}

// checkBWSession checks if Bitwarden session is active
func checkBWSession() bool {
	if os.Getenv("BW_SESSION") == "" {
		return false
	}

	code, output, err := runBW([]string{"status"}, true)
	if err != nil {
		return false
	}
	if code != 0 {
		return false
	}

	var status map[string]interface{}
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		return false
	}

	return status["status"] == "unlocked"
}

// getOrganizationID gets organization ID by name
func getOrganizationID(orgName string) (string, error) {
	code, output, err := runBW([]string{"list", "organizations"}, true)
	if err != nil {
		return "", fmt.Errorf("failed to list organizations: %w", err)
	}
	if code != 0 {
		return "", fmt.Errorf("failed to list organizations: %s", output)
	}

	var orgs []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &orgs); err != nil {
		return "", fmt.Errorf("failed to parse organizations: %w", err)
	}

	for _, org := range orgs {
		if name, ok := org["name"].(string); ok && strings.EqualFold(name, orgName) {
			if id, ok := org["id"].(string); ok {
				return id, nil
			}
		}
	}

	return "", fmt.Errorf("organization '%s' not found", orgName)
}

// getCollectionID gets collection ID by name within an organization
func getCollectionID(orgID, collectionName string) (string, error) {
	code, output, err := runBW([]string{"list", "collections", "--organizationid", orgID}, true)
	if err != nil {
		return "", fmt.Errorf("failed to list collections: %w", err)
	}
	if code != 0 {
		return "", fmt.Errorf("failed to list collections: %s", output)
	}

	var collections []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &collections); err != nil {
		return "", fmt.Errorf("failed to parse collections: %w", err)
	}

	// Try exact match first
	for _, coll := range collections {
		if name, ok := coll["name"].(string); ok && strings.EqualFold(name, collectionName) {
			if id, ok := coll["id"].(string); ok {
				return id, nil
			}
		}
	}

	// Try "API Keys" as fallback
	for _, coll := range collections {
		if name, ok := coll["name"].(string); ok {
			nameLower := strings.ToLower(name)
			if strings.Contains(nameLower, "api") || strings.Contains(nameLower, "key") {
				if id, ok := coll["id"].(string); ok {
					return id, nil
				}
			}
		}
	}

	return "", fmt.Errorf("collection '%s' not found in organization", collectionName)
}

// parseEnvFile parses .env file into map
func parseEnvFile(envPath string) (map[string]string, error) {
	secrets := make(map[string]string)

	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return secrets, nil
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if idx := strings.Index(line, "="); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			if value != "" {
				secrets[key] = value
			}
		}
	}

	return secrets, nil
}

// groupSecrets groups secrets by platform/service
func groupSecrets(secrets map[string]string) map[string]map[string]string {
	grouped := make(map[string]map[string]string)
	for name := range secretGroups {
		grouped[name] = make(map[string]string)
	}
	grouped["Other"] = make(map[string]string)

	for key, value := range secrets {
		placed := false
		for groupName, prefixes := range secretGroups {
			for _, prefix := range prefixes {
				if strings.HasPrefix(key, prefix) {
					grouped[groupName][key] = value
					placed = true
					break
				}
			}
			if placed {
				break
			}
		}
		if !placed {
			grouped["Other"][key] = value
		}
	}

	// Remove empty groups
	result := make(map[string]map[string]string)
	for k, v := range grouped {
		if len(v) > 0 {
			result[k] = v
		}
	}

	return result
}

// createBWItem creates a Bitwarden secure note with secrets as custom fields
func createBWItem(name string, secrets map[string]string, orgID, collID string) (bool, error) {
	fields := []map[string]interface{}{}
	for key, value := range secrets {
		fields = append(fields, map[string]interface{}{
			"name":  key,
			"value": value,
			"type":  1, // hidden
		})
	}

	item := map[string]interface{}{
		"organizationId": orgID,
		"collectionIds": []string{collID},
		"type":           2, // Secure note
		"name":           name,
		"notes":          fmt.Sprintf("Secrets for %s\nManaged by sync_secrets", name),
		"secureNote":     map[string]interface{}{"type": 0},
		"fields":         fields,
	}

	itemJSON, err := json.Marshal(item)
	if err != nil {
		return false, err
	}

	itemB64 := base64.StdEncoding.EncodeToString(itemJSON)

	code, output, err := runBW([]string{"create", "item", itemB64, "--organizationid", orgID}, true)
	if err != nil {
		return false, err
	}

	if code != 0 {
		if strings.Contains(strings.ToLower(output), "already exists") {
			return updateBWItem(name, secrets, orgID, collID)
		}
		return false, nil
	}

	return true, nil
}

// getBWItems gets all items in organization
func getBWItems(orgID string) ([]map[string]interface{}, error) {
	code, output, err := runBW([]string{"list", "items", "--organizationid", orgID}, true)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return []map[string]interface{}{}, nil
	}

	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &items); err != nil {
		return []map[string]interface{}{}, nil
	}

	return items, nil
}

// updateBWItem updates existing Bitwarden item
func updateBWItem(name string, secrets map[string]string, orgID, collID string) (bool, error) {
	items, err := getBWItems(orgID)
	if err != nil {
		return false, err
	}

	var itemID string
	for _, item := range items {
		if itemName, ok := item["name"].(string); ok && itemName == name {
			if id, ok := item["id"].(string); ok {
				itemID = id
				break
			}
		}
	}

	if itemID == "" {
		return createBWItem(name, secrets, orgID, collID)
	}

	// Get current item
	code, output, err := runBW([]string{"get", "item", itemID}, true)
	if err != nil {
		return false, err
	}
	if code != 0 {
		return false, fmt.Errorf("failed to get item: %s", output)
	}

	var item map[string]interface{}
	if err := json.Unmarshal([]byte(output), &item); err != nil {
		return false, err
	}

	// Update fields
	fields := []map[string]interface{}{}
	for key, value := range secrets {
		fields = append(fields, map[string]interface{}{
			"name":  key,
			"value": value,
			"type":  1,
		})
	}
	item["fields"] = fields

	itemJSON, err := json.Marshal(item)
	if err != nil {
		return false, err
	}

	itemB64 := base64.StdEncoding.EncodeToString(itemJSON)

	code, output, err = runBW([]string{"edit", "item", itemID, itemB64}, true)
	if err != nil {
		return false, err
	}

	return code == 0, nil
}

// syncSecrets handles secret synchronization
func syncSecrets(project string, action string) error {

	switch action {
	case "push":
		return pushSecrets(project)
	case "pull":
		return pullSecrets(project)
	case "list":
		return listSecrets(project)
	case "diff":
		return diffSecrets(project)
	default:
		return fmt.Errorf("unknown action: %s (must be push, pull, list, or diff)", action)
	}
}

// pushSecrets pushes .env secrets to Bitwarden
func pushSecrets(project string) error {
	envPath := filepath.Join("projects", project, ".env")

	if !checkBWSession() {
		return fmt.Errorf("BW_SESSION not set. Run: export BW_SESSION=$(bw unlock --raw)")
	}

	secrets, err := parseEnvFile(envPath)
	if err != nil {
		return fmt.Errorf(".env not found: %s", envPath)
	}

	if len(secrets) == 0 {
		return nil
	}

	grouped := groupSecrets(secrets)

	orgID, err := getOrganizationID("ProseForge")
	if err != nil {
		return err
	}

	collID, err := getCollectionID(orgID, "Secrets")
	if err != nil {
		return fmt.Errorf("no collection found. Create one in Bitwarden web vault")
	}

	for groupName, secretsDict := range grouped {
		itemName := fmt.Sprintf("%s - %s", strings.Title(project), groupName)
		_, err := createBWItem(itemName, secretsDict, orgID, collID)
		if err != nil {
			return err
		}
	}

	runBW([]string{"sync"}, false)
	return nil
}

// pullSecrets pulls secrets from Bitwarden to .env
func pullSecrets(project string) error {
	envPath := filepath.Join("projects", project, ".env")

	if !checkBWSession() {
		return fmt.Errorf("BW_SESSION not set. Run: export BW_SESSION=$(bw unlock --raw)")
	}

	orgID, err := getOrganizationID("ProseForge")
	if err != nil {
		return err
	}

	runBW([]string{"sync"}, false)

	items, err := getBWItems(orgID)
	if err != nil {
		return err
	}

	projectItems := []map[string]interface{}{}
	for _, item := range items {
		if name, ok := item["name"].(string); ok && strings.HasPrefix(strings.ToLower(name), strings.ToLower(project)) {
			projectItems = append(projectItems, item)
		}
	}

	if len(projectItems) == 0 {
		return nil
	}

	secrets := make(map[string]string)
	for _, item := range projectItems {
		if fields, ok := item["fields"].([]interface{}); ok {
			for _, fieldInterface := range fields {
				if field, ok := fieldInterface.(map[string]interface{}); ok {
					if name, ok := field["name"].(string); ok && name != "" {
						if value, ok := field["value"].(string); ok && value != "" {
							secrets[name] = value
						}
					}
				}
			}
		}
	}

	// Read existing .env to preserve structure and comments
	var lines []string
	if _, err := os.Stat(envPath); err == nil {
		data, err := os.ReadFile(envPath)
		if err == nil {
			lines = strings.Split(string(data), "\n")
		}
	}

	if len(lines) == 0 {
		// Create new .env
		keys := make([]string, 0, len(secrets))
		for k := range secrets {
			keys = append(keys, k)
		}
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("%s=%s", key, secrets[key]))
		}
	} else {
		// Update existing .env
		newLines := []string{}
		updatedKeys := make(map[string]bool)

		for _, line := range lines {
			if strings.Contains(line, "=") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
				parts := strings.SplitN(line, "=", 2)
				key := strings.TrimSpace(parts[0])
				if value, exists := secrets[key]; exists {
					newLines = append(newLines, fmt.Sprintf("%s=%s", key, value))
					updatedKeys[key] = true
				} else {
					newLines = append(newLines, line)
				}
			} else {
				newLines = append(newLines, line)
			}
		}

		// Add any new keys not in original file
		for key, value := range secrets {
			if !updatedKeys[key] {
				newLines = append(newLines, fmt.Sprintf("%s=%s", key, value))
			}
		}

		lines = newLines
	}

	return os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// listSecrets lists secrets stored in Bitwarden
func listSecrets(project string) error {
	orgID, err := getOrganizationID("ProseForge")
	if err != nil {
		return err
	}

	runBW([]string{"sync"}, false)

	items, err := getBWItems(orgID)
	if err != nil {
		return err
	}

	projectItems := []map[string]interface{}{}
	for _, item := range items {
		if name, ok := item["name"].(string); ok && strings.HasPrefix(strings.ToLower(name), strings.ToLower(project)) {
			projectItems = append(projectItems, item)
		}
	}

	if len(projectItems) == 0 {
		return nil
	}

	fmt.Printf("\nBitwarden items for '%s':\n", project)
	fmt.Println(strings.Repeat("-", 40))

	for _, item := range projectItems {
		fmt.Printf("\n%v:\n", item["name"])
		if fields, ok := item["fields"].([]interface{}); ok {
			for _, fieldInterface := range fields {
				if field, ok := fieldInterface.(map[string]interface{}); ok {
					name := ""
					value := ""
					if n, ok := field["name"].(string); ok {
						name = n
					}
					if v, ok := field["value"].(string); ok {
						value = v
					}
					if value != "" {
						masked := value
						if len(value) > 3 {
							masked = value[:3] + strings.Repeat("*", len(value)-3)
						} else {
							masked = "***"
						}
						fmt.Printf("  %s: %s\n", name, masked)
					}
				}
			}
		}
	}

	return nil
}

// diffSecrets compares .env with Bitwarden
func diffSecrets(project string) error {
	envPath := filepath.Join("projects", project, ".env")

	local, _ := parseEnvFile(envPath)

	orgID, err := getOrganizationID("ProseForge")
	if err != nil {
		return err
	}

	runBW([]string{"sync"}, false)

	items, err := getBWItems(orgID)
	if err != nil {
		return err
	}

	projectItems := []map[string]interface{}{}
	for _, item := range items {
		if name, ok := item["name"].(string); ok && strings.HasPrefix(strings.ToLower(name), strings.ToLower(project)) {
			projectItems = append(projectItems, item)
		}
	}

	remote := make(map[string]string)
	for _, item := range projectItems {
		if fields, ok := item["fields"].([]interface{}); ok {
			for _, fieldInterface := range fields {
				if field, ok := fieldInterface.(map[string]interface{}); ok {
					if name, ok := field["name"].(string); ok && name != "" {
						if value, ok := field["value"].(string); ok && value != "" {
							remote[name] = value
						}
					}
				}
			}
		}
	}

	// Compare
	localOnly := []string{}
	remoteOnly := []string{}
	different := []string{}

	for k := range local {
		if _, exists := remote[k]; !exists {
			localOnly = append(localOnly, k)
		} else if local[k] != remote[k] {
			different = append(different, k)
		}
	}

	for k := range remote {
		if _, exists := local[k]; !exists {
			remoteOnly = append(remoteOnly, k)
		}
	}

	fmt.Printf("\nComparing .env with Bitwarden for '%s':\n", project)
	fmt.Println(strings.Repeat("-", 40))

	if len(localOnly) > 0 {
		fmt.Printf("\nOnly in .env (%d):\n", len(localOnly))
		for _, k := range localOnly {
			fmt.Printf("  + %s\n", k)
		}
	}

	if len(remoteOnly) > 0 {
		fmt.Printf("\nOnly in Bitwarden (%d):\n", len(remoteOnly))
		for _, k := range remoteOnly {
			fmt.Printf("  - %s\n", k)
		}
	}

	if len(different) > 0 {
		fmt.Printf("\nDifferent values (%d):\n", len(different))
		for _, k := range different {
			fmt.Printf("  ~ %s\n", k)
		}
	}

	if len(localOnly) == 0 && len(remoteOnly) == 0 && len(different) == 0 {
		fmt.Println("Local and remote are in sync")
	}

	return nil
}
