// Purpose: Tests for MCP JSON-RPC protocol compliance.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	cmbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
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

// TestMCPProtocol_ResponseNewlines verifies exactly one trailing newline per response.
// Double newlines cause "Unexpected end of JSON input" errors in IDEs.
func TestMCPProtocol_ResponseNewlines(t *testing.T) {
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

	if !cmbridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	client := &http.Client{Timeout: 5 * time.Second}

	testCases := []struct {
		name    string
		request string
	}{
		{"initialize", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`},
		{"tools/list", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`},
		{"resources/list", `{"jsonrpc":"2.0","id":3,"method":"resources/list","params":{}}`},
		{"prompts/list", `{"jsonrpc":"2.0","id":4,"method":"prompts/list","params":{}}`},
		{"ping", `{"jsonrpc":"2.0","id":5,"method":"ping","params":{}}`},
		{"unknown method", `{"jsonrpc":"2.0","id":6,"method":"unknown/method","params":{}}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Post(mcpURL, "application/json", strings.NewReader(tc.request))
			if err != nil {
				t.Fatalf("HTTP request failed: %v", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response: %v", err)
			}

			// CRITICAL: Response must end with exactly ONE newline
			if len(body) == 0 {
				t.Fatalf("Empty response body")
			}

			// Check last character is newline
			if body[len(body)-1] != '\n' {
				t.Errorf("Response does not end with newline: %q", body[len(body)-min(20, len(body)):])
			}

			// Check second-to-last character is NOT newline (no double newlines)
			if len(body) > 1 && body[len(body)-2] == '\n' {
				t.Errorf("Response has DOUBLE NEWLINES (causes 'Unexpected end of JSON input'): ...%q", body[len(body)-min(30, len(body)):])
			}

			// Verify it's valid JSON (without the trailing newline)
			jsonBody := bytes.TrimSuffix(body, []byte("\n"))
			if !json.Valid(jsonBody) {
				t.Errorf("Response is not valid JSON: %s", string(body[:min(200, len(body))]))
			}

			t.Logf("✅ %s: Single trailing newline verified", tc.name)
		})
	}
}

// TestMCPProtocol_NotificationNoResponse verifies notifications don't get responses.
// Responding to notifications causes Zod validation errors in clients.
func TestMCPProtocol_NotificationNoResponse(t *testing.T) {
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

	if !cmbridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	client := &http.Client{Timeout: 5 * time.Second}

	// Notifications have NO id field (or method starts with "notifications/")
	notifications := []struct {
		name    string
		request string
	}{
		{"notifications/initialized", `{"jsonrpc":"2.0","method":"notifications/initialized"}`},
		{"notifications/cancelled", `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"1","reason":"test"}}`},
		{"notification without id", `{"jsonrpc":"2.0","method":"some/notification"}`},
	}

	for _, tc := range notifications {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Post(mcpURL, "application/json", strings.NewReader(tc.request))
			if err != nil {
				t.Fatalf("HTTP request failed: %v", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)

			// Notifications should get 204 No Content
			if resp.StatusCode != http.StatusNoContent {
				t.Errorf("Expected 204 No Content for notification, got %d with body: %s", resp.StatusCode, string(body))
			}

			// Body should be empty
			if len(body) > 0 {
				t.Errorf("Expected empty body for notification, got: %s", string(body))
			}

			t.Logf("✅ %s: No response (204 No Content)", tc.name)
		})
	}
}

// TestMCPProtocol_JSONRPCStructure verifies JSON-RPC 2.0 response structure.
func TestMCPProtocol_JSONRPCStructure(t *testing.T) {
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

	if !cmbridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("response has jsonrpc 2.0", func(t *testing.T) {
		resp, err := client.Post(mcpURL, "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{}}`))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		var response map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Must have jsonrpc: "2.0"
		if response["jsonrpc"] != "2.0" {
			t.Errorf("Expected jsonrpc: '2.0', got: %v", response["jsonrpc"])
		}

		t.Logf("✅ jsonrpc: '2.0' present")
	})

	t.Run("result and error are mutually exclusive", func(t *testing.T) {
		// Send valid request - should have result, not error
		resp, err := client.Post(mcpURL, "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{}}`))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		var response map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&response)

		_, hasResult := response["result"]
		_, hasError := response["error"]

		if hasResult && hasError {
			t.Errorf("Response has BOTH result and error (invalid JSON-RPC 2.0)")
		}

		if !hasResult && !hasError {
			t.Errorf("Response has NEITHER result nor error (invalid JSON-RPC 2.0)")
		}

		if hasResult {
			t.Logf("✅ Success response has 'result' only")
		}

		// Send invalid request - should have error, not result
		resp2, err := client.Post(mcpURL, "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"nonexistent","params":{}}`))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp2.Body.Close()

		var response2 map[string]any
		_ = json.NewDecoder(resp2.Body).Decode(&response2)

		_, hasResult2 := response2["result"]
		_, hasError2 := response2["error"]

		if hasResult2 && hasError2 {
			t.Errorf("Error response has BOTH result and error (invalid JSON-RPC 2.0)")
		}

		if hasError2 {
			t.Logf("✅ Error response has 'error' only")
		}
	})
}

// TestMCPProtocol_IDNeverNull verifies ID is never null (except parse errors per JSON-RPC).
func TestMCPProtocol_IDNeverNull(t *testing.T) {
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

	if !cmbridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	client := &http.Client{Timeout: 5 * time.Second}

	testCases := []struct {
		name       string
		request    string
		expectedID any
		expectNull bool
	}{
		{"string id", `{"jsonrpc":"2.0","id":"test-string-id","method":"ping","params":{}}`, "test-string-id", false},
		{"number id", `{"jsonrpc":"2.0","id":12345,"method":"ping","params":{}}`, float64(12345), false},
		{"parse error (malformed JSON)", `not valid json at all`, nil, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Post(mcpURL, "application/json", strings.NewReader(tc.request))
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			var response map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			id := response["id"]

			// Parse errors must return null id per JSON-RPC.
			if tc.expectNull {
				if id != nil {
					t.Errorf("parse error id = %v, want null", id)
				}
				return
			}

			// ID must NEVER be null for non-parse errors.
			if id == nil {
				t.Errorf("CRITICAL: ID is null for non-parse error")
			}

			// Verify ID matches expected
			if id != tc.expectedID {
				t.Logf("ID mismatch: expected %v (%T), got %v (%T)", tc.expectedID, tc.expectedID, id, id)
				// Only fail if it's null, otherwise just log
				if id == nil {
					t.Fail()
				}
			}

			t.Logf("✅ ID is %v (not null)", id)
		})
	}
}

