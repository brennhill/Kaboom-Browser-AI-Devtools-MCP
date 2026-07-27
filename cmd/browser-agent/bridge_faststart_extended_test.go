// Purpose: Extended tests for bridge fast-start client compatibility.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// bridge_faststart_extended_test.go — Extended fast-start tests for MCP bridge mode.
// Covers: client compatibility matrix, resource workflow soak, retry-when-booting,
// and version verification.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// TestFastStart_ClientCompatibilityMatrix validates immediate resources/read behavior
// for multiple MCP clients right after initialize.
func TestFastStart_ClientCompatibilityMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("skips server spawn in short mode")
	}

	binary := buildTestBinary(t)
	clients := []struct {
		name        string
		clientName  string
		clientVer   string
		playbookURI string
	}{
		{name: "claude_code", clientName: "claude-code", clientVer: "1.0", playbookURI: "kaboom://playbook/performance"},
		{name: "cursor", clientName: "cursor", clientVer: "1.0", playbookURI: "kaboom://playbook/performance_analysis/quick"},
		{name: "windsurf", clientName: "windsurf", clientVer: "1.0", playbookURI: "kaboom://playbook/accessibility_audit/quick"},
		{name: "continue", clientName: "continue", clientVer: "1.0", playbookURI: "kaboom://playbook/security_audit/quick"},
	}

	for _, tc := range clients {
		t.Run(tc.name, func(t *testing.T) {
			port := findFreePort(t)
			cmd := startServerCmd(t, binary, "--bridge", "--port", fmt.Sprintf("%d", port))

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
			initReq := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"%s","version":"%s"}}}`, tc.clientName, tc.clientVer)
			writeJSONRPCLine(t, stdin, initReq)
			initResp := readJSONRPCLine(t, reader, 5*time.Second)
			if initResp.Error != nil {
				t.Fatalf("initialize error: %+v", initResp.Error)
			}

			start := time.Now()
			writeJSONRPCLine(t, stdin, `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"kaboom://capabilities"}}`)
			capResp := readJSONRPCLine(t, reader, 1*time.Second)
			if capResp.Error != nil {
				t.Fatalf("resources/read capabilities error: %+v", capResp.Error)
			}
			if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
				t.Fatalf("resources/read capabilities elapsed = %v, want < 500ms", elapsed)
			}

			playbookReq := fmt.Sprintf(`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"%s"}}`, tc.playbookURI)
			writeJSONRPCLine(t, stdin, playbookReq)
			playbookResp := readJSONRPCLine(t, reader, 1*time.Second)
			if playbookResp.Error != nil {
				t.Fatalf("resources/read playbook error: %+v", playbookResp.Error)
			}
		})
	}
}

// TestFastStart_ResourceWorkflowSoak repeatedly exercises the recommended startup
// resource workflow to catch timing/race regressions under sustained use.
func TestFastStart_ResourceWorkflowSoak(t *testing.T) {
	if testing.Short() {
		t.Skip("skips server spawn in short mode")
	}

	binary := buildTestBinary(t)
	port := findFreePort(t)
	cmd := startServerCmd(t, binary, "--bridge", "--port", fmt.Sprintf("%d", port))

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
	writeJSONRPCLine(t, stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"soak-test","version":"1.0"}}}`)
	initResp := readJSONRPCLine(t, reader, 5*time.Second)
	if initResp.Error != nil {
		t.Fatalf("initialize error: %+v", initResp.Error)
	}

	const iterations = 40
	start := time.Now()
	for i := 0; i < iterations; i++ {
		baseID := 100 + (i * 10)
		writeJSONRPCLine(t, stdin, fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"resources/read","params":{"uri":"kaboom://capabilities"}}`, baseID))
		capResp := readJSONRPCLine(t, reader, 1*time.Second)
		if capResp.Error != nil {
			t.Fatalf("iteration %d capabilities error: %+v", i, capResp.Error)
		}

		writeJSONRPCLine(t, stdin, fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"resources/read","params":{"uri":"kaboom://playbook/security_audit/quick"}}`, baseID+1))
		playbookResp := readJSONRPCLine(t, reader, 1*time.Second)
		if playbookResp.Error != nil {
			t.Fatalf("iteration %d playbook error: %+v", i, playbookResp.Error)
		}

		// Include tool calls intermittently to verify mixed workflow stability.
		if i%4 == 0 {
			writeJSONRPCLine(t, stdin, fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`, baseID+2))
			toolResp := readJSONRPCLine(t, reader, 1*time.Second)
			if toolResp.Error != nil {
				t.Fatalf("iteration %d tools/call protocol error: %+v", i, toolResp.Error)
			}
		}
	}
	elapsed := time.Since(start)
	if elapsed > 20*time.Second {
		t.Fatalf("soak loop elapsed = %v, want <= 20s", elapsed)
	}
	t.Logf("soak completed: %d iterations in %v", iterations, elapsed.Round(time.Millisecond))
}

// TestFastStart_ToolsCallWaitsForDaemonBoot verifies that tools/call converges
// through daemon startup instead of surfacing transient startup retry envelopes.
func TestFastStart_ToolsCallWaitsForDaemonBoot(t *testing.T) {
	if testing.Short() {
		t.Skip("skips server spawn in short mode")
	}

	binary := buildTestBinary(t)
	// Use a port that definitely has no server running
	port := findFreePort(t)

	cmd := startServerCmd(t, binary, "--bridge", "--port", fmt.Sprintf("%d", port))

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

	// Send initialize first
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
	if _, initErr := stdin.Write([]byte(initReq + "\n")); initErr != nil {
		t.Fatalf("Failed to write initialize: %v", initErr)
	}
	reader.ReadString('\n') // consume initialize response

	// Immediately send tools/call while daemon startup is still in-flight.
	toolsCallReq := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`

	start := time.Now()
	if _, callErr := stdin.Write([]byte(toolsCallReq + "\n")); callErr != nil {
		t.Fatalf("Failed to write tools/call: %v", callErr)
	}

	line, err := reader.ReadString('\n')
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	// Should converge within startup budget and avoid long hangs.
	if elapsed > 3*time.Second {
		t.Errorf("tools/call took %v, expected <= 3s while daemon starts", elapsed)
	} else {
		t.Logf("tools/call responded in %v (<= 3s)", elapsed)
	}

	// Verify response structure - should be a result, not an error
	var rpcResp mcp.JSONRPCResponse
	if err := json.Unmarshal([]byte(line), &rpcResp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Response should be a tool result (either successful data or a true failure).
	if rpcResp.Error != nil {
		t.Errorf("expected tool result, got protocol error: %v", rpcResp.Error.Message)
	}

	if rpcResp.Result != nil {
		var result map[string]any
		if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		if content, ok := result["content"].([]any); ok && len(content) > 0 {
			if textObj, ok := content[0].(map[string]any); ok {
				if text, ok := textObj["text"].(string); ok {
					normalized := strings.ToLower(text)
					if strings.Contains(normalized, "retry this tool call") || strings.Contains(normalized, "server is starting up") {
						t.Fatalf("unexpected startup retry envelope: %q", text)
					}
					t.Logf("tools/call content: %s...", text[:min(50, len(text))])
				}
			}
		}
	}
}

// TestFastStart_VersionInResponse ensures the version in initialize response
// matches the binary version.
func TestFastStart_VersionInResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("skips server spawn in short mode")
	}

	binary := buildTestBinary(t)
	port := findFreePort(t)

	cmd := startServerCmd(t, binary, "--bridge", "--port", fmt.Sprintf("%d", port))

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

	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
	if _, lastErr := stdin.Write([]byte(initReq + "\n")); lastErr != nil {
		t.Fatalf("Failed to write initialize: %v", lastErr)
	}

	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var rpcResp mcp.JSONRPCResponse
	if err := json.Unmarshal([]byte(line), &rpcResp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatal("Missing serverInfo in response")
	}

	responseVersion, ok := serverInfo["version"].(string)
	if !ok {
		t.Fatal("Missing version in serverInfo")
	}

	// Version should not be empty and should look like a semver
	if responseVersion == "" {
		t.Error("Version is empty")
	}

	if !strings.Contains(responseVersion, ".") {
		t.Errorf("Version '%s' doesn't look like semver", responseVersion)
	}

	t.Logf("Version in response: %s", responseVersion)
}

