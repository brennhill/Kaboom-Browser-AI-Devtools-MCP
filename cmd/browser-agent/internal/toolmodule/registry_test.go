// registry_test.go — Tests canonical tool-module registration and validation.

package toolmodule

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type stubModule struct {
	validateErr   error
	executeCalled bool
}

func (module *stubModule) Validate(json.RawMessage) error { return module.validateErr }
func (module *stubModule) Execute(request mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	module.executeCalled = true
	return mcp.Succeed(request, "stub module executed", nil)
}
func (*stubModule) Describe() Description { return Description{Name: "stub"} }
func (*stubModule) Examples() []json.RawMessage {
	return []json.RawMessage{json.RawMessage(`{"example":"value"}`)}
}

func TestRegistryDispatchesRegisteredModule(t *testing.T) {
	module := &stubModule{}
	registry := New()
	registry.Register("stub", module)
	request := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	response, handled := registry.Dispatch(request, "stub", json.RawMessage(`{"x":1}`))
	if !handled || module.executeCalled == false || response.Error != nil {
		t.Fatalf("dispatch = %#v, %t; executed=%t", response, handled, module.executeCalled)
	}
	if len(registry.Examples("stub")) != 1 {
		t.Fatal("registered examples were not retained")
	}
}

func TestRegistryValidationFailureDoesNotExecute(t *testing.T) {
	module := &stubModule{validateErr: errors.New("bad params")}
	registry := New()
	registry.Register("stub", module)
	response, handled := registry.Dispatch(mcp.JSONRPCRequest{ID: 1}, "stub", json.RawMessage(`{"x":1}`))
	if !handled || module.executeCalled || response.Result == nil {
		t.Fatalf("validation response = %#v, %t; executed=%t", response, handled, module.executeCalled)
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if !result.IsError || !strings.Contains(result.Content[0].Text, "invalid_param") {
		t.Fatalf("validation result = %#v", result)
	}
}

func TestMethodModuleRejectsNonObjectArguments(t *testing.T) {
	registry := New(Spec{Name: "observe", Handle: func(request mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
		return mcp.Succeed(request, "ok", nil)
	}})
	for _, arguments := range []json.RawMessage{json.RawMessage(`not-json`), json.RawMessage(`[]`)} {
		response, handled := registry.Dispatch(mcp.JSONRPCRequest{ID: 1}, "observe", arguments)
		if !handled || response.Result == nil {
			t.Fatalf("invalid arguments %s were not handled", arguments)
		}
	}
	if _, handled := registry.Dispatch(mcp.JSONRPCRequest{ID: 1}, "missing", nil); handled {
		t.Fatal("unknown module was handled")
	}
}
