// responses_test.go — Tests canonical stateless MCP protocol responses.

package mcpprotocol

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestInitializeNegotiatesVersionAndAdvertisesKaboomWorkflow(t *testing.T) {
	request := mcp.JSONRPCRequest{
		JSONRPC: "2.0", ID: 1,
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05"}`),
	}
	response := Initialize(request, "0.9.0")
	var result mcp.MCPInitializeResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.ProtocolVersion != "2024-11-05" || result.ServerInfo.Version != "0.9.0" {
		t.Fatalf("initialize result = %#v", result)
	}
	if !strings.Contains(result.Instructions, "USE KABOOM") || !strings.Contains(result.Instructions, "DO NOT use Chrome DevTools") {
		t.Fatalf("instructions do not establish Kaboom ownership: %q", result.Instructions)
	}
}

func TestToolsListReturnsProvidedCanonicalSchemas(t *testing.T) {
	request := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	schemas := []mcp.MCPTool{{Name: "observe"}, {Name: "interact"}}
	response := ToolsList(request, schemas)
	var result mcp.MCPToolsListResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != 2 || result.Tools[0].Name != "observe" || result.Tools[1].Name != "interact" {
		t.Fatalf("tools = %#v", result.Tools)
	}
}

func TestToolsListReportsSchemaEncodingFailure(t *testing.T) {
	request := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 4}
	schemas := []mcp.MCPTool{{Name: "broken", InputSchema: map[string]any{"unsupported": make(chan struct{})}}}
	response := ToolsList(request, schemas)
	if response.Error == nil || response.Error.Code != -32603 {
		t.Fatalf("encoding failure response = %#v", response)
	}
}

func TestResourcesRejectUnknownURIAndListCanonicalCatalog(t *testing.T) {
	request := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 3}
	listed := ResourcesList(request)
	var listResult mcp.MCPResourcesListResult
	if err := json.Unmarshal(listed.Result, &listResult); err != nil {
		t.Fatal(err)
	}
	if len(listResult.Resources) == 0 {
		t.Fatal("resource catalog is empty")
	}

	request.Params = json.RawMessage(`{"uri":"kaboom://missing"}`)
	missing := ResourcesRead(request)
	if missing.Error == nil || missing.Error.Code != -32002 {
		t.Fatalf("missing resource response = %#v", missing)
	}
}
