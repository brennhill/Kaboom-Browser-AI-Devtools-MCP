// invariants_test.go — Schema invariants that must hold for Claude API compatibility.
package schema

import (
	"encoding/json"
	"slices"
	"testing"
)

// TestAllToolSchemas_NoTopLevelCombiners ensures no tool input_schema uses
// oneOf, allOf, or anyOf at the top level. The Claude API rejects such schemas
// (error: "input_schema does not support oneOf, allOf, or anyOf at the top level").
func TestAllToolSchemas_NoTopLevelCombiners(t *testing.T) {
	t.Parallel()

	forbidden := []string{"oneOf", "allOf", "anyOf"}

	for _, tool := range AllTools() {
		t.Run(tool.Name, func(t *testing.T) {
			t.Parallel()
			for _, key := range forbidden {
				if _, found := tool.InputSchema[key]; found {
					t.Errorf("tool %q: input_schema has top-level %q — Claude API does not support oneOf/allOf/anyOf at the top level", tool.Name, key)
				}
			}
		})
	}
}

func TestAnalyzeSchemaHasNoAnnotationURLAlias(t *testing.T) {
	t.Parallel()

	properties := analyzeToolSchema().InputSchema["properties"].(map[string]any)
	if _, exists := properties["url_pattern"]; exists {
		t.Fatal("analyze schema retains url_pattern compatibility alias; use url")
	}
}

func TestAnalyzeSchemaIncludesQueuedAnalysisFamilies(t *testing.T) {
	t.Parallel()
	properties := analyzeToolSchema().InputSchema["properties"].(map[string]any)
	what := properties["what"].(map[string]any)["enum"].([]string)
	for _, mode := range []string{"navigation", "page_structure", "form_state", "data_table", "audit"} {
		if !slices.Contains(what, mode) {
			t.Errorf("analyze what enum missing %q: %v", mode, what)
		}
	}
}

func TestAnalyzeSchemaDeclaresFrameTarget(t *testing.T) {
	t.Parallel()
	properties := analyzeToolSchema().InputSchema["properties"].(map[string]any)
	frame, ok := properties["frame"].(map[string]any)
	if !ok || frame["type"] != "string" {
		t.Fatalf("analyze frame schema = %#v", properties["frame"])
	}
}

func TestGenerateSchemaDeclaresSharedDispatchParameters(t *testing.T) {
	t.Parallel()
	properties := generateToolSchema().InputSchema["properties"].(map[string]any)
	for _, name := range []string{"what", "context", "action", "telemetry_mode", "save_to"} {
		if _, exists := properties[name]; !exists {
			t.Errorf("generate schema missing shared dispatch parameter %q", name)
		}
	}
}

// TestAllToolSchemas_NoNestedCombiners checks that property-level schemas also
// avoid combiners. Nested oneOf/anyOf/allOf in property definitions can cause
// Claude API validation errors in some contexts. Walks recursively into items
// and nested properties so array-typed fields are also covered.
func TestAllToolSchemas_NoNestedCombiners(t *testing.T) {
	t.Parallel()

	forbidden := []string{"oneOf", "allOf", "anyOf", "not"}

	for _, tool := range AllTools() {
		t.Run(tool.Name, func(t *testing.T) {
			t.Parallel()
			props, ok := tool.InputSchema["properties"].(map[string]any)
			if !ok {
				return
			}
			checkPropsForCombiners(t, tool.Name, props, forbidden)
		})
	}
}

// checkPropsForCombiners recursively checks a properties map and any nested
// items.properties for forbidden combiner keywords.
func checkPropsForCombiners(t *testing.T, toolName string, props map[string]any, forbidden []string) {
	t.Helper()
	for propName, propRaw := range props {
		prop, ok := propRaw.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range forbidden {
			if _, found := prop[key]; found {
				t.Errorf("tool %q property %q: has nested %q — avoid combiners in property schemas", toolName, propName, key)
			}
		}
		// Recurse into items.properties for array-typed fields.
		if items, ok := prop["items"].(map[string]any); ok {
			if nestedProps, ok := items["properties"].(map[string]any); ok {
				checkPropsForCombiners(t, toolName, nestedProps, forbidden)
			}
		}
	}
}

// TestObserveSchema_LevelNotExposed verifies that min_level is the sole public
// severity filter for browser logs.
func TestObserveSchema_LevelNotExposed(t *testing.T) {
	t.Parallel()
	tool := observeToolSchema()
	props, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("observe schema missing properties")
	}
	if _, found := props["level"]; found {
		t.Error("observe schema should not expose the removed exact-level filter")
	}
	if _, found := props["min_level"]; !found {
		t.Error("observe schema should expose 'min_level'")
	}
}

// TestAllToolSchemas_HavePropertiesAndObjectType ensures every tool schema has
// type:object and a non-empty properties field, catching accidentally empty or
// malformed schemas.
func TestAllToolSchemas_HavePropertiesAndObjectType(t *testing.T) {
	t.Parallel()

	for _, tool := range AllTools() {
		t.Run(tool.Name, func(t *testing.T) {
			t.Parallel()
			props, ok := tool.InputSchema["properties"].(map[string]any)
			if !ok {
				t.Errorf("tool %q: input_schema missing or invalid top-level \"properties\"", tool.Name)
			} else if len(props) == 0 {
				t.Errorf("tool %q: input_schema \"properties\" is empty — every tool must declare at least one parameter", tool.Name)
			}
			typeVal, ok := tool.InputSchema["type"].(string)
			if !ok || typeVal != "object" {
				t.Errorf("tool %q: input_schema type = %T(%v), want string \"object\"", tool.Name, tool.InputSchema["type"], tool.InputSchema["type"])
			}
		})
	}
}

// TestAllToolSchemas_ValidJSON ensures every tool schema serializes to valid JSON
// and survives a round-trip without losing properties. This catches type
// mismatches (e.g. []int instead of []string in required) that compile fine but
// break at the MCP serialization boundary.
func TestAllToolSchemas_ValidJSON(t *testing.T) {
	t.Parallel()

	for _, tool := range AllTools() {
		t.Run(tool.Name, func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatalf("tool %q: input_schema failed JSON marshal: %v", tool.Name, err)
			}
			var round map[string]any
			if err := json.Unmarshal(data, &round); err != nil {
				t.Fatalf("tool %q: marshaled schema is not valid JSON: %v", tool.Name, err)
			}
			// Verify the round-trip preserved the properties map and its size.
			origProps, _ := tool.InputSchema["properties"].(map[string]any)
			roundProps, ok := round["properties"].(map[string]any)
			if !ok {
				t.Fatalf("tool %q: round-trip lost \"properties\"", tool.Name)
			}
			if len(roundProps) != len(origProps) {
				t.Errorf("tool %q: round-trip properties count = %d, want %d", tool.Name, len(roundProps), len(origProps))
			}
			// Verify type field survived round-trip.
			if round["type"] != tool.InputSchema["type"] {
				t.Errorf("tool %q: round-trip type = %v, want %v", tool.Name, round["type"], tool.InputSchema["type"])
			}
		})
	}
}
