// redaction_map.go — Owns sensitive-key matching and recursive structured-value redaction.
package redaction

import "strings"

// sensitiveKeyNames matches key names that indicate sensitive data.
// Values for these keys are always redacted regardless of content.
var sensitiveKeyNames = map[string]bool{
	"password":   true,
	"passwd":     true,
	"secret":     true,
	"token":      true,
	"ssn":        true,
	"creditcard": true,
	"cvv":        true,
	"cvc":        true,
	"auth":       true,
	"credential": true,
	"apikey":     true,
	"passcode":   true,
	"session":    true,
	"cookie":     true,
	"bearer":     true,
	"otp":        true,
}

// sensitiveKeyFragments catches common key-name variants (snake_case, kebab-case, camelCase).
var sensitiveKeyFragments = []string{
	"password",
	"passwd",
	"passcode",
	"token",
	"secret",
	"apikey",
	"auth",
	"credential",
	"session",
	"cookie",
	"bearer",
	"otp",
	"ssn",
	"creditcard",
	"cvv",
	"cvc",
}

func normalizeSensitiveKeyName(key string) string {
	key = strings.ToLower(key)
	var b strings.Builder
	b.Grow(len(key))
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isSensitiveKeyName(key string) bool {
	normalized := normalizeSensitiveKeyName(key)
	if normalized == "" {
		return false
	}
	if sensitiveKeyNames[normalized] {
		return true
	}
	for _, fragment := range sensitiveKeyFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

// RedactMapValues walks a map recursively and redacts sensitive data.
// String values are run through Redact() for pattern matching.
// Keys matching sensitiveKeyNames have their values replaced entirely.
// Nested maps are recursed. Non-string, non-map values pass through unchanged.
// Returns a new map; the input is not modified.
func (e *RedactionEngine) RedactMapValues(data map[string]any) map[string]any {
	out := make(map[string]any, len(data))
	for k, v := range data {
		out[k] = e.redactValue(k, v)
	}
	return out
}

func (e *RedactionEngine) redactValue(key string, value any) any {
	// Check sensitive key name first
	if isSensitiveKeyName(key) {
		if preservesStateContainerValues(key) {
			switch current := value.(type) {
			case map[string]any:
				return e.RedactMapValues(current)
			case []any:
				out := make([]any, len(current))
				for index, child := range current {
					out[index] = e.redactValue("", child)
				}
				return out
			}
		}
		return redactSensitiveContainer(value, "[REDACTED:key-"+normalizeSensitiveKeyName(key)+"]")
	}

	switch v := value.(type) {
	case string:
		return e.Redact(v)
	case map[string]any:
		return e.RedactMapValues(v)
	case []any:
		out := make([]any, len(v))
		for i, elem := range v {
			out[i] = e.redactValue("", elem)
		}
		return out
	default:
		return value
	}
}

func preservesStateContainerValues(key string) bool {
	normalized := normalizeSensitiveKeyName(key)
	return normalized == "sessionstorage" || normalized == "cookies"
}

func redactSensitiveContainer(value any, replacement string) any {
	switch current := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(current))
		for key, child := range current {
			out[key] = redactSensitiveContainer(child, replacement)
		}
		return out
	case []any:
		out := make([]any, len(current))
		for index, child := range current {
			out[index] = redactSensitiveContainer(child, replacement)
		}
		return out
	default:
		return replacement
	}
}
