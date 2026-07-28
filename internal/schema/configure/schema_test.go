// schema_test.go — Invariants for the configure tool schema and its property groups.
package configure

import "testing"

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

// TestToolProperties_MergePreservesEveryGroupKey guards the merge in
// properties.go: both property groups must survive intact. A collision between
// the two groups would silently drop one definition, and a merge that skipped
// existing keys would silently drop the other — neither is visible from the
// assembled schema alone.
func TestToolProperties_MergePreservesEveryGroupKey(t *testing.T) {
	t.Parallel()

	core := coreProperties()
	runtime := runtimeProperties()
	merged := toolProperties()

	for key := range core {
		if _, dup := runtime[key]; dup {
			t.Errorf("property %q defined in BOTH core and runtime groups — one definition wins silently", key)
		}
	}

	for name, group := range map[string]map[string]any{"core": core, "runtime": runtime} {
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

	if len(merged) != len(core)+len(runtime) {
		t.Errorf("merged configure properties = %d, want %d (core %d + runtime %d)",
			len(merged), len(core)+len(runtime), len(core), len(runtime))
	}
}
