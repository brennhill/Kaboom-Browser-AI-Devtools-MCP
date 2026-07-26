// Purpose: Unit tests for JSON type/format inference (inferTypeAndFormat, inferStringFormat, isJSONContentType).
// Docs: docs/features/feature/api-schema/index.md

package apischema

import (
	"math"
	"testing"
)

func TestInferTypeAndFormat(t *testing.T) {
	tests := []struct {
		name       string
		input      any
		wantType   string
		wantFormat string
	}{
		{"nil", nil, "null", ""},
		{"bool true", true, "boolean", ""},
		{"bool false", false, "boolean", ""},
		{"float64 integer", 42.0, "integer", ""},
		{"float64 zero", 0.0, "integer", ""},
		{"float64 negative integer", -5.0, "integer", ""},
		{"float64 non-integer", 3.14, "number", ""},
		{"float64 NaN", math.NaN(), "number", ""},
		{"float64 very large", 1e18, "integer", ""},
		{"string uuid", "550e8400-e29b-41d4-a716-446655440000", "string", "uuid"},
		{"string datetime", "2024-01-15T10:30:00Z", "string", "datetime"},
		{"string email", "user@example.com", "string", "email"},
		{"string url http", "http://example.com/path", "string", "url"},
		{"string url https", "https://example.com/path", "string", "url"},
		{"string plain", "hello world", "string", ""},
		{"string empty", "", "string", ""},
		{"array", []any{1, 2, 3}, "array", ""},
		{"array empty", []any{}, "array", ""},
		{"object", map[string]any{"key": "value"}, "object", ""},
		{"object empty", map[string]any{}, "object", ""},
		{"non-JSON type int", int(42), "string", ""},
		{"non-JSON type int64", int64(99), "string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotFormat := inferTypeAndFormat(tt.input)
			if gotType != tt.wantType || gotFormat != tt.wantFormat {
				t.Errorf("inferTypeAndFormat(%v) = (%q, %q), want (%q, %q)",
					tt.input, gotType, gotFormat, tt.wantType, tt.wantFormat)
			}
		})
	}
}

func TestInferStringFormat(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantFormat string
	}{
		{"uuid lowercase", "550e8400-e29b-41d4-a716-446655440000", "uuid"},
		{"uuid uppercase", "550E8400-E29B-41D4-A716-446655440000", "uuid"},
		{"uuid mixed case", "550e8400-E29B-41d4-a716-446655440000", "uuid"},
		{"datetime with Z", "2024-01-15T10:30:00Z", "datetime"},
		{"datetime with offset", "2024-01-15T10:30:00+05:00", "datetime"},
		{"datetime with millis", "2024-01-15T10:30:00.123Z", "datetime"},
		{"email simple", "user@example.com", "email"},
		{"email with plus", "user+tag@example.com", "email"},
		{"at but no dot", "user@localhost", ""},
		{"dot but no at", "not.an.email", ""},
		{"http url", "http://example.com", "url"},
		{"https url", "https://example.com/path?q=1", "url"},
		{"ftp not matched", "ftp://example.com", ""},
		{"plain string", "hello", ""},
		{"empty string", "", ""},
		{"numbers only", "12345", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferStringFormat(tt.input)
			if got != tt.wantFormat {
				t.Errorf("inferStringFormat(%q) = %q, want %q", tt.input, got, tt.wantFormat)
			}
		})
	}
}

// ============================================
// isJSONContentType — covers empty string branch
// ============================================

func TestIsJSONContentType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", true},
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"text/html", false},
		{"text/plain", false},
		{"application/xml", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isJSONContentType(tt.input); got != tt.want {
				t.Errorf("isJSONContentType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
