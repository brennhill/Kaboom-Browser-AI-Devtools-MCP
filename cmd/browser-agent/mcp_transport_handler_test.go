// mcp_transport_handler_test.go — Tests MCP HTTP handlers and bridge forwarding.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
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

// TestMCPProtocol_HandlerUnit tests the handler directly (faster unit test).
func TestMCPProtocol_HandlerUnit(t *testing.T) {
	// Create handler with minimal dependencies
	handler := NewMCPHandler(nil, "test-version")

	testCases := []struct {
		name           string
		request        mcp.JSONRPCRequest
		expectResponse bool
		expectError    bool
	}{
		{
			name:           "notification returns nil",
			request:        mcp.JSONRPCRequest{JSONRPC: "2.0", Method: "notifications/initialized"},
			expectResponse: false,
		},
		{
			name:           "notification with nil ID returns nil",
			request:        mcp.JSONRPCRequest{JSONRPC: "2.0", ID: nil, Method: "some/method"},
			expectResponse: false,
		},
		{
			name:           "request with ID returns response",
			request:        mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "ping"},
			expectResponse: true,
			expectError:    false,
		},
		{
			name:           "unknown method returns error",
			request:        mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "unknown/method"},
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
	testServer := httptest.NewServer(newMCPHTTPHandler(handler))
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
	// Matched case-insensitively on the call rather than on a bare `cmbridge.WriteMCPPayload(`:
	// the bridge extraction moved this behind a Deps struct, so the call site is now
	// `deps.WriteMCPPayload(body, framing)`. The invariant is unchanged — the body
	// must reach the framing-aware serialized writer — and this still fails if the
	// body is written any other way.
	if !strings.Contains(source, "WriteMCPPayload(body, framing)") &&
		!strings.Contains(source, "cmbridge.WriteMCPPayload(body, framing)") {
		t.Error("CRITICAL: bridge.go must forward HTTP bodies via WriteMCPPayload(body, framing)")
	} else {
		t.Log("bridge.go forwards HTTP bodies via WriteMCPPayload")
	}

	// Verify no fmt.Println(string(body)) pattern
	if strings.Contains(source, "fmt.Println(string(body))") {
		t.Error("CRITICAL: Found fmt.Println(string(body)) in bridge.go - this causes double newlines!")
	}
}
