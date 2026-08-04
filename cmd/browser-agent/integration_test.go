// Purpose: Integration tests for browser-agent end-to-end flows.
// Docs: docs/features/feature/mcp-persistent-server/index.md

//go:build integration

// integration_test.go — Server startup and MCP API completeness tests.
//
// ⚠️ RELEASE GATE 8: MCP COMMAND COMPLETENESS
//
// These tests enforce that:
// 1. Server boots in under 1 second (cold start performance)
// 2. ALL exposed MCP tools return valid responses (not stubs or "not implemented")
//
// If any MCP tool fails to return a valid response, it MUST be:
// - Removed from tools_schema.go (don't expose unimplemented commands)
// - Marked as TODO in the codebase
// - Tracked in docs/core/known-issues.md
//
// See: docs/core/release.md#gate-8-mcp-command-completeness-mandatory
//
// Run: go test ./cmd/browser-agent -run "TestIntegration" -v
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// ============================================
// Server Startup Tests
// ============================================

// TestIntegration_ServerStartupUnder1Second verifies the server boots in < 1 second.
// This is a RELEASE GATE requirement - cold start must be fast.
func TestIntegration_ServerStartupUnder1Second(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

	// Measure startup time
	startTime := time.Now()

	// Start server
	cmd := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Wait for server to be ready (health endpoint)
	if !bridgeRuntime().WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start within 5 seconds")
	}

	startupTime := time.Since(startTime)

	// RELEASE GATE: Server must start in < 1 second
	if startupTime > 1*time.Second {
		t.Errorf("RELEASE GATE VIOLATION: Server startup took %v (must be < 1 second)", startupTime)
	} else {
		t.Logf("✅ Server started in %v (target: < 1 second)", startupTime.Round(time.Millisecond))
	}

	// Verify health endpoint returns 200
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health endpoint returned %d, expected 200", resp.StatusCode)
	}
}

// ============================================
// MCP API Completeness Tests
// ============================================

