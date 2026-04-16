package functional

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var mcpBin string

func init() {
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "..", "..", "..")
	mcpBin = filepath.Join(projectRoot, "bin", "mstack-mcp")
}

func ensureMCPBuilt(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(mcpBin); os.IsNotExist(err) {
		t.Log("Building mstack-mcp binary...")
		cmd := exec.Command("make", "build-mcp")
		cmd.Dir = filepath.Join(mcpBin, "..", "..")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Failed to build mstack-mcp: %v\n%s", err, out)
		}
	}
}

// mcpRequest sends a JSON-RPC request to the MCP server and returns the response
func mcpRequest(t *testing.T, request string) map[string]interface{} {
	t.Helper()
	ensureMCPBuilt(t)

	cmd := exec.Command(mcpBin)
	cmd.Stdin = strings.NewReader(request)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("MCP request failed: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(out, &response); err != nil {
		t.Fatalf("Failed to parse MCP response: %v\nOutput: %s", err, out)
	}

	return response
}

func TestMCPToolsList(t *testing.T) {
	request := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	response := mcpRequest(t, request)

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result in response, got: %v", response)
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("Expected tools array in result, got: %v", result)
	}

	// Check that expected tools exist
	expectedTools := []string{"video_parse", "video_analyze", "video_validate"}
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		if toolMap, ok := tool.(map[string]interface{}); ok {
			if name, ok := toolMap["name"].(string); ok {
				toolNames[name] = true
			}
		}
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Expected tool '%s' not found in tools list", expected)
		}
	}
}

func TestMCPVideoValidateSchema(t *testing.T) {
	request := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	response := mcpRequest(t, request)

	result := response["result"].(map[string]interface{})
	tools := result["tools"].([]interface{})

	var validateTool map[string]interface{}
	for _, tool := range tools {
		toolMap := tool.(map[string]interface{})
		if toolMap["name"] == "video_validate" {
			validateTool = toolMap
			break
		}
	}

	if validateTool == nil {
		t.Fatal("video_validate tool not found")
	}

	schema := validateTool["inputSchema"].(map[string]interface{})
	properties := schema["properties"].(map[string]interface{})

	// Check required parameters exist
	requiredParams := []string{"video_path", "narration_path"}
	for _, param := range requiredParams {
		if _, ok := properties[param]; !ok {
			t.Errorf("Expected parameter '%s' in video_validate schema", param)
		}
	}

	// Check segments_path is NOT in schema (old parameter)
	if _, ok := properties["segments_path"]; ok {
		t.Error("Unexpected parameter 'segments_path' in video_validate schema - should use narration_path")
	}

	// Check script_path is optional
	if _, ok := properties["script_path"]; !ok {
		t.Error("Expected optional parameter 'script_path' in video_validate schema")
	}
}

func TestMCPVideoAnalyzeSchema(t *testing.T) {
	request := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	response := mcpRequest(t, request)

	result := response["result"].(map[string]interface{})
	tools := result["tools"].([]interface{})

	var analyzeTool map[string]interface{}
	for _, tool := range tools {
		toolMap := tool.(map[string]interface{})
		if toolMap["name"] == "video_analyze" {
			analyzeTool = toolMap
			break
		}
	}

	if analyzeTool == nil {
		t.Fatal("video_analyze tool not found")
	}

	schema := analyzeTool["inputSchema"].(map[string]interface{})
	properties := schema["properties"].(map[string]interface{})

	// Check required parameters exist
	requiredParams := []string{"narration_path", "audio_dir"}
	for _, param := range requiredParams {
		if _, ok := properties[param]; !ok {
			t.Errorf("Expected parameter '%s' in video_analyze schema", param)
		}
	}

	// Check segments_path is NOT in schema (old parameter)
	if _, ok := properties["segments_path"]; ok {
		t.Error("Unexpected parameter 'segments_path' in video_analyze schema - should use narration_path")
	}

	// Check video_name is NOT in schema (old parameter)
	if _, ok := properties["video_name"]; ok {
		t.Error("Unexpected parameter 'video_name' in video_analyze schema - should use narration_path")
	}
}

func TestMCPVideoBuildSchema(t *testing.T) {
	request := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	response := mcpRequest(t, request)

	result := response["result"].(map[string]interface{})
	tools := result["tools"].([]interface{})

	var buildTool map[string]interface{}
	for _, tool := range tools {
		toolMap := tool.(map[string]interface{})
		if toolMap["name"] == "video_build" {
			buildTool = toolMap
			break
		}
	}

	if buildTool == nil {
		t.Fatal("video_build tool not found")
	}

	schema := buildTool["inputSchema"].(map[string]interface{})
	properties := schema["properties"].(map[string]interface{})

	// Check required parameters exist
	requiredParams := []string{"narration_path", "video_path", "output_path"}
	for _, param := range requiredParams {
		if _, ok := properties[param]; !ok {
			t.Errorf("Expected required parameter '%s' in video_build schema", param)
		}
	}

	// Check optional parameters exist
	optionalParams := []string{"voice", "words_per_minute", "force"}
	for _, param := range optionalParams {
		if _, ok := properties[param]; !ok {
			t.Errorf("Expected optional parameter '%s' in video_build schema", param)
		}
	}

	// Verify required array contains all three required params
	required := schema["required"].([]interface{})
	requiredMap := make(map[string]bool)
	for _, r := range required {
		requiredMap[r.(string)] = true
	}

	for _, param := range requiredParams {
		if !requiredMap[param] {
			t.Errorf("Parameter '%s' should be marked as required", param)
		}
	}
}

func TestMCPVideoParse(t *testing.T) {
	narrationFile := filepath.Join(fixturesDir, "narration.md")

	request := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"video_parse","arguments":{"file_path":"` + narrationFile + `"}}}`
	response := mcpRequest(t, request)

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result in response, got: %v", response)
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("Expected content array in result, got: %v", result)
	}

	// Get the text content
	textContent := content[0].(map[string]interface{})
	text := textContent["text"].(string)

	// Parse the segments JSON
	var segments []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &segments); err != nil {
		t.Fatalf("Failed to parse segments JSON: %v", err)
	}

	if len(segments) != 5 {
		t.Errorf("Expected 5 segments, got %d", len(segments))
	}

	// Verify audio_file uses .m4a extension (not .mp3)
	if len(segments) > 0 {
		audioFile := segments[0]["audio_file"].(string)
		if !strings.HasSuffix(audioFile, ".m4a") {
			t.Errorf("Expected audio_file to end with .m4a, got: %s", audioFile)
		}
	}
}
