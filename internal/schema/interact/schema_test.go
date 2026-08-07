package interact

import (
	"reflect"
	"strings"
	"testing"
)

func TestInteractToolSchema_DescribesAutomaticPerformanceDiff(t *testing.T) {
	t.Parallel()

	description := strings.ToLower(ToolSchema().Description)
	if !strings.Contains(description, "perf_diff") || !strings.Contains(description, "timing") {
		t.Fatalf("interact description must advertise automatic perf_diff timing data: %q", description)
	}
}

func TestActionSpecHasNoCompatibilityMetadata(t *testing.T) {
	t.Parallel()

	if _, exists := reflect.TypeOf(ActionSpec{}).FieldByName("IsAlias"); exists {
		t.Fatal("ActionSpec retains compatibility-only IsAlias metadata")
	}
}

func TestInteractToolSchema_RequiresCanonicalWhat(t *testing.T) {
	t.Parallel()

	tool := ToolSchema()
	props, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing properties")
	}

	whatProp, ok := props["what"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing 'what' property")
	}
	if _, exists := props["action"]; exists {
		t.Fatal("interact schema must not expose an action selector alias")
	}

	whatEnum := toSchemaStringSlice(t, whatProp["enum"])
	if len(whatEnum) == 0 {
		t.Fatal("interact 'what' enum must be non-empty")
	}

	// Spot-check that well-known actions are present in the canonical 'what' enum.
	mustContain := []string{"navigate", "click", "type", "execute_js", "upload", "batch", "auto_dismiss_overlays", "wait_for_stable"}
	whatSet := make(map[string]bool, len(whatEnum))
	for _, v := range whatEnum {
		whatSet[v] = true
	}
	for _, name := range mustContain {
		if !whatSet[name] {
			t.Errorf("interact enum missing expected action %q", name)
		}
	}
	if whatSet["screenshot"] {
		t.Fatal("interact must not expose the observe screenshot mode as a compatibility action")
	}

	// Claude API forbids oneOf/allOf/anyOf at the top level of input_schema.
	// 'what' is the sole routing selector.
	if _, hasAnyOf := tool.InputSchema["anyOf"]; hasAnyOf {
		t.Fatal("interact schema must not use top-level anyOf (Claude API limitation)")
	}

	required := toSchemaStringSlice(t, tool.InputSchema["required"])
	if len(required) != 1 || required[0] != "what" {
		t.Fatalf("interact schema required = %v, want [what]", required)
	}
}

func TestInteractToolSchema_BatchStepsAreAnArray(t *testing.T) {
	t.Parallel()
	properties, ok := ToolSchema().InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing properties")
	}
	steps, ok := properties["steps"].(map[string]any)
	if !ok || steps["type"] != "array" {
		t.Fatalf("batch steps property = %#v", properties["steps"])
	}
}

func TestInteractToolSchema_AutoDismissParam(t *testing.T) {
	t.Parallel()
	tool := ToolSchema()
	props, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	adProp, ok := props["auto_dismiss"].(map[string]any)
	if !ok {
		t.Fatal("auto_dismiss param should be in interact schema properties")
	}
	if adProp["type"] != "boolean" {
		t.Errorf("auto_dismiss type = %v, want boolean", adProp["type"])
	}
}

func TestInteractToolSchema_StabilityMsParam(t *testing.T) {
	t.Parallel()
	tool := ToolSchema()
	props, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	smProp, ok := props["stability_ms"].(map[string]any)
	if !ok {
		t.Fatal("stability_ms param should be in interact schema properties")
	}
	if smProp["type"] != "number" {
		t.Errorf("stability_ms type = %v, want number", smProp["type"])
	}
}

func TestInteractToolSchema_IncludeScreenshotParam(t *testing.T) {
	t.Parallel()
	props, ok := ToolSchema().InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	property, ok := props["include_screenshot"].(map[string]any)
	if !ok || property["type"] != "boolean" {
		t.Fatalf("include_screenshot property = %#v", props["include_screenshot"])
	}
}

func TestInteractActionSpecs_EnumParity(t *testing.T) {
	t.Parallel()

	specs := ActionSpecs()
	if len(specs) == 0 {
		t.Fatal("ActionSpecs should be non-empty")
	}

	if len(actionEnum) != len(specs) {
		t.Fatalf("actionEnum/spec count mismatch: actions=%d specs=%d", len(actionEnum), len(specs))
	}

	for i, spec := range specs {
		if actionEnum[i] != spec.Name {
			t.Fatalf("interact action order mismatch at %d: enum=%q spec=%q", i, actionEnum[i], spec.Name)
		}
	}
}

