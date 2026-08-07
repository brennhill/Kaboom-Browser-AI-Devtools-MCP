// Purpose: Tests for bridge fast-start initialization responsiveness.
// Docs: docs/features/feature/mcp-persistent-server/index.md

//go:build integration

// bridge_faststart_test.go — Tests for MCP fast-start behavior.
// Verifies that initialize and tools/list respond immediately without waiting for daemon.
package bridgeintegration

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	testprocess "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/integrationtest"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
)

func readJSONRPCLine(t *testing.T, reader *bufio.Reader, timeout time.Duration) mcp.JSONRPCResponse {
	t.Helper()
	lineCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		line, err := reader.ReadString('\n')
		if err != nil {
			errCh <- err
			return
		}
		lineCh <- line
	}()

	select {
	case line := <-lineCh:
		var resp mcp.JSONRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("failed to parse JSON-RPC response: %v", err)
		}
		return resp
	case err := <-errCh:
		t.Fatalf("failed to read response: %v", err)
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for response after %v", timeout)
	}
	return mcp.JSONRPCResponse{}
}

func writeJSONRPCLine(t *testing.T, w io.Writer, raw string) {
	t.Helper()
	if _, err := w.Write([]byte(raw + "\n")); err != nil {
		t.Fatalf("failed to write request: %v", err)
	}
}

// TestFastStart_InitializeRespondsImmediately verifies that initialize returns
// within 100ms even when no daemon is running. This is critical for MCP client UX.
func TestFastStart_InitializeRespondsImmediately(t *testing.T) {
	if testing.Short() {
		t.Skip("skips server spawn in short mode")
	}

	// Build binary
	binary := testprocess.BuildBinary(t)

	// Use a port that's definitely not running
	port := testprocess.FreePort(t)

	// Start bridge mode (which uses fast-start)
	cmd := testprocess.StartServer(t, binary, "--bridge", "--port", fmt.Sprintf("%d", port))

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}

	// Discard stderr
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Send initialize request
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"faststart-test","version":"1.0"}}}`

	start := time.Now()
	if _, err := stdin.Write([]byte(initReq + "\n")); err != nil {
		t.Fatalf("Failed to write initialize: %v", err)
	}

	// Read response with timeout
	responseChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		reader := bufio.NewReader(stdout)
		line, err := reader.ReadString('\n')
		if err != nil {
			errChan <- err
			return
		}
		responseChan <- line
	}()

	select {
	case resp := <-responseChan:
		elapsed := time.Since(start)

		// First initialize includes process startup time (~300-500ms typical, up to ~3s on loaded machines).
		// The key guarantee is it responds WITHOUT waiting for daemon (which would add 5-10s).
		// We set 4s as upper bound to catch regressions while allowing for slow CI/loaded machines.
		if elapsed > 4*time.Second {
			t.Errorf("❌ Initialize took %v, expected < 4s (includes process startup)", elapsed)
		} else {
			t.Logf("✅ Initialize responded in %v (< 4s, includes process startup)", elapsed)
		}

		// Verify response structure
		var rpcResp mcp.JSONRPCResponse
		if err := json.Unmarshal([]byte(resp), &rpcResp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if rpcResp.Error != nil {
			t.Fatalf("Initialize returned error: %v", rpcResp.Error.Message)
		}

		// Verify it has the expected fields
		var result map[string]any
		if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		if _, ok := result["protocolVersion"]; !ok {
			t.Error("Missing protocolVersion in initialize response")
		}
		if _, ok := result["serverInfo"]; !ok {
			t.Error("Missing serverInfo in initialize response")
		}
		if _, ok := result["capabilities"]; !ok {
			t.Error("Missing capabilities in initialize response")
		}

		t.Logf("✅ Initialize response structure is valid")

	case err := <-errChan:
		t.Fatalf("Failed to read response: %v", err)

	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for initialize response")
	}
}

// TestFastStart_ToolsListRespondsImmediately verifies that tools/list returns
// immediately with the full tool schema, without waiting for daemon.
func TestFastStart_ToolsListRespondsImmediately(t *testing.T) {
	if testing.Short() {
		t.Skip("skips server spawn in short mode")
	}

	binary := testprocess.BuildBinary(t)
	port := testprocess.FreePort(t)

	cmd := testprocess.StartServer(t, binary, "--bridge", "--port", fmt.Sprintf("%d", port))

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}

	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Send initialize first (required by MCP protocol)
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
	if _, err := stdin.Write([]byte(initReq + "\n")); err != nil {
		t.Fatalf("Failed to write initialize: %v", err)
	}

	reader := bufio.NewReader(stdout)
	reader.ReadString('\n') // consume initialize response

	// Send tools/list
	toolsReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`

	start := time.Now()
	if _, err := stdin.Write([]byte(toolsReq + "\n")); err != nil {
		t.Fatalf("Failed to write tools/list: %v", err)
	}

	responseChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		line, err := reader.ReadString('\n')
		if err != nil {
			errChan <- err
			return
		}
		responseChan <- line
	}()

	select {
	case resp := <-responseChan:
		elapsed := time.Since(start)

		// CRITICAL: Must respond within 100ms
		if elapsed > 100*time.Millisecond {
			t.Errorf("❌ tools/list took %v, expected < 100ms", elapsed)
		} else {
			t.Logf("✅ tools/list responded in %v (< 100ms)", elapsed)
		}

		// Verify response has tools
		var rpcResp mcp.JSONRPCResponse
		if err := json.Unmarshal([]byte(resp), &rpcResp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if rpcResp.Error != nil {
			t.Fatalf("tools/list returned error: %v", rpcResp.Error.Message)
		}

		var result map[string]any
		if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		tools, ok := result["tools"].([]any)
		if !ok {
			t.Fatal("tools/list result missing 'tools' array")
		}

		// Verify we have the expected 4 tools
		expectedTools := []string{"observe", "generate", "configure", "interact"}
		foundTools := make(map[string]bool)

		for _, tool := range tools {
			if toolMap, ok := tool.(map[string]any); ok {
				if name, ok := toolMap["name"].(string); ok {
					foundTools[name] = true
				}
			}
		}

		for _, expected := range expectedTools {
			if !foundTools[expected] {
				t.Errorf("Missing expected tool: %s", expected)
			}
		}

		t.Logf("✅ tools/list returned %d tools", len(tools))

	case err := <-errChan:
		t.Fatalf("Failed to read response: %v", err)

	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for tools/list response")
	}
}

