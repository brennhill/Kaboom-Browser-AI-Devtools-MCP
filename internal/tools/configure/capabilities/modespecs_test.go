// modespecs_test.go — Tests for per-mode parameter specs.
// Docs: docs/features/generation/describe_capabilities.md
package capabilities

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema/interact"
)

func TestToolModeSpecs_AllToolsPresent(t *testing.T) {
	t.Parallel()

	expected := []string{"observe", "interact", "analyze", "generate", "configure"}
	for _, tool := range expected {
		if _, ok := toolModeSpecs[tool]; !ok {
			t.Errorf("toolModeSpecs missing tool %q", tool)
		}
	}
	if len(toolModeSpecs) != len(expected) {
		t.Errorf("toolModeSpecs has %d tools, want %d", len(toolModeSpecs), len(expected))
	}
}

func TestToolModeSpecs_NoUnknownParams(t *testing.T) {
	t.Parallel()

	tools := schema.AllTools()
	for _, tool := range tools {
		specs, ok := toolModeSpecs[tool.Name]
		if !ok {
			continue
		}
		props, _ := tool.InputSchema["properties"].(map[string]any)
		for mode, spec := range specs {
			for _, param := range spec.Required {
				if _, ok := props[param]; !ok {
					t.Errorf("%s/%s: required param %q not in schema", tool.Name, mode, param)
				}
			}
			for _, param := range spec.Optional {
				if _, ok := props[param]; !ok {
					t.Errorf("%s/%s: optional param %q not in schema", tool.Name, mode, param)
				}
			}
		}
	}
}

func TestToolModeSpecs_AllModesHaveSpecs(t *testing.T) {
	t.Parallel()

	tools := schema.AllTools()
	for _, tool := range tools {
		specs, ok := toolModeSpecs[tool.Name]
		if !ok {
			continue
		}
		props, _ := tool.InputSchema["properties"].(map[string]any)
		required := toStringSlice(tool.InputSchema["required"])
		dispatchParam := ""
		if len(required) > 0 {
			dispatchParam = required[0]
		}
		modes := extractModes(dispatchParam, props)
		for _, mode := range modes {
			if _, ok := specs[mode]; !ok {
				t.Errorf("%s: mode %q has no spec entry", tool.Name, mode)
			}
		}
	}
}

func TestToolModeSpecs_ObserveErrorsFiltered(t *testing.T) {
	t.Parallel()

	spec, ok := toolModeSpecs["observe"]["errors"]
	if !ok {
		t.Fatal("observe/errors spec missing")
	}

	allParams := append(append([]string{}, spec.Required...), spec.Optional...)
	excluded := []string{"format", "quality", "full_page", "selector", "wait_for_stable", "database", "store", "body_path"}
	for _, param := range excluded {
		if containsString(allParams, param) {
			t.Errorf("observe/errors should not include %q", param)
		}
	}
}

func TestToolModeSpecs_InteractClickFiltered(t *testing.T) {
	t.Parallel()

	spec, ok := toolModeSpecs["interact"]["click"]
	if !ok {
		t.Fatal("interact/click spec missing")
	}

	allParams := append(append([]string{}, spec.Required...), spec.Optional...)
	excluded := []string{"file_path", "api_endpoint", "audio", "fps", "script", "fields", "submit_selector"}
	for _, param := range excluded {
		if containsString(allParams, param) {
			t.Errorf("interact/click should not include %q", param)
		}
	}
}

func TestToolModeSpecs_AllModesHaveHints(t *testing.T) {
	t.Parallel()

	for toolName, specs := range toolModeSpecs {
		for mode, spec := range specs {
			if spec.Hint == "" {
				t.Errorf("%s/%s: missing hint", toolName, mode)
			}
		}
	}
}

func TestBuildCapabilitiesSummary_IncludesHints(t *testing.T) {
	t.Parallel()

	tools := schema.AllTools()
	summary := BuildCapabilitiesSummary(tools)

	for _, tool := range tools {
		toolRaw, ok := summary[tool.Name]
		if !ok {
			t.Errorf("missing tool %q in summary", tool.Name)
			continue
		}
		toolMap := toolRaw.(map[string]any)
		modes, ok := toolMap["modes"].(map[string]string)
		if !ok {
			t.Fatalf("%s: modes type = %T, want map[string]string", tool.Name, toolMap["modes"])
		}
		if len(modes) == 0 {
			t.Errorf("%s: no modes in summary", tool.Name)
			continue
		}
		for mode, hint := range modes {
			if hint == "" {
				t.Errorf("%s/%s: empty hint in summary", tool.Name, mode)
			}
		}
	}
}

