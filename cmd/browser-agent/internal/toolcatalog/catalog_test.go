// catalog_test.go — Verifies unified tool module and schema ownership.

package toolcatalog

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolmodule"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestCatalogOwnsDispatchSchemaAndExamples(t *testing.T) {
	schema := map[string]any{"type": "object"}
	catalog := New(
		[]toolmodule.Spec{{
			Name: "observe", Examples: []json.RawMessage{json.RawMessage(`{"what":"logs"}`)},
			Handle: func(req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
				return mcp.Succeed(req, "observed", nil)
			},
		}},
		[]mcp.MCPTool{{Name: "observe", InputSchema: schema}},
	)

	response, handled := catalog.Dispatch(mcp.JSONRPCRequest{ID: 1}, "observe", json.RawMessage(`{}`))
	if !handled || response.Error != nil {
		t.Fatalf("dispatch = (%#v, %v), want handled success", response, handled)
	}
	if catalog.Schema("observe")["type"] != "object" {
		t.Fatalf("schema = %#v", catalog.Schema("observe"))
	}
	if examples := catalog.Examples("observe"); len(examples) != 1 {
		t.Fatalf("examples = %d, want 1", len(examples))
	}
}

func TestCatalogDoesNotHandleUnknownTool(t *testing.T) {
	catalog := New(nil, nil)
	if _, handled := catalog.Dispatch(mcp.JSONRPCRequest{ID: 1}, "missing", nil); handled {
		t.Fatal("unknown tool should not be handled")
	}
	if catalog.Schema("missing") != nil {
		t.Fatal("unknown tool should not have a schema")
	}
}
