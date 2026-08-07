// postprocess_test.go — Tests shared MCP tool-result post-processing.

package toolpostprocess

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestApplyAddsUnknownParameterWarningsToSuccessfulResults(t *testing.T) {
	t.Parallel()
	response := mcp.JSONRPCResponse{Result: json.RawMessage(`{"content":[{"type":"text","text":"ok"}]}`)}
	schema := map[string]any{"type": "object", "properties": map[string]any{"what": map[string]any{"type": "string"}}}

	processed, isError := Apply(response, json.RawMessage(`{"what":"errors","unexpected":true}`), schema)
	if isError {
		t.Fatal("successful result classified as an error")
	}
	if !strings.Contains(string(processed.Result), "unknown parameter 'unexpected'") {
		t.Fatalf("warning missing from result: %s", processed.Result)
	}
}

func TestApplyPreservesErrorsAndMalformedResults(t *testing.T) {
	t.Parallel()
	schema := map[string]any{"type": "object", "properties": map[string]any{}}
	for _, test := range []struct {
		name    string
		result  json.RawMessage
		isError bool
	}{
		{name: "tool error", result: json.RawMessage(`{"isError":true,"content":[{"type":"text","text":"failed"}]}`), isError: true},
		{name: "malformed", result: json.RawMessage(`not-json`)},
		{name: "absent"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := mcp.JSONRPCResponse{Result: test.result}
			processed, isError := Apply(response, json.RawMessage(`{"unexpected":true}`), schema)
			if isError != test.isError {
				t.Fatalf("isError = %t, want %t", isError, test.isError)
			}
			if string(processed.Result) != string(test.result) {
				t.Fatalf("result changed: %q", processed.Result)
			}
		})
	}
}
