// Purpose: Tests for CSS and semantic selector resolution.
// Docs: docs/features/feature/interact-explore/index.md

// selector_test.go — Tests for selector parsing and DOM action utilities.
package interact

import (
	"testing"
)

func TestTruncateToLen_Short(t *testing.T) {
	t.Parallel()
	if got := TruncateToLen("short", 10); got != "short" {
		t.Errorf("TruncateToLen('short', 10) = %q, want 'short'", got)
	}
}

func TestTruncateToLen_Long(t *testing.T) {
	t.Parallel()
	if got := TruncateToLen("longstring", 4); got != "long..." {
		t.Errorf("TruncateToLen('longstring', 4) = %q, want 'long...'", got)
	}
}

func TestTruncateToLen_Empty(t *testing.T) {
	t.Parallel()
	if got := TruncateToLen("", 10); got != "" {
		t.Errorf("TruncateToLen('', 10) = %q, want ''", got)
	}
}

func TestExtractElementList_Direct(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"elements": []any{
			map[string]any{"index": float64(0), "selector": "#btn"},
		},
	}
	elems := ExtractElementList(data)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestExtractElementList_NestedResult(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"result": map[string]any{
			"elements": []any{
				map[string]any{"index": float64(0), "selector": "input"},
			},
		},
	}
	elems := ExtractElementList(data)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element from nested result, got %d", len(elems))
	}
}

func TestExtractElementList_DoubleNestedResult(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"result": map[string]any{
			"result": map[string]any{
				"elements": []any{
					map[string]any{"index": float64(0), "selector": "a"},
				},
			},
		},
	}
	elems := ExtractElementList(data)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element from double-nested result, got %d", len(elems))
	}
}

func TestExtractElementList_NoElements(t *testing.T) {
	t.Parallel()
	data := map[string]any{"status": "ok"}
	elems := ExtractElementList(data)
	if elems != nil {
		t.Errorf("expected nil for data without elements, got %v", elems)
	}
}

func TestDOMPrimitiveActions_Contains(t *testing.T) {
	t.Parallel()
	expected := []string{"click", "type", "select", "check", "get_text", "get_value",
		"get_attribute", "set_attribute", "focus", "scroll_to", "wait_for", "key_press", "paste",
		"open_composer", "submit_active_composer", "confirm_top_dialog", "dismiss_top_overlay",
		"hover"}
	for _, action := range expected {
		if !DOMPrimitiveActions[action] {
			t.Errorf("DOMPrimitiveActions missing %q", action)
		}
	}
}

func TestValidWorldValues(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"auto", "main", "isolated"} {
		if !ValidWorldValues[v] {
			t.Errorf("ValidWorldValues missing %q", v)
		}
	}
	if ValidWorldValues["invalid"] {
		t.Error("ValidWorldValues should not contain 'invalid'")
	}
}

func TestDOMActionRequiredParams_Structure(t *testing.T) {
	t.Parallel()
	if param, ok := DOMActionRequiredParams["type"]; !ok {
		t.Error("DOMActionRequiredParams missing 'type'")
	} else if param.Field != "text" {
		t.Errorf("type param field = %q, want 'text'", param.Field)
	}

	if param, ok := DOMActionRequiredParams["select"]; !ok {
		t.Error("DOMActionRequiredParams missing 'select'")
	} else if param.Field != "value" {
		t.Errorf("select param field = %q, want 'value'", param.Field)
	}

	if param, ok := DOMActionRequiredParams["get_attribute"]; !ok {
		t.Error("DOMActionRequiredParams missing 'get_attribute'")
	} else if param.Field != "name" {
		t.Errorf("get_attribute param field = %q, want 'name'", param.Field)
	}
}
