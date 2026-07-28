// Purpose: Converts semantic selector strings (text=, role=, etc.) into structured selector maps for reproduction.
// Docs: docs/features/feature/interact-explore/index.md

package interact

import (
	"strings"
)

// ParseSelectorForReproduction converts an interact-tool selector string into
// a selectors map compatible with the reproduction formatter.
// Handles semantic selectors (text=Submit, role=button) and CSS selectors.
func ParseSelectorForReproduction(selector string) map[string]any {
	selectors := map[string]any{}
	if idx := strings.Index(selector, "="); idx > 0 {
		prefix := selector[:idx]
		value := selector[idx+1:]
		switch prefix {
		case "text":
			selectors["text"] = value
		case "role":
			selectors["role"] = map[string]any{"role": value}
		case "label", "aria-label":
			selectors["ariaLabel"] = value
		case "placeholder":
			selectors["ariaLabel"] = value
		default:
			selectors["cssPath"] = selector
		}
	} else {
		// Plain CSS selector — detect #id vs general CSS
		if strings.HasPrefix(selector, "#") && !strings.ContainsAny(selector[1:], " >.+~[]:#") {
			selectors["id"] = selector[1:]
		} else {
			selectors["cssPath"] = selector
		}
	}
	return selectors
}

// TruncateToLen returns s unchanged if shorter than maxLen, otherwise truncates with "...".
func TruncateToLen(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// DOMPrimitiveActions is the set of actions routed to the DOM primitive handler.
var DOMPrimitiveActions = map[string]bool{
	"click": true, "type": true, "select": true, "check": true,
	"get_text": true, "get_value": true, "get_attribute": true,
	"set_attribute": true, "focus": true, "scroll_to": true,
	"wait_for": true, "key_press": true, "paste": true,
	"open_composer": true, "submit_active_composer": true,
	"confirm_top_dialog": true, "dismiss_top_overlay": true,
	"hover": true,
	"query": true,
}

// DOMActionToReproType maps interact DOM action names to reproduction-compatible types.
// Actions not in this map are recorded as-is (with "dom_" prefix for audit trail).
var DOMActionToReproType = map[string]string{
	"click":     "click",
	"type":      "input",
	"select":    "select",
	"check":     "click",
	"key_press": "keypress",
	"scroll_to": "scroll_element",
	"focus":     "focus",
	"hover":     "hover",
}

// ValidWorldValues is the set of accepted values for the execute_js 'world' parameter.
var ValidWorldValues = map[string]bool{
	"auto": true, "main": true, "isolated": true,
}

// DOMActionRequiredParam describes a required parameter for a DOM action.
type DOMActionRequiredParam struct {
	Field   string
	Message string
	Retry   string
}

// DOMActionRequiredParams maps DOM actions to their required parameter name and error guidance.
var DOMActionRequiredParams = map[string]DOMActionRequiredParam{
	"type":          {"text", "Required parameter 'text' is missing for type action", "Add the 'text' parameter with the text to type"},
	"paste":         {"text", "Required parameter 'text' is missing for paste action", "Add the 'text' parameter with the text to paste"},
	"select":        {"value", "Required parameter 'value' is missing for select action", "Add the 'value' parameter with the option value to select"},
	"get_attribute": {"name", "Required parameter 'name' is missing for get_attribute action", "Add the 'name' parameter with the attribute name"},
	"set_attribute": {"name", "Required parameter 'name' is missing for set_attribute action", "Add the 'name' parameter with the attribute name"},
}

// ExtractElementList walks nested result JSON to find the elements array.
func ExtractElementList(data map[string]any) []any {
	// Direct elements field
	if elems, ok := data["elements"].([]any); ok {
		return elems
	}
	// Nested in result field (json.Unmarshal into map[string]any always produces map[string]any)
	if resultData, ok := data["result"].(map[string]any); ok {
		if elems, ok := resultData["elements"].([]any); ok {
			return elems
		}
		// Recurse into nested result (command result wrapping)
		return ExtractElementList(resultData)
	}
	return nil
}
