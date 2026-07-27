// Purpose: Tests for tool schema parity between bridge and daemon.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_schema_parity_test.go — Enforces parity between tool schema enums and runtime dispatch handlers.
package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/cli"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	interactschema "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema/interact"
)

func TestSchemaParity_AnalyzeWhatEnumMatchesHandlers(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	schemaModes := mustToolEnumValues(t, h.ToolsList(), "analyze", "what")
	runtimeModes := sortedKeysAnalyzeHandlers()

	assertSameStringSet(t, "analyze.what enum vs analyzeHandlers", schemaModes, runtimeModes)
}

func TestSchemaParity_GenerateWhatEnumMatchesHandlers(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)
	schemaFormats := mustToolEnumValues(t, h.ToolsList(), "generate", "what")
	runtimeFormats := sortedKeysGenerateHandlers()
	assertSameStringSet(t, "generate.what enum vs generateHandlers", schemaFormats, runtimeFormats)
}

func TestSchemaParity_ConfigureWhatEnumMatchesHandlers(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)
	schemaActions := mustToolEnumValues(t, h.ToolsList(), "configure", "what")
	runtimeActions := sortedKeysConfigureHandlers()
	assertSameStringSet(t, "configure.what enum vs configureHandlers", schemaActions, runtimeActions)
}

func TestSchemaParity_InteractWhatEnumMatchesDispatch(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	schemaActions := mustToolEnumValues(t, h.ToolsList(), "interact", "what")
	runtimeActions := sortedInteractRuntimeActions(h)

	assertSameStringSet(t, "interact.what enum vs interact runtime actions", schemaActions, runtimeActions)
}

func TestSchemaParity_ObserveWhatEnumMatchesHandlers(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	schemaModes := mustToolEnumValues(t, h.ToolsList(), "observe", "what")
	runtimeModes := sortedKeysObserveHandlers(h)

	assertSameStringSet(t, "observe.what enum vs observe dispatcher", schemaModes, runtimeModes)
}

func sortedKeysGenerateHandlers() []string {
	return strings.Split(toolgenerate.ValidFormats(), ", ")
}