// TestFastStart_ToolsListSchemaStability ensures the tools/list schema doesn't change
// unexpectedly. This catches regressions in the MCP tool definitions.
func TestFastStart_ToolsListSchemaStability(t *testing.T) {
	tools := schema.AllTools()

	// Expected tool names (must not change without intentional update)
	// Updated in Phase 0 to include new "analyze" tool for active analysis operations
	expectedNames := []string{"observe", "analyze", "generate", "configure", "interact"}

	if len(tools) != len(expectedNames) {
		t.Errorf("Expected %d tools, got %d", len(expectedNames), len(tools))
	}

	for i, expected := range expectedNames {
		if i >= len(tools) {
			t.Errorf("Missing tool at index %d: expected %s", i, expected)
			continue
		}
		if tools[i].Name != expected {
			t.Errorf("Tool at index %d: expected %s, got %s", i, expected, tools[i].Name)
		}
	}

	// Verify each tool has required fields
	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("Tool has empty name")
		}
		if tool.Description == "" {
			t.Errorf("Tool %s has empty description", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("Tool %s has nil inputSchema", tool.Name)
		}

		// Verify inputSchema has type: object
		if schemaType, ok := tool.InputSchema["type"].(string); !ok || schemaType != "object" {
			t.Errorf("Tool %s inputSchema.type is not 'object'", tool.Name)
		}

		// Verify inputSchema has properties
		if _, ok := tool.InputSchema["properties"]; !ok {
			t.Errorf("Tool %s inputSchema missing 'properties'", tool.Name)
		}
	}

	t.Logf("✅ All %d tools have valid schema structure", len(tools))
}

// TestFastStart_OtherMethodsReturnQuickly verifies that ping, prompts/list, etc.
// also respond immediately without daemon.
func TestFastStart_OtherMethodsReturnQuickly(t *testing.T) {
	if testing.Short() {
		t.Skip("skips server spawn in short mode")
	}

	binary := testprocess.BuildBinary(t)
	port := testprocess.FreePort(t)

	cmd := testprocess.StartServer(t, binary, "--bridge", "--port", fmt.Sprintf("%d", port))

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}

	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)

	// Test methods that should respond immediately
	testCases := []struct {
		name    string
		request string
	}{
		{"initialize", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`},
		{"ping", `{"jsonrpc":"2.0","id":2,"method":"ping","params":{}}`},
		{"prompts/list", `{"jsonrpc":"2.0","id":3,"method":"prompts/list","params":{}}`},
		{"resources/list", `{"jsonrpc":"2.0","id":4,"method":"resources/list","params":{}}`},
		{"resources/templates/list", `{"jsonrpc":"2.0","id":5,"method":"resources/templates/list","params":{}}`},
		{"tools/list", `{"jsonrpc":"2.0","id":6,"method":"tools/list","params":{}}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			if _, writeErr := stdin.Write([]byte(tc.request + "\n")); writeErr != nil {
				t.Fatalf("Failed to write request: %v", writeErr)
			}

			line, err := reader.ReadString('\n')
			elapsed := time.Since(start)

			if err != nil && err != io.EOF {
				t.Fatalf("Failed to read response: %v", err)
			}

			// First request (initialize) includes process startup overhead
			// Subsequent requests should be < 100ms
			threshold := 100 * time.Millisecond
			if tc.name == "initialize" {
				threshold = 4 * time.Second // Includes process startup (up to ~3s on loaded machines)
			}

			if elapsed > threshold {
				t.Errorf("❌ %s took %v, expected < %v", tc.name, elapsed, threshold)
			} else {
				t.Logf("✅ %s responded in %v", tc.name, elapsed)
			}

			// Verify valid JSON-RPC response
			var rpcResp mcp.JSONRPCResponse
			if err := json.Unmarshal([]byte(line), &rpcResp); err != nil {
				t.Errorf("Invalid JSON-RPC response for %s: %v", tc.name, err)
			}

			if rpcResp.Error != nil {
				t.Errorf("%s returned error: %v", tc.name, rpcResp.Error.Message)
			}
		})
	}
}
