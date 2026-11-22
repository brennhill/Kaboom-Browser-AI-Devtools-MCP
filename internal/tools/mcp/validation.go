// Purpose: Extracts JSON field names via reflection and validates/warns on unknown tool parameters.
// Docs: docs/features/feature/query-service/index.md

package mcp

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// getJSONFieldNames uses reflection to extract the set of known JSON field names
// from a struct's json tags. Fields without a json tag use their Go field name.
// Fields tagged with json:"-" are excluded.
func getJSONFieldNames(v any) map[string]bool {
	known := make(map[string]bool)
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return known
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}
		if tag == "" {
			known[field.Name] = true
			continue
		}
		// Strip options like ",omitempty"
		name := strings.Split(tag, ",")[0]
		if name != "" {
			known[name] = true
		}
	}
	return known
}

// UnmarshalWithWarnings unmarshals JSON into a struct and returns warnings for
// any unknown top-level fields. This helps LLMs discover misspelled parameters.
func UnmarshalWithWarnings(data json.RawMessage, v any) ([]string, error) {
	if err := json.Unmarshal(data, v); err != nil {
		return nil, err
	}
	// Check for unknown fields by unmarshaling into a map
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil // Can't check, skip warnings
	}
	known := getJSONFieldNames(v)
	var warnings []string
	for k := range raw {
		if !known[k] {
			warnings = append(warnings, fmt.Sprintf("unknown parameter '%s' (ignored)", k))
		}
	}
	return warnings, nil
}

// ValidateParamsAgainstSchema checks incoming JSON keys against a tool's known
// property names from its InputSchema. Returns warnings for unknown fields.
// This validates at the tool level (not handler level), catching typos across
// all parameters defined in the tool's schema.
func ValidateParamsAgainstSchema(data json.RawMessage, schema map[string]any) []string {
	if len(data) == 0 {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil
	}

	var warnings []string
	for k := range raw {
		if _, known := props[k]; !known {
			warnings = append(warnings, fmt.Sprintf("unknown parameter '%s' (ignored)", k))
		}
	}
	return warnings
}
