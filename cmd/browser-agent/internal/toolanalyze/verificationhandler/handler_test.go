// handler_test.go — analyze verification contract handler tests.

package verificationhandler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestHandleDefinesValidVersionedContract(t *testing.T) {
	response := Handle(request(), json.RawMessage(`{
		"what":"verification",
		"operation":"define",
		"contract":{"schema_version":"1","contract_id":"checkout","assertions":[{"assertion_id":"total","description":"total is visible","required_evidence":["dom"]}]}
	}`))
	isError, text := responseText(t, response)
	if isError {
		t.Fatalf("unexpected error: %s", text)
	}
	for _, want := range []string{`"schema_version":"1"`, `"contract_id":"checkout"`, `"status":"defined"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("response missing %s: %s", want, text)
		}
	}
}

func TestHandleEvaluatesContractAndRejectsInvalidOperation(t *testing.T) {
	args := json.RawMessage(`{
		"what":"verification",
		"operation":"evaluate",
		"contract":{"schema_version":"1","contract_id":"checkout","assertions":[{"assertion_id":"total","description":"total is visible","required_evidence":["dom"]}]},
		"results":[{"assertion_id":"total","verdict":"PASS","evidence":[{"evidence_id":"dom-1","kind":"dom"}]}]
	}`)
	isError, text := responseText(t, Handle(request(), args))
	if isError || !strings.Contains(text, `"verdict":"PASS"`) {
		t.Fatalf("unexpected evaluation response: error=%v text=%s", isError, text)
	}

	isError, _ = responseText(t, Handle(request(), json.RawMessage(`{"operation":"guess"}`)))
	if !isError {
		t.Fatal("unknown operation should fail")
	}
}

func TestHandleRejectsInvalidContract(t *testing.T) {
	isError, text := responseText(t, Handle(request(), json.RawMessage(`{"operation":"define","contract":{"schema_version":"1","contract_id":"empty"}}`)))
	if !isError || !strings.Contains(text, "assertion") {
		t.Fatalf("invalid contract response: error=%v text=%s", isError, text)
	}
}

func request() mcp.JSONRPCRequest { return mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1} }

func responseText(t *testing.T, response mcp.JSONRPCResponse) (bool, string) {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content blocks = %d, want 1", len(result.Content))
	}
	return result.IsError, result.Content[0].Text
}
