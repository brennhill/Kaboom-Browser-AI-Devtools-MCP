// mcp_initialize_test.go — Tests MCP initialization and tool discovery.
// Docs: docs/features/feature/mcp-persistent-server/index.md

//go:build integration

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// ⚠️ CRITICAL MCP PROTOCOL COMPLIANCE TESTS - DO NOT MODIFY WITHOUT PRINCIPAL REVIEW
//
// These tests verify MCP specification compliance. They MUST NEVER FAIL.
// The MCP spec defines exact response format requirements that clients depend on.
//
// Reference: https://spec.modelcontextprotocol.io/specification/
//
// Key invariants tested:
// 1. Exactly ONE trailing newline per message (not zero, not two)
// 2. Notifications NEVER receive responses
// 3. JSON-RPC 2.0 structure is always correct
// 4. Error codes match JSON-RPC 2.0 spec
// 5. ID is NEVER null in responses (Cursor rejects it)
// 6. Result and error are mutually exclusive
//
// DO NOT:
// - Remove or skip any test cases
// - Weaken assertions or add exceptions
// - Change without approval from principal engineer

// TestMCPProtocol_InitializeResponse verifies initialize response structure.
func TestMCPProtocol_InitializeResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

	// Start server
	serverCmd := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port))
	serverStdin, _ := serverCmd.StdinPipe()
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = serverStdin.Close()
		_ = serverCmd.Process.Kill()
		_ = serverCmd.Wait()
	}()

	if !bridgeRuntime().WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	client := &http.Client{Timeout: 5 * time.Second}

	request := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`

	resp, err := client.Post(mcpURL, "application/json", strings.NewReader(request))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	var response struct {
		JSONRPC string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Result  struct {
			ProtocolVersion string `json:"protocolVersion"`
			ServerInfo      struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
			Capabilities struct{} `json:"capabilities"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check required fields
	if response.Result.ProtocolVersion == "" {
		t.Error("Missing protocolVersion in initialize response")
	} else {
		t.Logf("✅ protocolVersion: %s", response.Result.ProtocolVersion)
	}

	if response.Result.ServerInfo.Name == "" {
		t.Error("Missing serverInfo.name in initialize response")
	} else {
		t.Logf("✅ serverInfo.name: %s", response.Result.ServerInfo.Name)
	}

	// Version can be empty in test builds, just log it
	t.Logf("✅ serverInfo.version: %s (may be empty in test builds)", response.Result.ServerInfo.Version)
}

// TestMCPProtocol_ToolsListStructure verifies tools/list response structure.
func TestMCPProtocol_ToolsListStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

	// Start server
	serverCmd := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port))
	serverStdin, _ := serverCmd.StdinPipe()
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = serverStdin.Close()
		_ = serverCmd.Process.Kill()
		_ = serverCmd.Wait()
	}()

	if !bridgeRuntime().WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	client := &http.Client{Timeout: 5 * time.Second}

	request := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`

	resp, err := client.Post(mcpURL, "application/json", strings.NewReader(request))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	var response struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				InputSchema any    `json:"inputSchema"`
				Meta        any    `json:"_meta"` // Should NOT exist
			} `json:"tools"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Result.Tools) == 0 {
		t.Fatal("tools/list should return at least one tool")
	}

	t.Logf("Found %d tools", len(response.Result.Tools))

	// Check first tool has required fields
	tool := response.Result.Tools[0]

	if tool.Name == "" {
		t.Error("Tool missing 'name' field")
	} else {
		t.Logf("✅ Tool has name: %s", tool.Name)
	}

	if tool.InputSchema == nil {
		t.Error("Tool missing 'inputSchema' field")
	} else {
		t.Logf("✅ Tool has inputSchema")
	}

	// Check NO _meta field (not in MCP spec)
	if tool.Meta != nil {
		t.Errorf("Tool has '_meta' field (not in MCP spec)")
	} else {
		t.Logf("✅ Tool has no '_meta' field")
	}
}