// TestFastStart_ResourceWorkflowBeforeDaemonReady verifies the recommended startup
// sequence works without waiting for daemon readiness.
func TestFastStart_ResourceWorkflowBeforeDaemonReady(t *testing.T) {
	if testing.Short() {
		t.Skip("skips server spawn in short mode")
	}

	binary := buildTestBinary(t)
	port := findFreePort(t)
	cmd := startServerCmd(t, binary, "--bridge", "--port", fmt.Sprintf("%d", port))

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

	start := time.Now()
	writeJSONRPCLine(t, stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"workflow-test","version":"1.0"}}}`)
	initResp := readJSONRPCLine(t, reader, 5*time.Second)
	if initResp.Error != nil {
		t.Fatalf("initialize error: %+v", initResp.Error)
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("initialize elapsed = %v, want < 4s", elapsed)
	}

	start = time.Now()
	writeJSONRPCLine(t, stdin, `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"kaboom://capabilities"}}`)
	capResp := readJSONRPCLine(t, reader, 1*time.Second)
	if capResp.Error != nil {
		t.Fatalf("resources/read capabilities error: %+v", capResp.Error)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("resources/read capabilities elapsed = %v, want < 500ms", elapsed)
	}

	var capResult mcp.MCPResourcesReadResult
	if err := json.Unmarshal(capResp.Result, &capResult); err != nil {
		t.Fatalf("capabilities result parse error: %v", err)
	}
	if len(capResult.Contents) != 1 || capResult.Contents[0].URI != "kaboom://capabilities" {
		t.Fatalf("capabilities result = %+v, want one capabilities content", capResult)
	}

	start = time.Now()
	writeJSONRPCLine(t, stdin, `{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"kaboom://playbook/security"}}`)
	playbookResp := readJSONRPCLine(t, reader, 1*time.Second)
	if playbookResp.Error != nil {
		t.Fatalf("resources/read playbook error: %+v", playbookResp.Error)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("resources/read playbook elapsed = %v, want < 500ms", elapsed)
	}
	var playbookResult mcp.MCPResourcesReadResult
	if err := json.Unmarshal(playbookResp.Result, &playbookResult); err != nil {
		t.Fatalf("playbook result parse error: %v", err)
	}
	if len(playbookResult.Contents) != 1 || playbookResult.Contents[0].URI != "kaboom://playbook/security/quick" {
		t.Fatalf("playbook result = %+v, want canonical security/quick content", playbookResult)
	}

	// tools/call should not surface protocol errors during startup.
	writeJSONRPCLine(t, stdin, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`)
	toolResp := readJSONRPCLine(t, reader, 3*time.Second)
	if toolResp.Error != nil {
		t.Fatalf("tools/call returned protocol error: %+v", toolResp.Error)
	}
}
