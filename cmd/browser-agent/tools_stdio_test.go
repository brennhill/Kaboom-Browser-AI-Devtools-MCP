// Purpose: Tests for stdio transport tool call handling.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_stdio_test.go — Unit tests verifying tool handlers produce no stdio pollution
//
// ⚠️ CRITICAL INVARIANT TEST - MCP TOOL STDIO SILENCE
//
// Tool handlers MUST NOT write to stdout. All output to stdout corrupts
// the MCP JSON-RPC protocol. Only the transport layer (bridge.go, main.go)
// should write JSON-RPC responses to stdout.
//
// These tests verify that calling any tool handler produces ZERO output
// to stdout during execution. The JSON-RPC response is returned as a
// value, not written to stdout.
//
// See: .claude/refs/mcp-stdio-invariant.md
//
// DO NOT:
// - Remove or weaken these tests
// - Add fmt.Print* calls to tool handlers
// - Add log.Print* calls that write to stdout
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// captureStdout runs fn while capturing stdout output.
// Returns any output written to stdout during execution.
// Thread-safe: uses a mutex to prevent concurrent stdout captures.
var stdoutCaptureMu sync.Mutex

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	stdoutCaptureMu.Lock()
	defer stdoutCaptureMu.Unlock()

	// Save original stdout
	oldStdout := os.Stdout

	// Create pipe
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	// Redirect stdout to pipe
	os.Stdout = w

	// Run the function
	fn()

	// Close writer and restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if err != nil {
		t.Fatalf("Failed to read captured stdout: %v", err)
	}
	r.Close()

	return buf.String()
}

// createTestToolHandler creates a minimal ToolHandler for testing
func createTestToolHandler(t *testing.T) *ToolHandler {
	t.Helper()

	// Create server with temp log file
	server, err := NewServer(t.TempDir()+"/test-kaboom-stdio.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	cap := capture.NewCapture()
	cap.Extension().SetPilotEnabled(false) // explicit default for legacy pilot-disabled expectations

	// Create handler using proper constructor
	mcpHandler := NewToolHandler(server, cap)

	// Extract the ToolHandler from MCPHandler.toolHandler
	toolHandler := mcpHandler.toolHandler.(*ToolHandler)

	return toolHandler
}

// TestToolHandler_Observe_NoStdout verifies observe tool produces no stdout output
func TestToolHandler_Observe_NoStdout(t *testing.T) {
	if testing.Short() {
		t.Skip("skips 5s+ waterfall timeout in short mode")
	}

	handler := createTestToolHandler(t)

	testCases := []struct {
		name string
		what string
	}{
		{"errors", "errors"},
		{"logs", "logs"},
		{"network", "network"},
		{"page", "page"},
		{"network_bodies", "network_bodies"},
		{"network_waterfall", "network_waterfall"},
		{"websocket_events", "websocket_events"},
		{"enhanced_actions", "enhanced_actions"},
		{"performance", "performance"},
		{"vitals", "vitals"},
		{"extension_logs", "extension_logs"},
		{"timeline", "timeline"},
		{"security_audit", "security_audit"},
		{"accessibility", "accessibility"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := json.RawMessage(`{"what":"` + tc.what + `"}`)
			req := mcp.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
			}

			output := captureStdout(t, func() {
				_ = handler.observeDispatcher.Handle(req, args)
			})

			if output != "" {
				t.Errorf("INVARIANT VIOLATION: observe(what=%q) wrote to stdout: %q", tc.what, output)
				t.Error("Tool handlers MUST NOT write to stdout - only the transport layer should")
			}
		})
	}
}

// TestToolHandler_Configure_NoStdout verifies configure tool produces no stdout output
func TestToolHandler_Configure_NoStdout(t *testing.T) {
	handler := createTestToolHandler(t)

	testCases := []struct {
		name   string
		action string
		args   string
	}{
		{"health", "health", `{"what":"health"}`},
		{"clear", "clear", `{"what":"clear"}`},
		{"status", "status", `{"what":"status"}`},
		{"store_invalid", "store", `{"what":"store","key":"test","data":"val"}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := json.RawMessage(tc.args)
			req := mcp.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
			}

			output := captureStdout(t, func() {
				_ = handler.toolConfigure(req, args)
			})

			if output != "" {
				t.Errorf("INVARIANT VIOLATION: configure(action=%q) wrote to stdout: %q", tc.action, output)
				t.Error("Tool handlers MUST NOT write to stdout - only the transport layer should")
			}
		})
	}
}

// TestToolHandler_Generate_NoStdout verifies generate tool produces no stdout output
func TestToolHandler_Generate_NoStdout(t *testing.T) {
	handler := createTestToolHandler(t)

	testCases := []struct {
		name   string
		format string
		args   string
	}{
		{"test", "test", `{"what":"test","test_name":"example"}`},
		{"reproduction", "reproduction", `{"what":"reproduction"}`},
		{"har", "har", `{"what":"har"}`},
		{"sarif", "sarif", `{"what":"sarif"}`},
		{"pr_summary", "pr_summary", `{"what":"pr_summary"}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := json.RawMessage(tc.args)
			req := mcp.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
			}

			output := captureStdout(t, func() {
				_ = handler.generateDispatcher.Handle(req, args)
			})

			if output != "" {
				t.Errorf("INVARIANT VIOLATION: generate(format=%q) wrote to stdout: %q", tc.format, output)
				t.Error("Tool handlers MUST NOT write to stdout - only the transport layer should")
			}
		})
	}
}

