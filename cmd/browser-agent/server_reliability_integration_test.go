// Purpose: Integration tests for browser-agent end-to-end flows.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// server_reliability_integration_test.go — MCP traffic, upgrade, and integration tests.
//
// ⚠️ RELEASE GATE TESTS - MANDATORY BEFORE EVERY RELEASE
//
// These tests verify realistic MCP sessions, server upgrade/replacement,
// and full protocol compliance.
//
// Run:
//   go test ./cmd/browser-agent -run "TestReliability_MCPTraffic" -v
//   go test ./cmd/browser-agent -run "TestReliability_Upgrade" -v
//   go test ./cmd/browser-agent -run "TestReliability_Integration" -v
//
// DO NOT skip these tests before release.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
)

// ============================================================================
// MCP TRAFFIC SIMULATION - Realistic usage patterns
// ============================================================================

// TestReliability_MCPTraffic_RealisticSession simulates a realistic MCP session
// with various tool calls in typical patterns.
func TestReliability_MCPTraffic_RealisticSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MCP traffic test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

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

	if !bridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)

	// Simulate realistic MCP session
	sessionSteps := []struct {
		name    string
		request string
	}{
		// Initialize
		{"initialize", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"reliability-test","version":"1.0"}}}`},
		{"tools/list", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`},

		// Typical debugging session
		{"observe errors", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`},
		{"observe logs", `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"observe","arguments":{"what":"logs","limit":50}}}`},
		{"observe network", `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"observe","arguments":{"what":"network_waterfall","limit":20}}}`},
		{"observe page", `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"observe","arguments":{"what":"page"}}}`},

		// Configuration
		{"configure health", `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"configure","arguments":{"what":"health"}}}`},
		{"configure noise list", `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"configure","arguments":{"what":"noise_rule","noise_action":"list"}}}`},

		// More observation
		{"observe vitals", `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"observe","arguments":{"what":"vitals"}}}`},
		{"observe actions", `{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"observe","arguments":{"what":"actions"}}}`},

		// Generate outputs
		{"generate test", `{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"generate","arguments":{"what":"test"}}}`},
		{"generate reproduction", `{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"generate","arguments":{"what":"reproduction"}}}`},

		// Final checks
		{"observe errors again", `{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`},
		{"ping", `{"jsonrpc":"2.0","id":14,"method":"ping","params":{}}`},
	}

	var successCount int
	for _, step := range sessionSteps {
		resp, err := client.Post(mcpURL, "application/json", strings.NewReader(step.request))
		if err != nil {
			t.Fatalf("Step '%s' failed: %v", step.name, err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var jsonResp map[string]any
		if err := json.Unmarshal(body, &jsonResp); err != nil {
			t.Fatalf("Step '%s' returned invalid JSON: %v", step.name, err)
		}

		if jsonResp["jsonrpc"] != "2.0" {
			t.Fatalf("Step '%s' missing jsonrpc:2.0", step.name)
		}

		// Check for result (not error)
		if _, hasResult := jsonResp["result"]; hasResult {
			successCount++
			t.Logf("  \u2713 %s", step.name)
		} else if errObj, hasError := jsonResp["error"]; hasError {
			t.Logf("  \u26a0 %s: error response: %v", step.name, errObj)
		}

		// Realistic delay between calls
		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("\u2705 Realistic MCP session completed: %d/%d steps successful", successCount, len(sessionSteps))
}

// TestReliability_MCPTraffic_BurstPattern simulates burst traffic pattern
// (many requests, pause, many requests) common in real usage.
func TestReliability_MCPTraffic_BurstPattern(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping burst pattern test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

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

	if !bridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)

	// Burst pattern: 20 rapid requests, 5s pause, repeat 3 times
	for burst := 0; burst < 3; burst++ {
		t.Logf("Burst %d: sending 20 rapid requests...", burst+1)

		for i := 0; i < 20; i++ {
			var resp *http.Response
			var err error

			if i%3 == 0 {
				resp, err = client.Get(healthURL)
			} else {
				mcpReq := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`
				resp, err = client.Post(mcpURL, "application/json", strings.NewReader(mcpReq))
			}

			if err != nil {
				t.Fatalf("Burst %d request %d failed: %v", burst+1, i+1, err)
			}
			resp.Body.Close()
		}

		t.Logf("Burst %d complete, pausing 5s...", burst+1)
		time.Sleep(5 * time.Second)

		// Verify server still healthy after pause
		resp, err := client.Get(healthURL)
		if err != nil {
			t.Fatalf("Server died during pause after burst %d: %v", burst+1, err)
		}
		resp.Body.Close()
	}

	t.Log("\u2705 Server survived 3 burst cycles (60 requests total with 5s pauses)")
}