// TestMCPProtocol_ErrorCodes verifies JSON-RPC 2.0 error codes.
func TestMCPProtocol_ErrorCodes(t *testing.T) {
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

	if !cmbridge.WaitForServer(port, serverStartTimeout) {
		t.Fatalf("Server failed to start")
	}

	mcpURL := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	client := &http.Client{Timeout: 5 * time.Second}

	testCases := []struct {
		name         string
		request      string
		expectedCode float64
		description  string
	}{
		{
			name:         "method not found",
			request:      `{"jsonrpc":"2.0","id":1,"method":"nonexistent/method","params":{}}`,
			expectedCode: -32601,
			description:  "Method not found",
		},
		{
			name:         "parse error",
			request:      `{invalid json}`,
			expectedCode: -32700,
			description:  "Parse error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Post(mcpURL, "application/json", strings.NewReader(tc.request))
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			var response map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			errorObj, ok := response["error"].(map[string]any)
			if !ok {
				t.Fatalf("Response should have error object")
			}

			code, ok := errorObj["code"].(float64)
			if !ok {
				t.Fatalf("Error should have numeric code")
			}

			if code != tc.expectedCode {
				t.Errorf("Expected error code %v (%s), got %v", tc.expectedCode, tc.description, code)
			}

			message, ok := errorObj["message"].(string)
			if !ok || message == "" {
				t.Errorf("Error must have non-empty message")
			}

			t.Logf("✅ %s: error code %v with message: %s", tc.name, code, message)
		})
	}
}

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

	if !cmbridge.WaitForServer(port, serverStartTimeout) {
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

	if !cmbridge.WaitForServer(port, serverStartTimeout) {
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

// TestMCPProtocol_HandlerUnit tests the handler directly (faster unit test).
func TestMCPProtocol_HandlerUnit(t *testing.T) {
	// Create handler with minimal dependencies
	handler := NewMCPHandler(nil, "test-version")

	testCases := []struct {
		name           string
		request        JSONRPCRequest
		expectResponse bool
		expectError    bool
	}{
		{
			name:           "notification returns nil",
			request:        JSONRPCRequest{JSONRPC: "2.0", Method: "notifications/initialized"},
			expectResponse: false,
		},
		{
			name:           "notification with nil ID returns nil",
			request:        JSONRPCRequest{JSONRPC: "2.0", ID: nil, Method: "some/method"},
			expectResponse: false,
		},
		{
			name:           "request with ID returns response",
			request:        JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "ping"},
			expectResponse: true,
			expectError:    false,
		},
		{
			name:           "unknown method returns error",
			request:        JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "unknown/method"},
			expectResponse: true,
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := handler.HandleRequest(tc.request)

			if tc.expectResponse {
				if resp == nil {
					t.Error("Expected response, got nil")
					return
				}

				if tc.expectError && resp.Error == nil {
					t.Error("Expected error response")
				}

				if !tc.expectError && resp.Error != nil {
					t.Errorf("Unexpected error: %s", resp.Error.Message)
				}

				// ID should never be nil in response
				if resp.ID == nil {
					t.Error("Response ID is nil")
				}
			} else {
				if resp != nil {
					t.Errorf("Expected nil response for notification, got: %+v", resp)
				}
			}
		})
	}
}

