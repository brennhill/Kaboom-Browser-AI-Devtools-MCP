// responses_test.go — Tests canonical stateless MCP protocol responses.

package mcpprotocol

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestInitializeMatchesGoldenContract(t *testing.T) {
	t.Parallel()
	request := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion, ID: 1,
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}`),
	}
	response := Initialize(request, "VERSION")
	actual, err := json.MarshalIndent(json.RawMessage(response.Result), "", "  ")
	if err != nil {
		t.Fatalf("marshal initialize result: %v", err)
	}
	actual = append(actual, '\n')
	path := "../../testdata/mcp-initialize.golden.json"
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, actual, 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
		return
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("initialize golden mismatch; run UPDATE_GOLDEN=1 go test ./cmd/browser-agent/internal/mcpprotocol -run TestInitializeMatchesGoldenContract")
	}
}

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
	for _, required := range []string{"USE KABOOM", "DO NOT use Chrome DevTools", "DO NOT use Playwright", "user explicitly requests", "capability gap"} {
		if !strings.Contains(result.Instructions, required) {
			t.Fatalf("instructions missing %q: %q", required, result.Instructions)
		}
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
