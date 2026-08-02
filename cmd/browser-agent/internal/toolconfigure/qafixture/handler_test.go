// handler_test.go — Tests configure QA fixture validation responses.

package qafixture

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestHandleValidateAcceptsVersionedFixtureWithoutEchoingState(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := Handle(req, json.RawMessage(`{
		"what":"qa_fixture",
		"fixture_action":"validate",
		"fixture":{"version":1,"local_storage":{"token":"private-value"}}
	}`))
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private-value") {
		t.Fatal("response leaked fixture state")
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, `"fixture_version":1`) || !strings.Contains(result.Content[0].Text, `"valid":true`) {
		t.Fatalf("response missing validation fields: %s", encoded)
	}
}

func TestHandleRejectsUnsupportedActionBeforeMutation(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := Handle(req, json.RawMessage(`{"fixture_action":"apply","fixture":{"version":1}}`))
	encoded, _ := json.Marshal(resp)
	if !strings.Contains(string(encoded), "invalid_param") || !strings.Contains(string(encoded), "validate") {
		t.Fatalf("response = %s, want actionable validate-only error", encoded)
	}
}

func TestHandleRejectsMalformedFixtureRedacted(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := Handle(req, json.RawMessage(`{"fixture_action":"validate","fixture":{"version":1,"unknown":"private-value"}}`))
	encoded, _ := json.Marshal(resp)
	if !strings.Contains(string(encoded), "invalid_param") {
		t.Fatalf("response = %s, want invalid_param", encoded)
	}
	if strings.Contains(string(encoded), "private-value") {
		t.Fatal("response leaked invalid fixture value")
	}
}