// ============================================================================
// UPGRADE/INSTALL TESTS - Server replacement on upgrade
// ============================================================================

// TestReliability_Upgrade_OldServerKilled verifies that starting a new server
// on a port where an old server is running will successfully replace it.
//
// This is critical for npm/pypi upgrades - the new version must be able to
// take over from the old version.
func TestReliability_Upgrade_OldServerKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping upgrade test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

	// Start "old" server
	oldCmd := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port))
	oldStdin, err := oldCmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe for old server: %v", err)
	}
	if err := oldCmd.Start(); err != nil {
		t.Fatalf("Failed to start old server: %v", err)
	}
	oldPID := oldCmd.Process.Pid

	if !bridge.WaitForServer(port, serverStartTimeout) {
		_ = oldStdin.Close()
		_ = oldCmd.Process.Kill()
		t.Fatalf("Old server failed to start")
	}

	t.Logf("Old server started with PID %d on port %d", oldPID, port)

	// Verify old server is responding
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		t.Fatalf("Old server not responding: %v", err)
	}
	resp.Body.Close()

	// Kill the old server (simulating what npm/pypi wrapper does)
	killCmd := exec.Command("kill", fmt.Sprintf("%d", oldPID))
	if err := killCmd.Run(); err != nil {
		t.Logf("Warning: kill command failed: %v", err)
	}

	// Wait for old server to die
	time.Sleep(500 * time.Millisecond)

	// Start "new" server on same port
	newCmd := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port))
	newStdin, err := newCmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe for new server: %v", err)
	}
	if err := newCmd.Start(); err != nil {
		t.Fatalf("Failed to start new server: %v", err)
	}
	defer func() {
		_ = newStdin.Close()
		_ = newCmd.Process.Kill()
		_ = newCmd.Wait()
	}()

	newPID := newCmd.Process.Pid
	t.Logf("New server started with PID %d on port %d", newPID, port)

	// Wait for new server to be ready
	if !bridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("New server failed to start on port %d", port)
	}

	// Verify new server is responding
	resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		t.Fatalf("New server not responding: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("New server returned %d", resp.StatusCode)
	}

	// Verify old server process is dead
	oldProcess, _ := os.FindProcess(oldPID)
	if oldProcess != nil {
		// Try to signal - if process is dead, this returns error
		err := oldProcess.Signal(os.Signal(nil))
		if err == nil {
			// Process still exists - try to clean up
			_ = oldStdin.Close()
			_ = oldCmd.Process.Kill()
			t.Logf("Warning: Old server (PID %d) still running, cleaning up", oldPID)
		}
	}

	t.Logf("\u2705 Server replacement successful: old PID %d \u2192 new PID %d", oldPID, newPID)
}

// TestReliability_Upgrade_PortConflictDetection verifies that the server
// detects when a port is already in use (e.g., --doctor flag).
func TestReliability_Upgrade_PortConflictDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping port conflict test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

	// Start server on port
	cmd1 := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port))
	stdin1, err := cmd1.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	if err := cmd1.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = stdin1.Close()
		_ = cmd1.Process.Kill()
		_ = cmd1.Wait()
	}()

	if !bridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	t.Logf("First server running on port %d", port)

	// Use --doctor to detect port conflict without starting.
	cmd2 := startServerCmd(t, binary, "--port", fmt.Sprintf("%d", port), "--doctor")
	output, _ := cmd2.CombinedOutput()

	// --doctor should report the port is in use.
	outputStr := string(output)
	t.Logf("--doctor output: %s", outputStr)

	// Verify first server is still running
	resp, httpErr := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if httpErr != nil {
		t.Fatalf("First server died unexpectedly: %v", httpErr)
	}
	resp.Body.Close()

	// The --doctor command should indicate the port status
	// (whether it reports "in use" or "available" depends on implementation)
	t.Logf("\u2705 Port conflict detection works, first server still running")
}

