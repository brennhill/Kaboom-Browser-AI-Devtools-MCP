// Purpose: Property-based tests for redaction invariants.
// Docs: docs/features/feature/redaction-patterns/index.md

// redaction_property_test.go — Property-based tests for redaction engine.

package redaction

import (
	"encoding/json"
	"testing"
	"testing/quick"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// TestPropertyRedactIdempotent verifies that Redact(Redact(s)) == Redact(s) for all strings.
func TestPropertyRedactIdempotent(t *testing.T) {
	engine := NewRedactionEngine("")

	f := func(s string) bool {
		first := engine.Redact(s)
		second := engine.Redact(first)
		return first == second
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyRedactJSONStructuralPreservation verifies that RedactJSON preserves JSON structure.
// Constructs a valid MCPToolResult from random text, marshals to JSON, redacts it,
// and verifies the result is still valid JSON that unmarshals to MCPToolResult.
func TestPropertyRedactJSONStructuralPreservation(t *testing.T) {
	engine := NewRedactionEngine("")

	f := func(text1, text2, text3 string) bool {
		// Construct a valid MCPToolResult from random text
		result := mcp.MCPToolResult{
			Content: []mcp.MCPContentBlock{
				{Type: "text", Text: text1},
				{Type: "text", Text: text2},
				{Type: "text", Text: text3},
			},
		}

		// Marshal to JSON
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			return false
		}

		// Redact the JSON
		redacted := engine.RedactJSON(json.RawMessage(jsonBytes))

		// Verify it's still valid JSON by unmarshaling
		var parsed mcp.MCPToolResult
		if err := json.Unmarshal([]byte(redacted), &parsed); err != nil {
			return false
		}

		// Verify structure is preserved (same number of content items)
		if len(parsed.Content) != len(result.Content) {
			return false
		}

		// Verify all content items have the expected type
		for _, content := range parsed.Content {
			if content.Type != "text" {
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyLuhnDeterminism verifies that luhnValid always returns the same result
// for the same input.
func TestPropertyLuhnDeterminism(t *testing.T) {
	f := func(s string) bool {
		// Call luhnValid twice with the same input
		first := luhnValid(s)
		second := luhnValid(s)

		// Results must be identical
		return first == second
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyRedactLengthBound verifies that redaction never produces output
// longer than input + some reasonable overhead for replacement tokens.
func TestPropertyRedactLengthBound(t *testing.T) {
	engine := NewRedactionEngine("")

	f := func(s string) bool {
		redacted := engine.Redact(s)
		// Redaction should not massively inflate the string.
		// Allow up to 10x growth to account for replacement patterns.
		maxLen := len(s)*10 + 1000
		return len(redacted) <= maxLen
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyRedactEmptyString verifies that redacting an empty string returns empty string.
func TestPropertyRedactEmptyString(t *testing.T) {
	engine := NewRedactionEngine("")

	result := engine.Redact("")
	if result != "" {
		t.Errorf("Redact(\"\") = %q, want \"\"", result)
	}
}

// TestPropertyLuhnValidEmptyString verifies luhnValid behavior on empty input.
func TestPropertyLuhnValidEmptyString(t *testing.T) {
	result := luhnValid("")
	if result {
		t.Error("luhnValid(\"\") = true, want false")
	}
}

// TestPropertyRedactJSONEmptyObject verifies RedactJSON preserves empty JSON objects.
func TestPropertyRedactJSONEmptyObject(t *testing.T) {
	engine := NewRedactionEngine("")

	inputs := []string{
		"{}",
		"[]",
		`{"content":[]}`,
	}

	for _, input := range inputs {
		redacted := engine.RedactJSON(json.RawMessage(input))

		// Verify it's still valid JSON
		var parsed any
		if err := json.Unmarshal([]byte(redacted), &parsed); err != nil {
			t.Errorf("RedactJSON(%q) produced invalid JSON: %v", input, err)
		}
	}
}
