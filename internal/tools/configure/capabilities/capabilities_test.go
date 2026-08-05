// Purpose: Tests for describe_capabilities schema generation.
// Docs: docs/features/feature/enhanced-cli-config/index.md

// capabilities_test.go — Tests for capability map building.
package capabilities

import (
	"os"
	"sort"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestCapabilitiesPackageRespectsTenFileBoundary(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			files++
		}
	}
	if files > 10 {
		t.Fatalf("capabilities package has %d files; want at most 10 change-coupled owners", files)
	}
}

func TestBuildCapabilitiesMap_Empty(t *testing.T) {
	t.Parallel()

	result := BuildCapabilitiesMap(nil)
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestBuildCapabilitiesMap_SingleTool(t *testing.T) {
	t.Parallel()

	tools := []mcp.MCPTool{
		{
			Name:        "observe",
			Description: "Observe browser state",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"what": map[string]any{
						"type": "string",
						"enum": []string{"errors", "logs"},
					},
					"limit": map[string]any{
						"type": "number",
					},
				},
				"required": []string{"what"},
			},
		},
	}

	result := BuildCapabilitiesMap(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	observeRaw, ok := result["observe"]
	if !ok {
		t.Fatal("expected 'observe' in result")
	}
	observe := observeRaw.(map[string]any)

	if observe["dispatch_param"] != "what" {
		t.Errorf("dispatch_param = %v, want 'what'", observe["dispatch_param"])
	}

	modes, ok := observe["modes"].([]string)
	if !ok {
		t.Fatal("modes should be []string")
	}
	if len(modes) != 2 || modes[0] != "errors" || modes[1] != "logs" {
		t.Errorf("modes = %v, want [errors, logs]", modes)
	}

	params, ok := observe["params"].([]string)
	if !ok {
		t.Fatal("params should be []string")
	}
	if len(params) != 1 || params[0] != "limit" {
		t.Errorf("params = %v, want [limit]", params)
	}

	if observe["description"] != "Observe browser state" {
		t.Errorf("description = %v, want 'Observe browser state'", observe["description"])
	}
}

func TestBuildCapabilitiesMap_ParamsSorted(t *testing.T) {
	t.Parallel()

	tools := []mcp.MCPTool{
		{
			Name: "configure",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"action": map[string]any{"type": "string"},
					"zebra":  map[string]any{"type": "string"},
					"alpha":  map[string]any{"type": "string"},
					"middle": map[string]any{"type": "string"},
				},
				"required": []string{"action"},
			},
		},
	}

	result := BuildCapabilitiesMap(tools)
	configRaw := result["configure"].(map[string]any)
	params := configRaw["params"].([]string)

	sorted := make([]string, len(params))
	copy(sorted, params)
	sort.Strings(sorted)

	for i := range params {
		if params[i] != sorted[i] {
			t.Errorf("params not sorted: %v", params)
			break
		}
	}
}

func TestBuildCapabilitiesMap_NoRequired(t *testing.T) {
	t.Parallel()

	tools := []mcp.MCPTool{
		{
			Name: "simple",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"foo": map[string]any{"type": "string"},
				},
			},
		},
	}

	result := BuildCapabilitiesMap(tools)
	simple := result["simple"].(map[string]any)
	if simple["dispatch_param"] != "" {
		t.Errorf("dispatch_param = %v, want empty", simple["dispatch_param"])
	}
}

// TestBuildCapabilitiesMap_InferDispatchParamFallbackToAnyOf exercises the
// anyOf fallback path in inferDispatchParam. Real schemas must not use top-level
// anyOf (see TestAllToolSchemas_NoTopLevelCombiners), but inferDispatchParam
// supports it defensively for external or legacy schemas.
func TestBuildCapabilitiesMap_InferDispatchParamFallbackToAnyOf(t *testing.T) {
	t.Parallel()

	// Fixture uses anyOf ([]map[string]any form) — not valid in production schemas.
	tools := []mcp.MCPTool{
		{
			Name: "interact",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"what": map[string]any{
						"type": "string",
						"enum": []string{"navigate", "click"},
					},
					"action": map[string]any{
						"type": "string",
						"enum": []string{"navigate", "click"},
					},
					"url": map[string]any{"type": "string"},
				},
				"anyOf": []map[string]any{
					{"required": []string{"what"}},
					{"required": []string{"action"}},
				},
			},
		},
	}

	result := BuildCapabilitiesMap(tools)
	tool := result["interact"].(map[string]any)

	if got := tool["dispatch_param"]; got != "what" {
		t.Fatalf("dispatch_param = %v, want what", got)
	}

	modes := tool["modes"].([]string)
	if len(modes) != 2 || modes[0] != "navigate" || modes[1] != "click" {
		t.Fatalf("modes = %v, want [navigate click]", modes)
	}
}