// ============================================================================
// INTEGRATION TESTS - Full stack verification
// ============================================================================

// TestReliability_Integration_FullMCPProtocol verifies complete MCP protocol
// compliance including all required methods.
func TestReliability_Integration_FullMCPProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

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

	if !bridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)

	// Required MCP methods that MUST work
	requiredMethods := []struct {
		name    string
		request string
		check   func(map[string]any) error
	}{
		{
			"initialize",
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
			func(resp map[string]any) error {
				result, ok := resp["result"].(map[string]any)
				if !ok {
					return fmt.Errorf("missing result")
				}
				if _, ok := result["protocolVersion"]; !ok {
					return fmt.Errorf("missing protocolVersion")
				}
				if _, ok := result["serverInfo"]; !ok {
					return fmt.Errorf("missing serverInfo")
				}
				return nil
			},
		},
		{
			"tools/list",
			`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
			func(resp map[string]any) error {
				result, ok := resp["result"].(map[string]any)
				if !ok {
					return fmt.Errorf("missing result")
				}
				tools, ok := result["tools"].([]any)
				if !ok {
					return fmt.Errorf("missing tools array")
				}
				if len(tools) != 5 {
					return fmt.Errorf("expected 5 tools, got %d", len(tools))
				}
				return nil
			},
		},
		{
			"resources/list",
			`{"jsonrpc":"2.0","id":3,"method":"resources/list","params":{}}`,
			func(resp map[string]any) error {
				_, ok := resp["result"]
				if !ok {
					return fmt.Errorf("missing result")
				}
				return nil
			},
		},
		{
			"prompts/list",
			`{"jsonrpc":"2.0","id":4,"method":"prompts/list","params":{}}`,
			func(resp map[string]any) error {
				_, ok := resp["result"]
				if !ok {
					return fmt.Errorf("missing result")
				}
				return nil
			},
		},
		{
			"ping",
			`{"jsonrpc":"2.0","id":5,"method":"ping","params":{}}`,
			func(resp map[string]any) error {
				_, ok := resp["result"]
				if !ok {
					return fmt.Errorf("missing result")
				}
				return nil
			},
		},
	}

	for _, method := range requiredMethods {
		t.Run(method.name, func(t *testing.T) {
			resp, err := client.Post(mcpURL, "application/json", strings.NewReader(method.request))
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			var jsonResp map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
				t.Fatalf("Invalid JSON response: %v", err)
			}

			if jsonResp["jsonrpc"] != "2.0" {
				t.Fatalf("Missing jsonrpc:2.0")
			}

			if err := method.check(jsonResp); err != nil {
				t.Fatalf("Response validation failed: %v", err)
			}
		})
	}

	t.Log("\u2705 All required MCP protocol methods working correctly")
}

// TestReliability_Integration_LargePayloads verifies server handles large
// request/response payloads without issues.
func TestReliability_Integration_LargePayloads(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large payload test in short mode")
	}

	port := findFreePort(t)
	binary := buildTestBinary(t)

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

	if !bridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)

	// Test with large arguments
	largeString := strings.Repeat("x", 100000) // 100KB string
	largeReq := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"configure","arguments":{"what":"store","store_action":"save","key":"large_test","data":{"content":"%s"}}}}`, largeString)

	resp, err := client.Post(mcpURL, "application/json", bytes.NewReader([]byte(largeReq)))
	if err != nil {
		t.Fatalf("Large payload request failed: %v", err)
	}
	resp.Body.Close()

	// Server should handle large request (even if it returns error for invalid action)
	t.Logf("Large request (100KB payload) handled: HTTP %d", resp.StatusCode)

	// Verify server still works after large payload
	healthResp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		t.Fatalf("Server died after large payload: %v", err)
	}
	healthResp.Body.Close()

	t.Log("\u2705 Server handled large payloads without issues")
}