// TestIntegration_AllMCPToolsReturnValidResponses verifies every MCP tool works.
// This is a RELEASE GATE requirement - no stub implementations allowed.
func TestIntegration_AllMCPToolsReturnValidResponses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

	// Start server
	cmd := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	if !bridgeRuntime().WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	client := &http.Client{Timeout: 10 * time.Second}

	// Test all MCP tools with their required parameters
	// Each tool must return a valid JSON-RPC response with "result" (not error)
	toolTests := []struct {
		name    string
		request string
	}{
		// Core MCP methods
		{"initialize", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"integration-test","version":"1.0"}}}`},
		{"tools/list", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`},
		{"resources/list", `{"jsonrpc":"2.0","id":3,"method":"resources/list","params":{}}`},
		{"prompts/list", `{"jsonrpc":"2.0","id":4,"method":"prompts/list","params":{}}`},
		{"ping", `{"jsonrpc":"2.0","id":5,"method":"ping","params":{}}`},

		// observe tool - all 26 modes
		{"observe errors", `{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`},
		{"observe logs", `{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"observe","arguments":{"what":"logs"}}}`},
		{"observe extension_logs", `{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"observe","arguments":{"what":"extension_logs"}}}`},
		{"observe network_waterfall", `{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"observe","arguments":{"what":"network_waterfall"}}}`},
		{"observe network_bodies", `{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"observe","arguments":{"what":"network_bodies"}}}`},
		{"observe websocket_events", `{"jsonrpc":"2.0","id":15,"method":"tools/call","params":{"name":"observe","arguments":{"what":"websocket_events"}}}`},
		{"observe websocket_status", `{"jsonrpc":"2.0","id":16,"method":"tools/call","params":{"name":"observe","arguments":{"what":"websocket_status"}}}`},
		{"observe actions", `{"jsonrpc":"2.0","id":17,"method":"tools/call","params":{"name":"observe","arguments":{"what":"actions"}}}`},
		{"observe vitals", `{"jsonrpc":"2.0","id":18,"method":"tools/call","params":{"name":"observe","arguments":{"what":"vitals"}}}`},
		{"observe page", `{"jsonrpc":"2.0","id":19,"method":"tools/call","params":{"name":"observe","arguments":{"what":"page"}}}`},
		{"observe tabs", `{"jsonrpc":"2.0","id":20,"method":"tools/call","params":{"name":"observe","arguments":{"what":"tabs"}}}`},
		{"observe pilot", `{"jsonrpc":"2.0","id":21,"method":"tools/call","params":{"name":"observe","arguments":{"what":"pilot"}}}`},
		{"observe timeline", `{"jsonrpc":"2.0","id":24,"method":"tools/call","params":{"name":"observe","arguments":{"what":"timeline"}}}`},
		{"observe command_result", `{"jsonrpc":"2.0","id":32,"method":"tools/call","params":{"name":"observe","arguments":{"what":"command_result"}}}`},

		// analyze tool - modes that were moved from observe
		{"analyze performance", `{"jsonrpc":"2.0","id":22,"method":"tools/call","params":{"name":"analyze","arguments":{"what":"performance"}}}`},
		{"analyze accessibility", `{"jsonrpc":"2.0","id":23,"method":"tools/call","params":{"name":"analyze","arguments":{"what":"accessibility"}}}`},
		{"analyze error_clusters", `{"jsonrpc":"2.0","id":27,"method":"tools/call","params":{"name":"analyze","arguments":{"what":"error_clusters"}}}`},
		{"analyze history", `{"jsonrpc":"2.0","id":28,"method":"tools/call","params":{"name":"analyze","arguments":{"what":"history"}}}`},
		{"analyze security_audit", `{"jsonrpc":"2.0","id":29,"method":"tools/call","params":{"name":"analyze","arguments":{"what":"security_audit"}}}`},
		{"analyze third_party_audit", `{"jsonrpc":"2.0","id":30,"method":"tools/call","params":{"name":"analyze","arguments":{"what":"third_party_audit"}}}`},
		{"observe pending_commands", `{"jsonrpc":"2.0","id":33,"method":"tools/call","params":{"name":"observe","arguments":{"what":"pending_commands"}}}`},
		{"observe failed_commands", `{"jsonrpc":"2.0","id":34,"method":"tools/call","params":{"name":"observe","arguments":{"what":"failed_commands"}}}`},

		// generate tool - all 7 formats
		{"generate reproduction", `{"jsonrpc":"2.0","id":40,"method":"tools/call","params":{"name":"generate","arguments":{"what":"reproduction"}}}`},
		{"generate test", `{"jsonrpc":"2.0","id":41,"method":"tools/call","params":{"name":"generate","arguments":{"what":"test"}}}`},
		{"generate pr_summary", `{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"generate","arguments":{"what":"pr_summary"}}}`},
		{"generate sarif", `{"jsonrpc":"2.0","id":43,"method":"tools/call","params":{"name":"generate","arguments":{"what":"sarif"}}}`},
		{"generate har", `{"jsonrpc":"2.0","id":44,"method":"tools/call","params":{"name":"generate","arguments":{"what":"har"}}}`},
		{"generate csp", `{"jsonrpc":"2.0","id":45,"method":"tools/call","params":{"name":"generate","arguments":{"what":"csp"}}}`},
		{"generate sri", `{"jsonrpc":"2.0","id":46,"method":"tools/call","params":{"name":"generate","arguments":{"what":"sri"}}}`},

		// configure tool - sample of 14 actions
		{"configure health", `{"jsonrpc":"2.0","id":50,"method":"tools/call","params":{"name":"configure","arguments":{"what":"health"}}}`},
		{"configure store list", `{"jsonrpc":"2.0","id":51,"method":"tools/call","params":{"name":"configure","arguments":{"what":"store","store_action":"list"}}}`},
		{"configure store stats", `{"jsonrpc":"2.0","id":52,"method":"tools/call","params":{"name":"configure","arguments":{"what":"store","store_action":"stats"}}}`},
		{"configure noise_rule list", `{"jsonrpc":"2.0","id":53,"method":"tools/call","params":{"name":"configure","arguments":{"what":"noise_rule","noise_action":"list"}}}`},
		{"analyze api_validation report", `{"jsonrpc":"2.0","id":55,"method":"tools/call","params":{"name":"analyze","arguments":{"what":"api_validation","operation":"report"}}}`},
		{"configure streaming status", `{"jsonrpc":"2.0","id":57,"method":"tools/call","params":{"name":"configure","arguments":{"what":"streaming","streaming_action":"status"}}}`},

		// interact tool - list_states action (doesn't require browser)
		{"interact list_states", `{"jsonrpc":"2.0","id":60,"method":"tools/call","params":{"name":"interact","arguments":{"what":"list_states"}}}`},
	}

	var failures []string

	for _, tc := range toolTests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Post(mcpURL, "application/json", strings.NewReader(tc.request))
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: HTTP error: %v", tc.name, err))
				t.Errorf("HTTP request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: Read error: %v", tc.name, err))
				t.Errorf("Failed to read response: %v", err)
				return
			}

			// Verify HTTP 200
			if resp.StatusCode != http.StatusOK {
				failures = append(failures, fmt.Sprintf("%s: HTTP %d", tc.name, resp.StatusCode))
				t.Errorf("Expected HTTP 200, got %d", resp.StatusCode)
				return
			}

			// Parse JSON-RPC response
			var jsonResp map[string]any
			if err := json.Unmarshal(body, &jsonResp); err != nil {
				failures = append(failures, fmt.Sprintf("%s: Invalid JSON", tc.name))
				t.Errorf("Invalid JSON response: %v", err)
				return
			}

			// Verify JSON-RPC 2.0
			if jsonResp["jsonrpc"] != "2.0" {
				failures = append(failures, fmt.Sprintf("%s: Missing jsonrpc:2.0", tc.name))
				t.Errorf("Response missing jsonrpc:2.0")
				return
			}

			// Verify has result (not just error)
			_, hasResult := jsonResp["result"]
			errObj, hasError := jsonResp["error"]

			if hasError && !hasResult {
				// Check if this is a "not implemented" error
				if errMap, ok := errObj.(map[string]any); ok {
					msg := fmt.Sprintf("%v", errMap["message"])
					if strings.Contains(strings.ToLower(msg), "not implemented") ||
						strings.Contains(strings.ToLower(msg), "unimplemented") ||
						strings.Contains(strings.ToLower(msg), "todo") {
						failures = append(failures, fmt.Sprintf("%s: NOT IMPLEMENTED - must remove from MCP schema", tc.name))
						t.Errorf("RELEASE GATE VIOLATION: %s returns 'not implemented' error - must be removed from tools_schema.go", tc.name)
						return
					}
				}
				// Other errors might be OK (e.g., "no data" is valid for empty buffers)
				t.Logf("⚠️ %s returned error (may be OK): %v", tc.name, errObj)
				return
			}

			if !hasResult {
				failures = append(failures, fmt.Sprintf("%s: Missing result", tc.name))
				t.Errorf("Response missing 'result' field")
				return
			}

			// Check for "not implemented" in result text (some tools return this as text)
			if result, ok := jsonResp["result"].(map[string]any); ok {
				if content, ok := result["content"].([]any); ok && len(content) > 0 {
					if textBlock, ok := content[0].(map[string]any); ok {
						if text, ok := textBlock["text"].(string); ok {
							lowerText := strings.ToLower(text)
							if strings.Contains(lowerText, "not implemented") ||
								strings.Contains(lowerText, "unimplemented") {
								failures = append(failures, fmt.Sprintf("%s: Returns 'not implemented' text", tc.name))
								t.Errorf("RELEASE GATE VIOLATION: %s returns 'not implemented' - must be removed from tools_schema.go", tc.name)
								return
							}
						}
					}
				}
			}

			t.Logf("✅ %s: Valid response", tc.name)
		})
	}

	// Summary
	if len(failures) > 0 {
		t.Logf("\n=== RELEASE GATE 8 FAILURES ===")
		for _, f := range failures {
			t.Logf("  ❌ %s", f)
		}
		t.Logf("\nAction Required:")
		t.Logf("  1. Remove unimplemented tools from cmd/browser-agent/tools_schema.go")
		t.Logf("  2. Add TODO comments in the code")
		t.Logf("  3. Track in docs/core/known-issues.md")
	}
}