func sortedKeysConfigureHandlers() []string {
	keys := make([]string, 0, len(configureHandlers))
	for action := range configureHandlers {
		keys = append(keys, action)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysAnalyzeHandlers() []string {
	keys := make([]string, 0, len(analyzeHandlers))
	for mode := range analyzeHandlers {
		keys = append(keys, mode)
	}
	sort.Strings(keys)
	return keys
}

// observeSilentModes are registered handlers intentionally omitted from the
// schema enum so they don't appear in tool descriptions. They work at runtime
// but are not advertised — e.g., annotation modes proxied from analyze.
var observeSilentModes = map[string]bool{
	"annotations":       true,
	"annotation_detail": true,
	"draw_history":      true,
	"draw_session":      true,
}

func sortedKeysObserveHandlers(h *ToolHandler) []string {
	keys := make([]string, 0)
	for _, mode := range h.observeDispatcher.ValidModes() {
		if !observeSilentModes[mode] {
			keys = append(keys, mode)
		}
	}
	return keys
}

func sortedInteractRuntimeActions(h *ToolHandler) []string {
	// Build set of alias action names to exclude from parity check.
	// Aliases are hidden from the schema enum but still routed at runtime.
	aliasSet := make(map[string]bool)
	for _, spec := range interactschema.ActionSpecs() {
		if spec.IsAlias {
			aliasSet[spec.Name] = true
		}
	}

	actions := make(map[string]bool)
	for action := range getInteractHandlers() {
		if !aliasSet[action] {
			actions[action] = true
		}
	}
	keys := make([]string, 0, len(actions))
	for action := range actions {
		keys = append(keys, action)
	}
	sort.Strings(keys)
	return keys
}

func mustToolEnumValues(t *testing.T, tools []mcp.MCPTool, toolName, propertyName string) []string {
	t.Helper()

	var tool *mcp.MCPTool
	for i := range tools {
		if tools[i].Name == toolName {
			tool = &tools[i]
			break
		}
	}
	if tool == nil {
		t.Fatalf("tool %q not found in ToolsList", toolName)
	}

	props, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("tool %q schema missing properties", toolName)
	}

	prop, ok := props[propertyName].(map[string]any)
	if !ok {
		t.Fatalf("tool %q schema missing property %q", toolName, propertyName)
	}

	enumRaw, ok := prop["enum"]
	if !ok {
		t.Fatalf("tool %q property %q missing enum", toolName, propertyName)
	}

	enum, err := toStringSlice(enumRaw)
	if err != nil {
		t.Fatalf("tool %q property %q enum parse failed: %v", toolName, propertyName, err)
	}

	sort.Strings(enum)
	return enum
}

func toStringSlice(v any) ([]string, error) {
	switch vv := v.(type) {
	case []string:
		out := make([]string, len(vv))
		copy(out, vv)
		return out, nil
	case []any:
		out := make([]string, 0, len(vv))
		for i, item := range vv {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("enum[%d] is %T, want string", i, item)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported enum type %T", v)
	}
}

// TestCLIParserParity_AllSchemaPropertiesMapped verifies that every MCP schema property
// for each tool has a corresponding CLI flag in the parser. This prevents drift where
// new schema properties are added but the CLI parser isn't updated.
func TestCLIParserParity_AllSchemaPropertiesMapped(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)
	tools := h.ToolsList()

	// Known exceptions: params that are intentionally not CLI flags.
	// "what" is the positional mode arg, not a flag.
	// Deprecated aliases are handled at runtime, not exposed as CLI flags.
	globalExceptions := map[string]bool{"what": true}
	perTool := map[string]map[string]bool{
		"observe":   {"telemetry_mode": true},
		"analyze":   {"telemetry_mode": true},
		"generate":  {"telemetry_mode": true, "format": true},
		"configure": {"telemetry_mode": true, "action": true},
		"interact":  {"telemetry_mode": true, "action": true},
	}

	cliParsers := map[string]func(string, []string) (map[string]any, error){
		"observe":   cli.ParseObserveArgs,
		"analyze":   cli.ParseAnalyzeArgs,
		"generate":  cli.ParseGenerateArgs,
		"configure": cli.ParseConfigureArgs,
		"interact":  cli.ParseInteractArgs,
	}

	for _, tool := range tools {
		parser, ok := cliParsers[tool.Name]
		if !ok {
			continue
		}
		t.Run(tool.Name, func(t *testing.T) {
			t.Parallel()
			props, ok := tool.InputSchema["properties"].(map[string]any)
			if !ok {
				t.Fatal("schema missing properties")
			}

			// Collect CLI mcpKeys by parsing a dummy call with no args.
			// We can't easily extract keys from the parser function, so we check
			// that the schema property count is reasonable vs the parser.
			_ = parser // used below

			exceptions := perTool[tool.Name]
			var missing []string
			for propName := range props {
				if globalExceptions[propName] || exceptions[propName] {
					continue
				}
				// Check that the flag is recognized by the parser. Pass a value
				// that works for all flag kinds; only flag "unknown flag" as missing.
				flag := "--" + schemaKeyToCLIFlag(propName)
				_, err := parser("test", []string{flag, "1"})
				if err != nil && strings.Contains(err.Error(), "unknown flag: "+flag) {
					missing = append(missing, propName)
				}
			}
			sort.Strings(missing)
			if len(missing) > 0 {
				t.Errorf("CLI parser for %s is missing flags for schema properties: %v", tool.Name, missing)
			}
		})
	}
}

// schemaKeyToCLIFlag converts a snake_case MCP key to kebab-case CLI flag name.
func schemaKeyToCLIFlag(key string) string {
	return strings.ReplaceAll(key, "_", "-")
}

func assertSameStringSet(t *testing.T, label string, got, want []string) {
	t.Helper()

	gotSet := make(map[string]bool, len(got))
	wantSet := make(map[string]bool, len(want))

	for _, v := range got {
		gotSet[v] = true
	}
	for _, v := range want {
		wantSet[v] = true
	}

	missingInSchema := make([]string, 0)
	missingInRuntime := make([]string, 0)

	for v := range wantSet {
		if !gotSet[v] {
			missingInSchema = append(missingInSchema, v)
		}
	}
	for v := range gotSet {
		if !wantSet[v] {
			missingInRuntime = append(missingInRuntime, v)
		}
	}

	sort.Strings(missingInSchema)
	sort.Strings(missingInRuntime)

	if len(missingInSchema) > 0 || len(missingInRuntime) > 0 {
		t.Fatalf(
			"%s mismatch\nmissing_in_schema=%v\nmissing_in_runtime=%v\ngot=%v\nwant=%v",
			label,
			missingInSchema,
			missingInRuntime,
			got,
			want,
		)
	}
}
