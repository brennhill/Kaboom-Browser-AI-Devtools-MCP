// Purpose: Coverage tests for OpenAPI stub emission helpers (mapToOpenAPIType, intToString).
// Docs: docs/features/feature/api-schema/index.md

package apischema

import (
	"testing"
)

// ============================================
// mapToOpenAPIType Coverage Tests
// ============================================

func TestMapToOpenAPITypeInteger(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("integer")
	if result != "integer" {
		t.Errorf("Expected 'integer', got: %s", result)
	}
}

func TestMapToOpenAPITypeNumber(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("number")
	if result != "number" {
		t.Errorf("Expected 'number', got: %s", result)
	}
}

func TestMapToOpenAPITypeBoolean(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("boolean")
	if result != "boolean" {
		t.Errorf("Expected 'boolean', got: %s", result)
	}
}

func TestMapToOpenAPITypeArray(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("array")
	if result != "array" {
		t.Errorf("Expected 'array', got: %s", result)
	}
}

func TestMapToOpenAPITypeObject(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("object")
	if result != "object" {
		t.Errorf("Expected 'object', got: %s", result)
	}
}

func TestMapToOpenAPITypeUUID(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("uuid")
	if result != "string" {
		t.Errorf("Expected 'string' for uuid, got: %s", result)
	}
}

func TestMapToOpenAPITypeDefault(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("something_unknown")
	if result != "string" {
		t.Errorf("Expected 'string' for default, got: %s", result)
	}
}

func TestMapToOpenAPITypeEmptyString(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("")
	if result != "string" {
		t.Errorf("Expected 'string' for empty input, got: %s", result)
	}
}

// ============================================
// intToString Coverage Tests
// ============================================

func TestIntToStringZero(t *testing.T) {
	t.Parallel()
	result := intToString(0)
	if result != "0" {
		t.Errorf("Expected '0', got: %s", result)
	}
}

func TestIntToStringPositive(t *testing.T) {
	t.Parallel()
	result := intToString(200)
	if result != "200" {
		t.Errorf("Expected '200', got: %s", result)
	}
}

func TestIntToStringLargePositive(t *testing.T) {
	t.Parallel()
	result := intToString(12345)
	if result != "12345" {
		t.Errorf("Expected '12345', got: %s", result)
	}
}

func TestIntToStringNegative(t *testing.T) {
	t.Parallel()
	result := intToString(-42)
	if result != "-42" {
		t.Errorf("Expected '-42', got: %s", result)
	}
}

func TestIntToStringLargeNegative(t *testing.T) {
	t.Parallel()
	result := intToString(-999)
	if result != "-999" {
		t.Errorf("Expected '-999', got: %s", result)
	}
}

func TestIntToStringSingleDigit(t *testing.T) {
	t.Parallel()
	result := intToString(7)
	if result != "7" {
		t.Errorf("Expected '7', got: %s", result)
	}
}

func TestIntToStringStatusCodes(t *testing.T) {
	t.Parallel()
	// Test common HTTP status codes that might be used in the codebase
	tests := []struct {
		input    int
		expected string
	}{
		{200, "200"},
		{201, "201"},
		{301, "301"},
		{400, "400"},
		{401, "401"},
		{403, "403"},
		{404, "404"},
		{500, "500"},
		{502, "502"},
		{503, "503"},
	}
	for _, tt := range tests {
		result := intToString(tt.input)
		if result != tt.expected {
			t.Errorf("intToString(%d) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}