func TestBuildCapabilitiesSummary_ObserveHints(t *testing.T) {
	t.Parallel()

	tools := []mcp.MCPTool{
		{
			Name:        "observe",
			Description: "Observe browser state",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"what": map[string]any{
						"type": "string",
						"enum": []string{"errors", "screenshot"},
					},
				},
				"required": []string{"what"},
			},
		},
	}

	summary := BuildCapabilitiesSummary(tools)
	observeRaw := summary["observe"].(map[string]any)
	modes := observeRaw["modes"].(map[string]string)

	if modes["errors"] != "Raw JavaScript console errors. summary=true returns counts by source + top messages" {
		t.Errorf("errors hint = %q, want 'Raw JavaScript console errors. summary=true returns counts by source + top messages'", modes["errors"])
	}
	if modes["screenshot"] != "Capture page screenshot (full page or element)" {
		t.Errorf("screenshot hint = %q", modes["screenshot"])
	}
}

func TestInteractModeSpecs_DerivedFromSchemaRegistry(t *testing.T) {
	t.Parallel()

	actionSpecs := interact.ActionSpecs()
	if len(actionSpecs) == 0 {
		t.Fatal("interact.ActionSpecs() should be non-empty")
	}

	for _, spec := range actionSpecs {
		modeSpec, ok := interactModeSpecs[spec.Name]
		if !ok {
			t.Fatalf("interact mode spec missing action %q", spec.Name)
		}
		if modeSpec.Hint != spec.Hint {
			t.Fatalf("hint mismatch for %q: got=%q want=%q", spec.Name, modeSpec.Hint, spec.Hint)
		}
		if !equalStringSlices(modeSpec.Required, spec.Required) {
			t.Fatalf("required params mismatch for %q: got=%v want=%v", spec.Name, modeSpec.Required, spec.Required)
		}
		if !equalStringSlices(modeSpec.Optional, spec.Optional) {
			t.Fatalf("optional params mismatch for %q: got=%v want=%v", spec.Name, modeSpec.Optional, spec.Optional)
		}
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Every mode must eventually say what its RESPONSE CONTAINS, not only what it
// does. A Hint is not a contract: "List captured browser session recordings"
// and "List saved browser recording videos" were indistinguishable, and neither
// disclosed that a recordings entry carried every captured action — so the
// listing shipped its whole corpus to answer a question about names.
//
// This is a ratchet. The count of modes without a stated response may only
// shrink; a new mode must state one.
func TestModesWithoutAStatedResponseOnlyShrink(t *testing.T) {
	const baseline = 0

	missing := []string{}
	for tool, specs := range toolModeSpecs {
		for mode, spec := range specs {
			if spec.Returns == "" {
				missing = append(missing, tool+"/"+mode)
			}
		}
	}
	sort.Strings(missing)

	if len(missing) > baseline {
		t.Fatalf("%d modes have no stated response, above the baseline of %d. A new mode must set Returns: what fields come back, and anything a reader would expect but will not find.\n%s",
			len(missing), baseline, strings.Join(missing, "\n"))
	}
	// Baseline is zero: every mode states what it returns, and a new one must
	// too. There is nothing left to ratchet down.
}

// A stated response must describe the payload, not restate the action.
func TestStatedResponsesNameTheirFields(t *testing.T) {
	for tool, specs := range toolModeSpecs {
		for mode, spec := range specs {
			if spec.Returns == "" {
				continue
			}
			// Plain English, not a field dump: a sentence saying what KIND of
			// thing comes back. "The current telemetry mode." is a complete
			// answer; length is not the measure. A leading "field[]:" is the
			// schema restated, which the input schema already carries.
			if spec.Returns == spec.Hint {
				t.Errorf("%s/%s Returns just repeats Hint; it must say what comes back", tool, mode)
			}
			if !strings.HasSuffix(spec.Returns, ".") {
				t.Errorf("%s/%s Returns must read as a sentence: %q", tool, mode, spec.Returns)
			}
			if matched, _ := regexp.MatchString(`^[a-z_]+\[\]:`, spec.Returns); matched {
				t.Errorf("%s/%s Returns is a field dump, not plain English: %q", tool, mode, spec.Returns)
			}
		}
	}
}
