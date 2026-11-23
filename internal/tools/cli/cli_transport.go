// cli_transport.go — Implements CLI-side JSON-RPC transport to the local MCP endpoint.
// Why: Isolates request/response wire handling from CLI orchestration and daemon startup concerns.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/mcp"
)

// CallTool builds a JSON-RPC tools/call request, POSTs to /mcp, and parses the response.
func CallTool(baseURL, toolName string, mcpArgs map[string]any, timeoutMs int, maxBodySize int64) (*mcp.MCPToolResult, error) {
	body, err := BuildToolCallBody(toolName, mcpArgs)
	if err != nil {
		return nil, err
	}

	respBody, err := PostToolCall(baseURL+"/mcp", body, timeoutMs, maxBodySize)
	if err != nil {
		return nil, err
	}

	return ParseToolCallResponse(respBody)
}

// BuildToolCallBody creates the JSON-RPC request body for a tools/call.
func BuildToolCallBody(toolName string, mcpArgs map[string]any) ([]byte, error) {
	params := map[string]any{"name": toolName, "arguments": mcpArgs}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w. Check argument types", err)
	}

	rpcReq := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      "cli-1",
		Method:  "tools/call",
		Params:  paramsJSON,
	}
	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w. Check argument types", err)
	}
	return body, nil
}

// PostToolCall sends a JSON-RPC request to the MCP endpoint and returns the raw response body.
func PostToolCall(endpoint string, body []byte, timeoutMs int, maxBodySize int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create HTTP request: %w. Verify endpoint URL", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq) // #nosec G704 -- endpoint comes from EnsureDaemon() and is localhost-only
	if err != nil {
		return nil, fmt.Errorf("connect to server: %w. Verify daemon is running on the target port", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("read response: %w. Server may have disconnected", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// ParseToolCallResponse parses a JSON-RPC response into an MCPToolResult.
func ParseToolCallResponse(respBody []byte) (*mcp.MCPToolResult, error) {
	var rpcResp mcp.JSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("server error (%d): %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	var toolResult mcp.MCPToolResult
	if err := json.Unmarshal(rpcResp.Result, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}
	return &toolResult, nil
}
