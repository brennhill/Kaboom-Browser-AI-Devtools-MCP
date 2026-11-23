// reproselector.go — Maps interact DOM actions and selector strings onto reproduction step shapes.
// Docs: docs/features/feature/reproduction-scripts/index.md

package actionlog

import "strings"

// parseSelectorForReproduction converts an interact-tool selector string into
// a selectors map compatible with the reproduction formatter.
// Handles semantic selectors (text=Submit, role=button) and CSS selectors.
func parseSelectorForReproduction(selector string) map[string]any {
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

// reproTypeForDOMAction maps an interact DOM action name to its reproduction
// step type. Actions without a mapping report false and are recorded as-is
// (with a "dom_" prefix for the audit trail).
func reproTypeForDOMAction(action string) (string, bool) {
	switch action {
	case "click", "check":
		return "click", true
	case "type":
		return "input", true
	case "select":
		return "select", true
	case "key_press":
		return "keypress", true
	case "scroll_to":
		return "scroll_element", true
	case "focus":
		return "focus", true
	case "hover":
		return "hover", true
	default:
		return "", false
	}
}
