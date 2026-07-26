// Purpose: Infers JSON schema types and formats from observed API response values.
// Why: Separates type inference heuristics (UUID, datetime, number ranges) from the main schema builder.
// Docs: docs/features/feature/api-schema/index.md

package apischema

import (
	"math"
	"regexp"
	"strings"
)

// ============================================
// Type and Format Inference
// ============================================

var (
	uuidValuePattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	datetimeValuePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`)
)

func inferTypeAndFormat(value any) (string, string) {
	switch v := value.(type) {
	case nil:
		return "null", ""
	case bool:
		return "boolean", ""
	case float64:
		if v == math.Trunc(v) {
			return "integer", ""
		}
		return "number", ""
	case string:
		format := inferStringFormat(v)
		return "string", format
	case []any:
		return "array", ""
	case map[string]any:
		return "object", ""
	default:
		return "string", ""
	}
}

func inferStringFormat(v string) string {
	if uuidValuePattern.MatchString(v) {
		return "uuid"
	}
	if datetimeValuePattern.MatchString(v) {
		return "datetime"
	}
	if strings.Contains(v, "@") && strings.Contains(v, ".") {
		return "email"
	}
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return "url"
	}
	return ""
}

func isJSONContentType(ct string) bool {
	if ct == "" {
		// If no content type, try to parse as JSON anyway
		return true
	}
	return strings.Contains(ct, "json")
}