// TestToolHandler_Interact_NoStdout verifies interact tool produces no stdout output
func TestToolHandler_Interact_NoStdout(t *testing.T) {
	handler := createTestToolHandler(t)

	testCases := []struct {
		name   string
		action string
		args   string
	}{
		{"highlight", "highlight", `{"what":"highlight","selector":".test"}`},
		{"save_state", "save_state", `{"what":"save_state","name":"test"}`},
		{"load_state", "load_state", `{"what":"load_state","name":"test"}`},
		{"list_states", "list_states", `{"what":"list_states"}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := json.RawMessage(tc.args)
			req := mcp.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
			}

			output := captureStdout(t, func() {
				_ = handler.toolInteract(req, args)
			})

			if output != "" {
				t.Errorf("INVARIANT VIOLATION: interact(action=%q) wrote to stdout: %q", tc.action, output)
				t.Error("Tool handlers MUST NOT write to stdout - only the transport layer should")
			}
		})
	}
}

// TestToolHandler_HandleToolCall_NoStdout verifies the main dispatch produces no stdout
func TestToolHandler_HandleToolCall_NoStdout(t *testing.T) {
	handler := createTestToolHandler(t)

	testCases := []struct {
		name string
		tool string
		args string
	}{
		{"observe_errors", "observe", `{"what":"errors"}`},
		{"configure_health", "configure", `{"what":"health"}`},
		{"generate_test", "generate", `{"what":"test","test_name":"x"}`},
		{"interact_highlight", "interact", `{"what":"highlight","selector":"div"}`},
		{"unknown_tool", "nonexistent", `{}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := json.RawMessage(tc.args)
			req := mcp.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
			}

			output := captureStdout(t, func() {
				_, _ = handler.HandleToolCall(req, tc.tool, args)
			})

			if output != "" {
				t.Errorf("INVARIANT VIOLATION: HandleToolCall(%q) wrote to stdout: %q", tc.tool, output)
				t.Error("Tool handlers MUST NOT write to stdout - only the transport layer should")
			}
		})
	}
}

// TestToolHandler_ToolsList_NoStdout verifies tools/list produces no stdout
func TestToolHandler_ToolsList_NoStdout(t *testing.T) {
	handler := createTestToolHandler(t)

	output := captureStdout(t, func() {
		_ = handler.ToolsList()
	})

	if output != "" {
		t.Errorf("INVARIANT VIOLATION: ToolsList() wrote to stdout: %q", output)
		t.Error("Tool handlers MUST NOT write to stdout - only the transport layer should")
	}
}

// TestToolHandler_ResponseHelpers_NoStdout verifies MCP response helpers produce no stdout
func TestToolHandler_ResponseHelpers_NoStdout(t *testing.T) {
	t.Run("mcp.TextResponse", func(t *testing.T) {
		output := captureStdout(t, func() {
			_ = mcp.TextResponse("test message")
		})
		if output != "" {
			t.Errorf("INVARIANT VIOLATION: mcp.TextResponse wrote to stdout: %q", output)
		}
	})

	t.Run("mcp.JSONResponse", func(t *testing.T) {
		output := captureStdout(t, func() {
			_ = mcp.JSONResponse("summary", map[string]string{"key": "value"})
		})
		if output != "" {
			t.Errorf("INVARIANT VIOLATION: mcp.JSONResponse wrote to stdout: %q", output)
		}
	})

	t.Run("mcp.StructuredErrorResponse", func(t *testing.T) {
		output := captureStdout(t, func() {
			_ = mcp.StructuredErrorResponse(mcp.ErrInvalidParam, "test", "retry hint")
		})
		if output != "" {
			t.Errorf("INVARIANT VIOLATION: mcp.StructuredErrorResponse wrote to stdout: %q", output)
		}
	})
}

// TestToolHandler_MarshalFailure_NoStdout verifies marshal failures don't write to stdout
// (they should only write to stderr which is safe)
func TestToolHandler_MarshalFailure_NoStdout(t *testing.T) {
	// Create a value that will fail to marshal (channel)
	unmarshalable := make(chan int)

	output := captureStdout(t, func() {
		_ = mcp.SafeMarshal(unmarshalable, `{"fallback":true}`)
	})

	if output != "" {
		t.Errorf("INVARIANT VIOLATION: mcp.SafeMarshal failure wrote to stdout: %q", output)
		t.Error("Marshal errors should only go to stderr, not stdout")
	}
}

// TestToolHandler_StderrAllowed verifies stderr output is allowed (doesn't break MCP)
func TestToolHandler_StderrAllowed(t *testing.T) {
	// This is a documentation test - stderr is intentionally allowed
	// because MCP clients only parse stdout for JSON-RPC messages.
	//
	// mcp.SafeMarshal writes errors to stderr:
	//   fmt.Fprintf(os.Stderr, "[Kaboom] JSON marshal error: %v\n", err)
	//
	// This is CORRECT behavior - stderr doesn't pollute MCP protocol.
	t.Log("stderr output is intentionally allowed - it doesn't affect MCP JSON-RPC")
}