// TestMCPProtocol_HTTPHandler tests HTTP handler notification handling.
func TestMCPProtocol_HTTPHandler(t *testing.T) {
	handler := NewMCPHandler(nil, "test-version")

	// Create test server
	testServer := httptest.NewServer(http.HandlerFunc(handler.HandleHTTP))
	defer testServer.Close()

	t.Run("notification returns 204", func(t *testing.T) {
		resp, err := http.Post(
			testServer.URL,
			"application/json",
			strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`),
		)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected 204 No Content for notification, got %d: %s", resp.StatusCode, string(body))
		}
	})

	t.Run("request returns 200 with JSON", func(t *testing.T) {
		resp, err := http.Post(
			testServer.URL,
			"application/json",
			strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{}}`),
		)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)

		// Verify single trailing newline
		if len(body) > 0 && body[len(body)-1] != '\n' {
			t.Error("Response should end with newline")
		}

		if len(body) > 1 && body[len(body)-2] == '\n' {
			t.Error("Response has double newlines")
		}
	})
}

// TestMCPProtocol_BridgeCodeVerification verifies bridge forwarding routes response
// bodies through writeMCPPayload (single writer, framing-aware) and never uses
// fmt.Println for raw response body forwarding.
func TestMCPProtocol_BridgeCodeVerification(t *testing.T) {
	// Read the bridge source that performs HTTP body forwarding.
	//
	// This previously read "bridge_forward.go" relative to this package and
	// t.Skipf'd on failure. The file moved to internal/bridge/ in March, so the
	// read has failed ever since and the test has been silently skipping — it
	// reported as coverage while asserting nothing. Fatal, not Skip: if the
	// source this test inspects cannot be found, the test has not passed.
	bridgeSource, err := os.ReadFile(filepath.Join("internal", "bridge", "bridge.go"))
	if err != nil {
		t.Fatalf("could not read internal/bridge/bridge.go (did it move?): %v", err)
	}

	source := string(bridgeSource)

	// CRITICAL: forwarding must go through writeMCPPayload so stdout framing stays
	// consistent (line-delimited vs Content-Length) and writes remain serialized.
	//
	// Matched case-insensitively on the call rather than on a bare `writeMCPPayload(`:
	// the bridge extraction moved this behind a Deps struct, so the call site is now
	// `deps.WriteMCPPayload(body, framing)`. The invariant is unchanged — the body
	// must reach the framing-aware serialized writer — and this still fails if the
	// body is written any other way.
	if !strings.Contains(source, "WriteMCPPayload(body, framing)") &&
		!strings.Contains(source, "writeMCPPayload(body, framing)") {
		t.Error("CRITICAL: bridge.go must forward HTTP bodies via WriteMCPPayload(body, framing)")
	} else {
		t.Log("bridge.go forwards HTTP bodies via WriteMCPPayload")
	}

	// Verify no fmt.Println(string(body)) pattern
	if strings.Contains(source, "fmt.Println(string(body))") {
		t.Error("CRITICAL: Found fmt.Println(string(body)) in bridge.go - this causes double newlines!")
	}
}

