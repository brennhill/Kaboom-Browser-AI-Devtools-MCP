// flags.go — Shared CLI flag parsing primitives used by tool-specific command parsers.
// Why: Keeps flag decoding/validation logic DRY across observe/analyze/generate/configure/interact parsers.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package parser

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// --- Generic flag parser ---

// cliFlagKind defines the type of a CLI flag value.
type cliFlagKind int

const (
	FlagString cliFlagKind = iota
	FlagInt
	FlagBool
	FlagStringList
	FlagJSON
	FlagJSONOrString
	FlagIntOrString
)

// cliFlagSpec maps a CLI flag to an MCP argument key and its value type.
type cliFlagSpec struct {
	MCPKey string
	Kind   cliFlagKind
}

// parseFlagsBySpec parses CLI args against a spec map and returns MCP argument key-value pairs.
func parseFlagsBySpec(args []string, specs map[string]cliFlagSpec) (map[string]any, error) {
	out := make(map[string]any)
	for i := 0; i < len(args); i++ {
		flag := args[i]
		spec, ok := specs[flag]
		if !ok {
			return nil, fmt.Errorf("unknown flag: %s", flag)
		}
		switch spec.Kind {
		case FlagBool:
			out[spec.MCPKey] = true
		case FlagString:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.MCPKey] = val
			i = next
		case FlagInt:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			n, err := parseIntValue(val)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.MCPKey] = n
			i = next
		case FlagStringList:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.MCPKey] = parseCSVList(val)
			i = next
		case FlagJSON:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			var parsed any
			if err := json.Unmarshal([]byte(val), &parsed); err != nil {
				return nil, fmt.Errorf("%s: invalid JSON: %w", flag, err)
			}
			out[spec.MCPKey] = parsed
			i = next
		case FlagJSONOrString:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			out[spec.MCPKey] = parseJSONOrString(val)
			i = next
		case FlagIntOrString:
			val, next, err := requireFlagValue(args, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", flag, err)
			}
			if n, err := strconv.Atoi(val); err == nil {
				out[spec.MCPKey] = n
			} else {
				out[spec.MCPKey] = val
			}
			i = next
		default:
			return nil, fmt.Errorf("unsupported parser kind for %s", flag)
		}
	}
	return out, nil
}

// requireFlagValue returns the next arg as the flag's value, erroring if missing or another flag.
func requireFlagValue(args []string, idx int) (string, int, error) {
	next := idx + 1
	if next >= len(args) {
		return "", idx, fmt.Errorf("cli_parse: no value provided after flag. Add a value after the flag")
	}
	val := args[next]
	if strings.HasPrefix(val, "--") {
		return "", idx, fmt.Errorf("cli_parse: expected a value but got flag %q. Provide a value between the flags", val)
	}
	return val, next, nil
}

// parseIntValue parses a string as an integer.
func parseIntValue(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("expected integer, got %q: %w", s, err)
	}
	return n, nil
}

// parseCSVList splits a comma-separated string into a trimmed string slice.
func parseCSVList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

// parseJSONOrString attempts to parse s as JSON object; returns the raw string on failure.
func parseJSONOrString(s string) any {
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err == nil {
		return parsed
	}
	return s
}