// TestIntegration_ToolsListMatchesImplementation verifies tools/list only shows implemented tools.
func TestIntegration_ToolsListMatchesImplementation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

	// Start server
	cmd := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	if !bridgeRuntime().WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	client := &http.Client{Timeout: 5 * time.Second}

	// Get tools/list
	resp, err := client.Post(mcpURL, "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	if err != nil {
		t.Fatalf("tools/list request failed: %v", err)
	}
	defer resp.Body.Close()

	var toolsResp struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&toolsResp); err != nil {
		t.Fatalf("Failed to decode tools/list response: %v", err)
	}

	t.Logf("Server exposes %d tools:", len(toolsResp.Result.Tools))
	for _, tool := range toolsResp.Result.Tools {
		t.Logf("  - %s", tool.Name)
	}

	// Verify the 5 expected tools are present
	// Updated in Phase 0 to include new "analyze" tool for active analysis operations
	expectedTools := []string{"observe", "analyze", "generate", "configure", "interact"}
	for _, expected := range expectedTools {
		found := false
		for _, tool := range toolsResp.Result.Tools {
			if tool.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool '%s' not found in tools/list", expected)
		}
	}

	// Verify no extra unexpected tools
	if len(toolsResp.Result.Tools) != 5 {
		t.Errorf("Expected exactly 5 tools, got %d", len(toolsResp.Result.Tools))
	}
}

// Note: waitForServer, findFreePort, and buildTestBinary helpers are defined
// in connection_lifecycle_test.go and shared across test files in this package.
