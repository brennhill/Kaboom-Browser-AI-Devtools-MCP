// forms_test.go — Tests inspection response shaping at its package boundary.

package inspect

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestBuildFormValidationSummary(t *testing.T) {
	t.Parallel()
	forms, _ := json.Marshal(map[string]any{"forms": []any{
		map[string]any{"valid": true}, map[string]any{"valid": false},
	}})
	result, _ := json.Marshal(mcp.MCPToolResult{Content: []mcp.MCPContentBlock{
		{Type: "text", Text: "Form validation results\n" + string(forms)},
	}})
	response := BuildFormValidationSummary(mcp.JSONRPCResponse{JSONRPC: "2.0", Result: result})

	var shaped mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &shaped); err != nil {
		t.Fatal(err)
	}
	if len(shaped.Content) != 1 || shaped.Content[0].Text == "" {
		t.Fatalf("unexpected summary: %#v", shaped.Content)
	}
	var summary map[string]any
	text := shaped.Content[0].Text
	for index := range text {
		if text[index] == '{' {
			if err := json.Unmarshal([]byte(text[index:]), &summary); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if summary["total_forms"] != float64(2) || summary["valid"] != float64(1) || summary["invalid"] != float64(1) {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestExtractFormsListNested(t *testing.T) {
	t.Parallel()
	forms := ExtractFormsList(map[string]any{"result": map[string]any{
		"forms": []any{map[string]any{"id": "login"}},
	}})
	if len(forms) != 1 {
		t.Fatalf("forms = %#v", forms)
	}
}
