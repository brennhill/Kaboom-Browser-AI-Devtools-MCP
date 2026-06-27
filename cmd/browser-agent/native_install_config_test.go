// native_install_config_test.go — Tests for mergeJSONConfig safety guarantees.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeJSONConfig_PreservesExistingServers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	existing := map[string]any{
		"mcpServers": map[string]any{
			"github":    map[string]any{"command": "github-mcp"},
			"atlassian": map[string]any{"command": "atlassian-mcp"},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/kaboom", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	result := readJSONFile(t, path)
	servers := result["mcpServers"].(map[string]any)

	if _, ok := servers["github"]; !ok {
		t.Error("github server was deleted")
	}
	if _, ok := servers["atlassian"]; !ok {
		t.Error("atlassian server was deleted")
	}
	if _, ok := servers[mcpServerName]; !ok {
		t.Errorf("%s server was not added", mcpServerName)
	}
}

func TestMergeJSONConfig_RefusesToOverwriteInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	if err := os.WriteFile(path, []byte(`{not valid json`), 0600); err != nil {
		t.Fatal(err)
	}

	err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/kaboom", false)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	// Verify the original file was NOT overwritten.
	content, _ := os.ReadFile(path)
	if string(content) != `{not valid json` {
		t.Errorf("original file was modified: %s", content)
	}
}

func TestMergeJSONConfig_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	original := `{"mcpServers": {"other": {"command": "other-mcp"}}}`
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/kaboom", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	bakPath := path + ".bak"
	bakContent, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("backup file not created: %v", err)
	}
	if string(bakContent) != original {
		t.Errorf("backup content mismatch: got %s", bakContent)
	}
}

func TestMergeJSONConfig_RemovesLegacyKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	existing := map[string]any{
		"mcpServers": map[string]any{
			"kaboom":                 map[string]any{"command": "old"},
			"kaboom-agentic-browser": map[string]any{"command": "older"},
			"github":                   map[string]any{"command": "github-mcp"},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/kaboom", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	result := readJSONFile(t, path)
	servers := result["mcpServers"].(map[string]any)

	for _, legacy := range installerLegacyServerKeys {
		if _, ok := servers[legacy]; ok {
			t.Errorf("legacy key %q was not removed", legacy)
		}
	}
	if _, ok := servers["github"]; !ok {
		t.Error("github server was deleted")
	}
	if _, ok := servers[mcpServerName]; !ok {
		t.Errorf("%s server was not added", mcpServerName)
	}
}

func TestMergeJSONConfig_EmptyFileCreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	if err := os.WriteFile(path, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/kaboom", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	result := readJSONFile(t, path)
	servers := result["mcpServers"].(map[string]any)
	if _, ok := servers[mcpServerName]; !ok {
		t.Errorf("%s server was not added", mcpServerName)
	}
}

func TestMergeJSONConfig_MissingFileCreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	if err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/kaboom", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	result := readJSONFile(t, path)
	servers := result["mcpServers"].(map[string]any)
	if _, ok := servers[mcpServerName]; !ok {
		t.Errorf("%s server was not added", mcpServerName)
	}
}

func TestMergeJSONConfig_VSCodeServersKeyShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	// Pre-existing VS Code mcp.json with another server under "servers".
	original := `{"servers": {"other": {"command": "other-mcp", "args": []}}}`
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeJSONConfig(path, "servers", "/usr/local/bin/kaboom", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}

	result := readJSONFile(t, path)
	servers, ok := result["servers"].(map[string]any)
	if !ok {
		t.Fatalf("top-level %q key missing or wrong type: %v", "servers", result)
	}
	if _, ok := servers["other"]; !ok {
		t.Error("existing VS Code server was deleted")
	}
	entry, ok := servers[mcpServerName].(map[string]any)
	if !ok {
		t.Fatalf("%s entry missing under servers key", mcpServerName)
	}
	if entry["command"] != "/usr/local/bin/kaboom" {
		t.Errorf("entry command = %v, want /usr/local/bin/kaboom", entry["command"])
	}
	if _, ok := entry["args"]; !ok {
		t.Error("entry should keep the {command,args} shape")
	}
	if _, ok := result["mcpServers"]; ok {
		t.Error("VS Code config must not gain an mcpServers key")
	}
}

func TestMergeJSONConfig_AtomicWriteLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	original := `{"mcpServers": {"other": {"command": "other-mcp"}}}`
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/kaboom", false); err != nil {
		t.Fatalf("mergeJSONConfig failed: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind after merge: stat err = %v", err)
	}

	// Block the temp path with a directory: merge must fail without
	// truncating the existing config (regression for plain WriteFile).
	if err := os.MkdirAll(path+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := mergeJSONConfig(path, "mcpServers", "/usr/local/bin/kaboom", false); err == nil {
		t.Fatal("expected error when temp path is blocked, got nil")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	result := readJSONFile(t, path)
	if _, ok := result["mcpServers"].(map[string]any); !ok {
		t.Fatalf("existing config corrupted by failed merge: %s", content)
	}
}

func TestGoStaticContractsUseKaboomBranding(t *testing.T) {
	checklist := manualExtensionSetupChecklist("/tmp/KaboomExtension")
	joinedChecklist := strings.Join(checklist, "\n")
	if !strings.Contains(joinedChecklist, "Pin Kaboom") {
		t.Fatalf("manualExtensionSetupChecklist should mention Kaboom pinning, got %q", joinedChecklist)
	}
	if !strings.Contains(joinedChecklist, "Open the Kaboom popup") {
		t.Fatalf("manualExtensionSetupChecklist should mention the Kaboom popup, got %q", joinedChecklist)
	}

	setupHTML, err := os.ReadFile("setup.html")
	if err != nil {
		t.Fatalf("os.ReadFile(setup.html) error = %v", err)
	}
	setupText := string(setupHTML)
	if !strings.Contains(setupText, "Kaboom MCP Server") {
		t.Fatalf("setup.html should mention Kaboom MCP Server")
	}
	if !strings.Contains(setupText, "kaboom-mcp") {
		t.Fatalf("setup.html should reference kaboom-mcp")
	}

	docsHTML, err := os.ReadFile("docs.html")
	if err != nil {
		t.Fatalf("os.ReadFile(docs.html) error = %v", err)
	}
	if !strings.Contains(string(docsHTML), "Kaboom MCP Server") {
		t.Fatalf("docs.html should mention Kaboom MCP Server")
	}

	logsHTML, err := os.ReadFile("logs.html")
	if err != nil {
		t.Fatalf("os.ReadFile(logs.html) error = %v", err)
	}
	if !strings.Contains(string(logsHTML), "Kaboom MCP Server") {
		t.Fatalf("logs.html should mention Kaboom MCP Server")
	}

	openapiJSON, err := os.ReadFile("openapi.json")
	if err != nil {
		t.Fatalf("os.ReadFile(openapi.json) error = %v", err)
	}
	openapiText := string(openapiJSON)
	if !strings.Contains(openapiText, "Kaboom MCP Server") {
		t.Fatalf("openapi.json should mention Kaboom MCP Server")
	}
	if !strings.Contains(openapiText, "X-Kaboom-Client") {
		t.Fatalf("openapi.json should reference X-Kaboom-Client")
	}
}

// readJSONFile is a test helper that reads and parses a JSON file.
func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	var result map[string]any
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("failed to parse %s: %v", path, err)
	}
	return result
}
