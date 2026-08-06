// handler_test.go — Tests canonical MCP tool-call validation and execution policy.

package mcpcall

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcpresponse"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcptelemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type executorFunc func(mcp.JSONRPCRequest, string, json.RawMessage) (mcp.JSONRPCResponse, bool)

func (execute executorFunc) HandleToolCall(request mcp.JSONRPCRequest, name string, arguments json.RawMessage) (mcp.JSONRPCResponse, bool) {
	return execute(request, name, arguments)
}

type fixedLimiter bool

func (limiter fixedLimiter) Allow() bool { return bool(limiter) }

type replacementRedactor struct{}

func (replacementRedactor) RedactJSON(json.RawMessage) json.RawMessage {
	return mcp.TextResponse("redacted")
}

func TestHandleValidatesParametersAndUnknownTools(t *testing.T) {
	responseOwner := mcpresponse.New(mcpresponse.Config{})
	telemetryOwner := mcptelemetry.New(mcptelemetry.Config{})
	request := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Params: json.RawMessage(`{bad`)}
	if response := Handle(request, Backend{}, responseOwner, telemetryOwner); response.Error == nil || response.Error.Code != -32602 {
		t.Fatalf("invalid params response = %#v", response)
	}
	request.Params = json.RawMessage(`{"name":"missing","arguments":{}}`)
	if response := Handle(request, Backend{}, responseOwner, telemetryOwner); response.Error == nil || response.Error.Code != -32601 {
		t.Fatalf("missing executor response = %#v", response)
	}
}

func TestHandleEnforcesRateLimitAndPostProcessesSuccess(t *testing.T) {
	responseOwner := mcpresponse.New(mcpresponse.Config{})
	telemetryOwner := mcptelemetry.New(mcptelemetry.Config{})
	request := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2, Params: json.RawMessage(`{"name":"observe","arguments":{}}`)}
	executor := executorFunc(func(request mcp.JSONRPCRequest, _ string, _ json.RawMessage) (mcp.JSONRPCResponse, bool) {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: request.ID, Result: mcp.TextResponse("secret")}, true
	})
	blocked := Handle(request, Backend{Executor: executor, Limiter: fixedLimiter(false)}, responseOwner, telemetryOwner)
	if blocked.Error == nil || blocked.Error.Code != -32603 {
		t.Fatalf("rate-limited response = %#v", blocked)
	}

	allowed := Handle(request, Backend{Executor: executor, Limiter: fixedLimiter(true), Redactor: replacementRedactor{}}, responseOwner, telemetryOwner)
	if allowed.Error != nil || allowed.Result == nil {
		t.Fatalf("allowed response = %#v", allowed)
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(allowed.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Text != "redacted" {
		t.Fatalf("post-processed text = %q", result.Content[0].Text)
	}
}
