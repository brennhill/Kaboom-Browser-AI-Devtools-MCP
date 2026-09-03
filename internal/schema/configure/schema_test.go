// schema_test.go — Invariants for the configure tool schema and its property groups.
package configure

import (
	"reflect"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

// TestToolSchema_RequiresWhat verifies canonical routing while retaining action
// only as a mode-specific sub-action field.
func TestToolSchema_RequiresWhat(t *testing.T) {
	t.Parallel()

	tool := ToolSchema()
	props, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("configure schema missing properties")
	}

	whatProp, ok := props["what"].(map[string]any)
	if !ok {
		t.Fatal("configure schema missing 'what' property")
	}
	whatEnum, ok := whatProp["enum"].([]string)
	if !ok || len(whatEnum) == 0 {
		t.Fatalf("configure 'what' enum = %#v, want non-empty []string", whatProp["enum"])
	}

	actionProp, ok := props["action"].(map[string]any)
	if !ok {
		t.Fatal("configure schema missing mode-specific 'action' property")
	}
	if _, hasEnum := actionProp["enum"]; hasEnum {
		t.Fatal("configure mode-specific 'action' must not define a top-level routing enum")
	}

	required, ok := tool.InputSchema["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "what" {
		t.Fatalf("configure schema required = %#v, want [what]", tool.InputSchema["required"])
	}
}

func TestToolSchemaExposesAtomicQAFixtureApplyContract(t *testing.T) {
	props := ToolSchema().InputSchema["properties"].(map[string]any)
	what := props["what"].(map[string]any)["enum"].([]string)
	if !contains(what, "qa_fixture") {
		t.Fatal("configure what enum missing qa_fixture")
	}
	actions := props["fixture_action"].(map[string]any)["enum"].([]string)
	if len(actions) != 4 || actions[0] != "validate" || actions[1] != "apply" || actions[2] != "status" || actions[3] != "restore" {
		t.Fatalf("fixture_action enum = %v, want validate, apply, status, and restore", actions)
	}
	if _, ok := props["transaction_id"]; !ok {
		t.Fatal("configure schema missing fixture transaction_id")
	}
	fixture := props["fixture"].(map[string]any)
	if fixture["additionalProperties"] != false {
		t.Fatal("fixture schema must reject unknown fields")
	}
}

func TestToolSchemaExposesSessionDiffAuditAndTelemetryContracts(t *testing.T) {
	t.Parallel()
	props := ToolSchema().InputSchema["properties"].(map[string]any)
	for _, field := range []string{"url", "operation", "telemetry_mode"} {
		if _, ok := props[field]; !ok {
			t.Errorf("configure schema missing %q", field)
		}
	}
	operations, ok := props["operation"].(map[string]any)["enum"].([]string)
	if !ok {
		t.Fatalf("operation enum has unexpected shape: %#v", props["operation"])
	}
	for _, operation := range []string{"analyze", "report", "clear"} {
		if !contains(operations, operation) {
			t.Errorf("operation enum missing %q: %v", operation, operations)
		}
	}
}

func TestPerformanceBudgetSchemaMatchesRuntimeMetrics(t *testing.T) {
	t.Parallel()

	properties := ToolSchema().InputSchema["properties"].(map[string]any)
	budgets := properties["performance_budgets"].(map[string]any)
	propertyNames := budgets["propertyNames"].(map[string]any)
	got := propertyNames["enum"].([]string)
	want := performance.BudgetMetricNames()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("performance budget schema metrics = %v, runtime accepts %v", got, want)
	}
	if budgets["additionalProperties"] == true {
		t.Fatal("performance budget schema must constrain property names")
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// TestToolProperties_MergePreservesEveryGroupKey guards the merge in
// properties.go: both property groups must survive intact. A collision between
// the two groups would silently drop one definition, and a merge that skipped
// existing keys would silently drop the other — neither is visible from the
// assembled schema alone.
func TestToolProperties_MergePreservesEveryGroupKey(t *testing.T) {
	t.Parallel()

	core := coreProperties()
	runtime := runtimeProperties()
	fixture := fixtureProperties()
	consent := consentProperties()
	merged := toolProperties()

	groups := map[string]map[string]any{"core": core, "runtime": runtime, "fixture": fixture, "consent": consent}
	owners := make(map[string]string)
	for name, group := range groups {
		for key := range group {
			if prior, dup := owners[key]; dup {
				t.Errorf("property %q defined in BOTH %s and %s groups — one definition wins silently", key, prior, name)
			}
			owners[key] = name
		}
	}

	for name, group := range groups {
		for key, want := range group {
			got, ok := merged[key]
			if !ok {
				t.Errorf("%s property %q missing from merged configure properties", name, key)
				continue
			}
			gotMap, gotOK := got.(map[string]any)
			wantMap, wantOK := want.(map[string]any)
			if !gotOK || !wantOK {
				t.Errorf("property %q: unexpected shape %T (group %T)", key, got, want)
				continue
			}
			if gotMap["description"] != wantMap["description"] {
				t.Errorf("property %q: merged description = %v, want %v (from %s group)",
					key, gotMap["description"], wantMap["description"], name)
			}
		}
	}

	want := len(core) + len(runtime) + len(fixture) + len(consent)
	if len(merged) != want {
		t.Errorf("merged configure properties = %d, want %d (core %d + runtime %d + fixture %d + consent %d)",
			len(merged), want, len(core), len(runtime), len(fixture), len(consent))
	}
}
