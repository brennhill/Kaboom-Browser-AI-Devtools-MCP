// Purpose: Tests for tool handler registry and lookup.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolmodule"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type stubToolModule struct {
	validateErr    error
	validateCalled bool
	executeCalled  bool
}

func (m *stubToolModule) Validate(args json.RawMessage) error {
	m.validateCalled = true
	return m.validateErr
}

func (m *stubToolModule) Execute(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	m.executeCalled = true
	return mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcp.JSONResponse("stub module executed", map[string]any{"status": "ok"}),
	}
}

func (m *stubToolModule) Describe() toolmodule.Description {
	return toolmodule.Description{Name: "stub_tool", Summary: "test-only stub"}
}

func (m *stubToolModule) Examples() []json.RawMessage {
	return []json.RawMessage{json.RawMessage(`{"example":"value"}`)}
}

func TestNewToolHandler_WiresCoreToolModules(t *testing.T) {
	env := newToolTestEnv(t)

	if env.handler.toolModules == nil {
		t.Fatal("toolModules should be initialized")
	}
	for _, name := range []string{"observe", "analyze", "generate", "configure", "interact"} {
		module, ok := env.handler.toolModules.Get(name)
		if !ok || module == nil {
			t.Fatalf("%s module should be registered", name)
		}
		desc := module.Describe()
		if desc.Name != name {
			t.Fatalf("%s module Describe().Name = %q, want %q", name, desc.Name, name)
		}
		if len(module.Examples()) == 0 {
			t.Fatalf("%s module should expose at least one example", name)
		}
	}
}

func TestHandleToolCall_DispatchesRegisteredModule(t *testing.T) {
	env := newToolTestEnv(t)
	stub := &stubToolModule{}
	env.handler.toolModules.Register("stub_tool", stub)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`"test-id"`), Method: "tools/call"}
	resp, handled := env.handler.HandleToolCall(req, "stub_tool", json.RawMessage(`{"x":1}`))

	if !handled {
		t.Fatal("expected registered module tool to be handled")
	}
	if !stub.validateCalled {
		t.Fatal("expected module Validate to be called")
	}
	if !stub.executeCalled {
		t.Fatal("expected module Execute to be called")
	}

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected non-error response, got: %s", firstText(result))
	}
	if !strings.Contains(firstText(result), "stub module executed") {
		t.Fatalf("unexpected response text: %s", firstText(result))
	}
}

func TestHandleToolCall_ModuleValidationError(t *testing.T) {
	env := newToolTestEnv(t)
	stub := &stubToolModule{validateErr: errors.New("bad params")}
	env.handler.toolModules.Register("stub_tool", stub)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`"test-id"`), Method: "tools/call"}
	resp, handled := env.handler.HandleToolCall(req, "stub_tool", json.RawMessage(`{"x":1}`))

	if !handled {
		t.Fatal("expected module validation failure to return a handled response")
	}
	if !stub.validateCalled {
		t.Fatal("expected module Validate to be called")
	}
	if stub.executeCalled {
		t.Fatal("module Execute should not be called when Validate fails")
	}

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatalf("expected error response, got: %s", firstText(result))
	}
	if !strings.Contains(firstText(result), "invalid_param") {
		t.Fatalf("expected invalid_param in error response, got: %s", firstText(result))
	}
}