func TestInteractActionSpecs_NoDuplicateNamesOrEmptyHints(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool, len(actionSpecs))
	for i, spec := range actionSpecs {
		if spec.Name == "" {
			t.Fatalf("actionSpecs[%d] has empty Name", i)
		}
		if seen[spec.Name] {
			t.Fatalf("duplicate interact action name: %q", spec.Name)
		}
		seen[spec.Name] = true
		if spec.Hint == "" {
			t.Fatalf("interact action %q missing capability hint", spec.Name)
		}
	}
}

func TestInteractActionSpecs_RequiredMatchesRuntimeValidation(t *testing.T) {
	t.Parallel()

	// Expected required params per action, derived from DOMActionRequiredParams
	// and handler validation in the codebase.
	expected := map[string][]string{
		"type":                  {"text"},
		"paste":                 {"text"},
		"select":                {"value"},
		"get_attribute":         {"name"},
		"set_attribute":         {"name"},
		"navigate":              {"url"},
		"navigate_and_wait_for": {"url", "wait_for"},
		"execute_js":            {"script"},
		"set_storage":           {"key"},
		"delete_storage":        {"key"},
		"set_cookie":            {"name"},
		"delete_cookie":         {"name"},
	}

	specs := ActionSpecs()
	specMap := make(map[string]ActionSpec, len(specs))
	for _, s := range specs {
		specMap[s.Name] = s
	}

	for action, wantRequired := range expected {
		spec, ok := specMap[action]
		if !ok {
			t.Errorf("missing spec for action %q", action)
			continue
		}
		if len(spec.Required) != len(wantRequired) {
			t.Errorf("%s: Required=%v, want %v", action, spec.Required, wantRequired)
			continue
		}
		for i, r := range wantRequired {
			if i >= len(spec.Required) || spec.Required[i] != r {
				t.Errorf("%s: Required[%d]=%q, want %q", action, i, spec.Required[i], r)
			}
		}
		// Verify required params are NOT in Optional
		optSet := make(map[string]bool, len(spec.Optional))
		for _, o := range spec.Optional {
			optSet[o] = true
		}
		for _, r := range wantRequired {
			if optSet[r] {
				t.Errorf("%s: %q appears in both Required and Optional", action, r)
			}
		}
	}
}

func TestInteractEnum_ExcludesAliases(t *testing.T) {
	t.Parallel()

	tool := ToolSchema()
	props, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing properties")
	}
	whatProp, ok := props["what"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing 'what' property")
	}
	whatEnum := toSchemaStringSlice(t, whatProp["enum"])
	enumSet := make(map[string]bool, len(whatEnum))
	for _, v := range whatEnum {
		enumSet[v] = true
	}

	aliases := []string{"state_save", "state_load", "state_list", "state_delete", "record_start", "record_stop"}
	for _, alias := range aliases {
		if enumSet[alias] {
			t.Errorf("alias %q should be excluded from interact enum", alias)
		}
	}

	// Canonical forms must still be present
	canonicals := []string{
		"save_state", "load_state", "list_states", "delete_state",
		"screen_recording_start", "screen_recording_stop",
	}
	for _, c := range canonicals {
		if !enumSet[c] {
			t.Errorf("canonical action %q should be in interact enum", c)
		}
	}
}

func TestInteractActionSpecs_ExcludeStateAliases(t *testing.T) {
	t.Parallel()

	specs := ActionSpecs()
	specNames := make(map[string]bool, len(specs))
	for _, s := range specs {
		specNames[s.Name] = true
	}

	aliases := []string{"state_save", "state_load", "state_list", "state_delete"}
	for _, alias := range aliases {
		if specNames[alias] {
			t.Errorf("duplicate state action %q must not be registered", alias)
		}
	}
}

func TestInteractActionSpecs_ExcludeRecordingAliases(t *testing.T) {
	t.Parallel()

	specs := ActionSpecs()
	specNames := make(map[string]bool, len(specs))
	for _, s := range specs {
		specNames[s.Name] = true
	}

	for _, alias := range []string{"record_start", "record_stop"} {
		if specNames[alias] {
			t.Errorf("duplicate recording action %q must not be registered", alias)
		}
	}
}

func toSchemaStringSlice(t *testing.T, raw any) []string {
	t.Helper()
	switch v := raw.(type) {
	case []string:
		out := make([]string, len(v))
		copy(out, v)
		return out
	case []any:
		out := make([]string, 0, len(v))
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				t.Fatalf("string slice item[%d] type = %T, want string", i, item)
			}
			out = append(out, s)
		}
		return out
	default:
		t.Fatalf("string slice type = %T, want []string or []any", raw)
		return nil
	}
}
