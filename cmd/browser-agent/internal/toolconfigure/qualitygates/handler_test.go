// handler_test.go — Tests quality-gate setup boundary behavior.

package qualitygates

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type fakeCodebase struct{ path string }

func (f fakeCodebase) GetActiveCodebase() string { return f.path }

func TestHandleRequiresActiveCodebase(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	response := Handle(fakeCodebase{}, req, nil)
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected missing active codebase to fail")
	}
}

func decodeResponse(t *testing.T, response mcp.JSONRPCResponse) (mcp.MCPToolResult, map[string]any) {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 {
		t.Fatal("quality-gate handler returned no content")
	}
	start := strings.IndexByte(result.Content[0].Text, '{')
	if start < 0 {
		t.Fatalf("quality-gate response has no JSON payload: %q", result.Content[0].Text)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text[start:]), &data); err != nil {
		t.Fatal(err)
	}
	return result, data
}

func callHandler(t *testing.T, projectDir, arguments string) (mcp.MCPToolResult, map[string]any) {
	t.Helper()
	return decodeResponse(t, Handle(
		fakeCodebase{path: projectDir},
		mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)},
		json.RawMessage(arguments),
	))
}

func TestHandleCreatesQualityGateFilesAndHooks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	result, data := callHandler(t, dir, `{}`)
	if result.IsError {
		t.Fatalf("setup failed: %s", result.Content[0].Text)
	}
	configBytes, err := os.ReadFile(filepath.Join(dir, ".kaboom.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(configBytes, &config); err != nil {
		t.Fatal(err)
	}
	if config["code_standards"] != "kaboom-code-standards.md" || config["file_size_limit"] == nil {
		t.Fatalf("unexpected defaults: %+v", config)
	}
	standards, err := os.ReadFile(filepath.Join(dir, "kaboom-code-standards.md"))
	if err != nil || len(standards) < 100 {
		t.Fatalf("starter standards missing: %v", err)
	}
	settings, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{
		kaboomHookQualityGate, kaboomHookCompressOutput, kaboomHookSessionTrack,
		kaboomHookBlastRadius, kaboomHookDecisionGuard,
	} {
		if !strings.Contains(string(settings), command) {
			t.Errorf("settings missing %q", command)
		}
	}
	if data["hooks_installed"] != true || data["config_path"] == "" || data["settings_path"] == "" {
		t.Fatalf("unexpected setup response: %+v", data)
	}
	defaults, ok := data["defaults"].(map[string]any)
	if !ok || defaults["file_size_limit"] == nil || defaults["code_standards"] == nil {
		t.Fatalf("response defaults missing: %+v", data["defaults"])
	}
	if suggestions, ok := data["suggestions"].([]any); !ok || len(suggestions) == 0 {
		t.Fatalf("response suggestions missing: %+v", data["suggestions"])
	}
	if strings.Contains(result.Content[0].Text, "fileSizeLimit") {
		t.Fatalf("camelCase field leaked into response: %s", result.Content[0].Text)
	}
}

func TestHandlePreservesExistingConfigurationAndSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	config := `{"code_standards":"docs/my-patterns.md","file_size_limit":500}`
	if err := os.WriteFile(filepath.Join(dir, ".kaboom.json"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	settingsDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(`{"model":"sonnet","permissions":{"allow":["Read"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, data := callHandler(t, dir, `{}`)
	if result.IsError {
		t.Fatalf("setup failed: %s", result.Content[0].Text)
	}
	gotConfig, err := os.ReadFile(filepath.Join(dir, ".kaboom.json"))
	if err != nil || string(gotConfig) != config {
		t.Fatalf("existing config changed: %q, %v", gotConfig, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "kaboom-code-standards.md")); !os.IsNotExist(err) {
		t.Fatal("default standards created despite custom standards path")
	}
	settingsBytes, err := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(settingsBytes, &settings); err != nil {
		t.Fatal(err)
	}
	if settings["model"] != "sonnet" || settings["permissions"] == nil {
		t.Fatalf("existing settings were not preserved: %+v", settings)
	}
	if data["config_existed"] != true || data["hooks_installed"] != true {
		t.Fatalf("unexpected preservation response: %+v", data)
	}
}

func TestHandleIsIdempotentAndPreservesExistingStandards(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const standards = "# Custom standards\n\nPreserve this file."
	if err := os.WriteFile(filepath.Join(dir, "kaboom-code-standards.md"), []byte(standards), 0o644); err != nil {
		t.Fatal(err)
	}
	first, _ := callHandler(t, dir, `{}`)
	if first.IsError {
		t.Fatalf("first setup failed: %s", first.Content[0].Text)
	}
	second, data := callHandler(t, dir, `{}`)
	if second.IsError || data["hooks_installed"] != false {
		t.Fatalf("second setup was not idempotent: %+v", data)
	}
	gotStandards, err := os.ReadFile(filepath.Join(dir, "kaboom-code-standards.md"))
	if err != nil || string(gotStandards) != standards {
		t.Fatalf("existing standards changed: %q, %v", gotStandards, err)
	}
	settingsBytes, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(settingsBytes, &settings); err != nil {
		t.Fatal(err)
	}
	hooks := settings["hooks"].(map[string]any)
	if got := len(hooks["PostToolUse"].([]any)); got != 3 {
		t.Fatalf("managed hooks duplicated: got %d entries", got)
	}
}

func TestHandleValidatesTargetDirectoryBoundary(t *testing.T) {
	t.Parallel()
	projectDir := t.TempDir()
	subdir := filepath.Join(projectDir, "subproject")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	result, _ := callHandler(t, projectDir, `{"target_dir":`+strconvQuote(subdir)+`}`)
	if result.IsError {
		t.Fatalf("in-project target failed: %s", result.Content[0].Text)
	}
	if _, err := os.Stat(filepath.Join(subdir, ".kaboom.json")); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	result, _ = callHandler(t, projectDir, `{"target_dir":`+strconvQuote(outside)+`}`)
	if !result.IsError || !strings.Contains(result.Content[0].Text, "outside") {
		t.Fatalf("outside target was not rejected: %s", result.Content[0].Text)
	}
}

func strconvQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