func TestMCPProtocol_WriteMCPPayload_LineFramingNormalizesTrailingNewline(t *testing.T) {
	rawPayload := []byte(" \n\t" + `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}` + "\n\n ")
	output := captureStdout(t, func() {
		writeMCPPayload(rawPayload, bridge.StdioFramingLine)
	})

	if !strings.HasSuffix(output, "\n") {
		t.Fatalf("output must end with newline: %q", output)
	}
	if strings.HasSuffix(output, "\n\n") {
		t.Fatalf("output has double trailing newline: %q", output)
	}

	trimmed := strings.TrimSuffix(output, "\n")
	if !json.Valid([]byte(trimmed)) {
		t.Fatalf("output is not valid JSON after trimming newline: %q", output)
	}
	if strings.HasPrefix(trimmed, " ") || strings.HasSuffix(trimmed, " ") {
		t.Fatalf("output should not keep outer whitespace: %q", output)
	}
}

func TestMCPProtocol_WriteMCPPayload_ContentLengthUsesTrimmedPayload(t *testing.T) {
	rawPayload := []byte(" \n\t" + `{"jsonrpc":"2.0","id":9,"result":{"ok":true}}` + "\n\n ")
	output := captureStdout(t, func() {
		writeMCPPayload(rawPayload, bridge.StdioFramingContentLength)
	})

	parts := strings.SplitN(output, "\r\n\r\n", 2)
	if len(parts) != 2 {
		t.Fatalf("expected content-length framed output, got: %q", output)
	}
	header := parts[0]
	body := parts[1]

	if !strings.HasPrefix(header, "Content-Length: ") {
		t.Fatalf("missing Content-Length header: %q", header)
	}
	lengthPart := strings.TrimPrefix(strings.SplitN(header, "\r\n", 2)[0], "Content-Length: ")
	reportedLen, err := strconv.Atoi(strings.TrimSpace(lengthPart))
	if err != nil {
		t.Fatalf("invalid Content-Length header %q: %v", header, err)
	}
	if reportedLen != len(body) {
		t.Fatalf("Content-Length mismatch: header=%d body=%d", reportedLen, len(body))
	}
	if strings.HasPrefix(body, " ") || strings.HasSuffix(body, " ") {
		t.Fatalf("body should not keep outer whitespace: %q", body)
	}
	if !json.Valid([]byte(body)) {
		t.Fatalf("body is not valid JSON: %q", body)
	}
}

func TestMCPProtocol_WriteMCPPayload_InvalidJSONFallsBackToJSONRPCError(t *testing.T) {
	output := captureStdout(t, func() {
		writeMCPPayload([]byte("not-json"), bridge.StdioFramingLine)
	})

	trimmed := strings.TrimSuffix(output, "\n")
	if !json.Valid([]byte(trimmed)) {
		t.Fatalf("fallback output is not valid JSON: %q", output)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		t.Fatalf("unmarshal fallback output: %v", err)
	}
	if decoded["jsonrpc"] != "2.0" {
		t.Fatalf("expected jsonrpc=2.0, got: %#v", decoded["jsonrpc"])
	}
	errObj, ok := decoded["error"].(map[string]any)
	if !ok || errObj["message"] == nil {
		t.Fatalf("expected JSON-RPC error object, got: %#v", decoded["error"])
	}
}
