// config_test.go — Tests for mergeJSONConfig safety guarantees.

package nativeinstall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/identity"
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
	if _, ok := servers[identity.MCPServerName]; !ok {
		t.Errorf("%s server was not added", identity.MCPServerName)
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

func TestMergeJSONConfigPreservesUnrelatedServers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	existing := map[string]any{
		"mcpServers": map[string]any{
			"gasoline": map[string]any{"command": "other-mcp"},
			"github":   map[string]any{"command": "github-mcp"},
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

	if _, ok := servers["gasoline"]; !ok {
		t.Error("installer deleted an unrelated non-canonical server")
	}
	if _, ok := servers["github"]; !ok {
		t.Error("github server was deleted")
	}
	if _, ok := servers[identity.MCPServerName]; !ok {
		t.Errorf("%s server was not added", identity.MCPServerName)
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
	if _, ok := servers[identity.MCPServerName]; !ok {
		t.Errorf("%s server was not added", identity.MCPServerName)
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
	if _, ok := servers[identity.MCPServerName]; !ok {
		t.Errorf("%s server was not added", identity.MCPServerName)
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
	entry, ok := servers[identity.MCPServerName].(map[string]any)
	if !ok {
		t.Fatalf("%s entry missing under servers key", identity.MCPServerName)
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

func TestManualExtensionChecklistUsesKaboomBranding(t *testing.T) {
	checklist := strings.Join(manualExtensionSetupChecklist("/tmp/KaboomExtension"), "\n")
	if !strings.Contains(checklist, "Pin Kaboom") {
		t.Fatalf("checklist should mention Kaboom pinning, got %q", checklist)
	}
	if !strings.Contains(checklist, "Open the Kaboom popup") {
		t.Fatalf("checklist should mention the Kaboom popup, got %q", checklist)
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