// TestBuildCapabilitiesMap_InferDispatchParamAnyOfSliceAny exercises the []any
// branch of inferDispatchParam's anyOf type switch. JSON-unmarshaled schemas
// produce []any, not []map[string]any, so both paths must work.
func TestBuildCapabilitiesMap_InferDispatchParamAnyOfSliceAny(t *testing.T) {
	t.Parallel()

	// Simulate a schema that arrived via JSON round-trip (anyOf as []any).
	// The required field is []any (not []string) to mirror json.Unmarshal output.
	anyOfFixture := []any{
		map[string]any{"required": []any{"what"}},
	}
	// Self-assert: fixture must be []any (not []map[string]any) to test the right branch.
	if _, isSliceAny := anyOfFixture[0].(map[string]any)["required"].([]any); !isSliceAny {
		t.Fatal("fixture setup error: required must be []any to exercise the []any branch")
	}

	tools := []mcp.MCPTool{
		{
			Name: "legacy",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"what": map[string]any{
						"type": "string",
						"enum": []string{"read", "write"},
					},
				},
				"anyOf": anyOfFixture,
			},
		},
	}

	result := BuildCapabilitiesMap(tools)
	tool := result["legacy"].(map[string]any)

	if got := tool["dispatch_param"]; got != "what" {
		t.Fatalf("dispatch_param = %v, want what", got)
	}
}

func TestBuildCapabilitiesMap_IncludesModeParamsAndTypeMetadata(t *testing.T) {
	t.Parallel()

	tools := []mcp.MCPTool{
		{
			Name:        "test_tool",
			Description: "Test tool for metadata",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"what": map[string]any{
						"type": "string",
						"enum": []string{"errors", "logs"},
					},
					"timeout_ms": map[string]any{
						"type":        "number",
						"description": "Timeout ms (default 5000)",
					},
					"url": map[string]any{
						"type": "string",
					},
				},
				"required": []string{"what"},
			},
		},
	}

	result := BuildCapabilitiesMap(tools)
	toolRaw, ok := result["test_tool"]
	if !ok {
		t.Fatal("expected test_tool in result")
	}
	toolMap, ok := toolRaw.(map[string]any)
	if !ok {
		t.Fatalf("test_tool type = %T, want map[string]any", toolRaw)
	}

	modeParamsRaw, ok := toolMap["mode_params"]
	if !ok {
		t.Fatal("expected mode_params in capabilities")
	}
	modeParams, ok := modeParamsRaw.(map[string]any)
	if !ok {
		t.Fatalf("mode_params type = %T, want map[string]any", modeParamsRaw)
	}
	errorsMode, ok := modeParams["errors"].(map[string]any)
	if !ok {
		t.Fatalf("errors mode metadata missing or invalid: %T", modeParams["errors"])
	}

	required, ok := errorsMode["required"].([]string)
	if !ok {
		t.Fatalf("required type = %T, want []string", errorsMode["required"])
	}
	if len(required) != 1 || required[0] != "what" {
		t.Fatalf("required = %v, want [what]", required)
	}

	paramsRaw, ok := errorsMode["params"]
	if !ok {
		t.Fatal("expected params map in mode metadata")
	}
	params, ok := paramsRaw.(map[string]any)
	if !ok {
		t.Fatalf("params type = %T, want map[string]any", paramsRaw)
	}

	timeoutMetaRaw, ok := params["timeout_ms"]
	if !ok {
		t.Fatal("expected timeout_ms metadata in params")
	}
	timeoutMeta, ok := timeoutMetaRaw.(map[string]any)
	if !ok {
		t.Fatalf("timeout_ms metadata type = %T, want map[string]any", timeoutMetaRaw)
	}
	if timeoutMeta["type"] != "number" {
		t.Fatalf("timeout_ms type = %v, want number", timeoutMeta["type"])
	}
	if timeoutMeta["default"] != "5000" {
		t.Fatalf("timeout_ms default = %v, want 5000", timeoutMeta["default"])
	}
}
