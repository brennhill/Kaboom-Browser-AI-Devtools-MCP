// Purpose: Reads raw JSON tool schemas — dispatch parameter, mode enum, and per-parameter metadata.
// Why: All schema introspection lives here so the assembly code in capabilities.go
// never touches raw schema maps. Dispatch inference and parameter-detail extraction
// share this file because both decode schema values through toStringSlice.
package capabilities

import (
	"regexp"
	"strings"
)

// inferDispatchParam selects the canonical mode/action parameter for a tool.
// Primary source is schema.required[0]. For alias-friendly schemas that use
// anyOf/oneOf instead of a top-level required field, fall back to the first
// anyOf/oneOf branch whose required param has an enum in props.
func inferDispatchParam(inputSchema map[string]any) string {
	props, _ := inputSchema["properties"].(map[string]any)
	required := toStringSlice(inputSchema["required"])
	if len(required) > 0 {
		return required[0]
	}
	// Defensive fallback: schemas must not use top-level anyOf/oneOf (invariant
	// enforced by TestAllToolSchemas_NoTopLevelCombiners), but external tool
	// schemas passed to BuildCapabilitiesMap/BuildCapabilitiesSummary may predate
	// that constraint. Return the first branch whose required param has an enum.
	for _, combinerKey := range []string{"anyOf", "oneOf"} {
		combinerRaw, ok := inputSchema[combinerKey]
		if !ok {
			continue
		}
		var branches []map[string]any
		switch v := combinerRaw.(type) {
		case []map[string]any:
			branches = v
		case []any:
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					branches = append(branches, m)
				}
			}
		}
		for _, branch := range branches {
			branchRequired := toStringSlice(branch["required"])
			if len(branchRequired) == 0 {
				continue
			}
			candidate := branchRequired[0]
			if prop, ok := props[candidate].(map[string]any); ok {
				if _, hasEnum := prop["enum"]; hasEnum {
					return candidate
				}
			}
		}
	}
	return ""
}

func extractModes(dispatchParam string, props map[string]any) []string {
	if dispatchParam == "" {
		return nil
	}
	prop, ok := props[dispatchParam].(map[string]any)
	if !ok {
		return nil
	}
	return toStringSlice(prop["enum"])
}

// toStringSlice converts a raw schema value to a string slice.
// Handles both []string (Go-constructed schemas) and []any (JSON-unmarshaled schemas).
// Empty strings are silently dropped — schema fields must not contain blank entries.
func toStringSlice(raw any) []string {
	switch typed := raw.(type) {
	case []string:
		return append([]string{}, typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

var (
	defaultParenPattern = regexp.MustCompile(`(?i)\(default[:\s]*([^)]+)\)`)
	defaultsToPattern   = regexp.MustCompile(`(?i)defaults?\s+to\s+([a-zA-Z0-9_./:-]+)`)
)

func buildParamDetails(props map[string]any) map[string]any {
	details := make(map[string]any, len(props))
	for name, propRaw := range props {
		prop, ok := propRaw.(map[string]any)
		if !ok {
			continue
		}
		meta := map[string]any{}

		if typ, ok := prop["type"].(string); ok && typ != "" {
			meta["type"] = typ
		}

		if enumVals := toStringSlice(prop["enum"]); len(enumVals) > 0 {
			meta["enum"] = enumVals
		}

		if desc, ok := prop["description"].(string); ok && desc != "" {
			meta["description"] = desc
			if _, hasDefault := meta["default"]; !hasDefault {
				if parsedDefault, ok := extractDefaultFromDescription(desc); ok {
					meta["default"] = parsedDefault
				}
			}
		}

		if explicitDefault, ok := prop["default"]; ok {
			meta["default"] = explicitDefault
		}

		if items, ok := prop["items"].(map[string]any); ok {
			if itemType, ok := items["type"].(string); ok && itemType != "" {
				meta["item_type"] = itemType
			}
		}

		if len(meta) > 0 {
			details[name] = meta
		}
	}
	return details
}

func extractDefaultFromDescription(description string) (string, bool) {
	if description == "" {
		return "", false
	}
	if match := defaultParenPattern.FindStringSubmatch(description); len(match) == 2 {
		return cleanDefaultText(match[1]), true
	}
	if match := defaultsToPattern.FindStringSubmatch(description); len(match) == 2 {
		return cleanDefaultText(match[1]), true
	}
	return "", false
}

func cleanDefaultText(v string) string {
	trimmed := strings.TrimSpace(v)
	trimmed = strings.Trim(trimmed, "`'\"")
	trimmed = strings.TrimRight(trimmed, ".,;")
	return trimmed
}
