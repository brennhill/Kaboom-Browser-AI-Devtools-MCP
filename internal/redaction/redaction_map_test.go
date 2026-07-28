// redaction_map_test.go — Tests structured MCP and JSON redaction.
// Docs: docs/features/feature/redaction-patterns/index.md

package redaction

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// MCP Response Integration Tests
// ============================================

func TestRedactMCPToolResult(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	// Simulate an MCP tool result with sensitive content
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: `{"headers": {"Authorization": "Bearer secret123abc"}, "body": "SSN: 123-45-6789"}`},
		},
	}
	resultJSON, _ := json.Marshal(result)

	redacted := engine.RedactJSON(resultJSON)

	var redactedResult MCPToolResult
	if err := json.Unmarshal(redacted, &redactedResult); err != nil {
		t.Fatalf("Redacted JSON should be valid: %v", err)
	}

	text := redactedResult.Content[0].Text
	if strings.Contains(text, "secret123abc") {
		t.Errorf("Bearer token should be redacted from MCP result, got: %s", text)
	}
	if strings.Contains(text, "123-45-6789") {
		t.Errorf("SSN should be redacted from MCP result, got: %s", text)
	}
	if !strings.Contains(text, "[REDACTED:bearer-token]") {
		t.Errorf("Expected bearer-token redaction marker in: %s", text)
	}
	if !strings.Contains(text, "[REDACTED:ssn]") {
		t.Errorf("Expected ssn redaction marker in: %s", text)
	}
}

func TestRedactJSONPreservesStructure(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	// Non-sensitive JSON should pass through structurally intact
	input := `{"content":[{"type":"text","text":"Hello world, no secrets here"}]}`
	got := engine.RedactJSON(json.RawMessage(input))

	var result MCPToolResult
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("Output should be valid JSON: %v", err)
	}
	if result.Content[0].Text != "Hello world, no secrets here" {
		t.Errorf("Non-sensitive content should be unchanged, got: %s", result.Content[0].Text)
	}
}

func TestRedactJSONMultipleContentBlocks(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: "Bearer token_one"},
			{Type: "text", Text: "SSN: 999-88-7777"},
			{Type: "text", Text: "No secrets here"},
		},
	}
	resultJSON, _ := json.Marshal(result)

	redacted := engine.RedactJSON(resultJSON)
	var redactedResult MCPToolResult
	if err := json.Unmarshal(redacted, &redactedResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if !strings.Contains(redactedResult.Content[0].Text, "[REDACTED:bearer-token]") {
		t.Errorf("First block should be redacted: %s", redactedResult.Content[0].Text)
	}
	if !strings.Contains(redactedResult.Content[1].Text, "[REDACTED:ssn]") {
		t.Errorf("Second block should be redacted: %s", redactedResult.Content[1].Text)
	}
	if redactedResult.Content[2].Text != "No secrets here" {
		t.Errorf("Third block should be unchanged: %s", redactedResult.Content[2].Text)
	}
}
