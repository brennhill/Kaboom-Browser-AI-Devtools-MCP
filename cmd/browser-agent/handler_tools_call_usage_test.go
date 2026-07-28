// handler_tools_call_usage_test.go — Tests for usage counter wiring in tool call handler.

package main

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

func TestExtractWhatParam(t *testing.T) {
	tests := []struct {
		name string
		args json.RawMessage
		want string
	}{
		{
			name: "valid args with what=errors",
			args: json.RawMessage(`{"what":"errors"}`),
			want: "errors",
		},
		{
			name: "missing what key",
			args: json.RawMessage(`{"key":"value"}`),
			want: "",
		},
		{
			name: "empty args object",
			args: json.RawMessage(`{}`),
			want: "",
		},
		{
			name: "malformed JSON",
			args: json.RawMessage(`{not valid json`),
			want: "",
		},
		{
			name: "nil args",
			args: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractWhatParam(tt.args)
			if got != tt.want {
				t.Errorf("extractWhatParam(%s) = %q, want %q", string(tt.args), got, tt.want)
			}
		})
	}
}

func TestUsageKey_CommandResultMapsToOriginalCommand(t *testing.T) {
	tests := []struct {
		name string
		args json.RawMessage
		want string
	}{
		{
			name: "command_result with nav prefix",
			args: json.RawMessage(`{"what":"command_result","correlation_id":"nav_1708300000_123"}`),
			want: "command_result:nav",
		},
		{
			name: "command_result with click prefix",
			args: json.RawMessage(`{"what":"command_result","correlation_id":"click_1708300000_456"}`),
			want: "command_result:click",
		},
		{
			name: "command_result with draw prefix",
			args: json.RawMessage(`{"what":"command_result","correlation_id":"draw_1708300000_789"}`),
			want: "command_result:draw",
		},
		{
			name: "command_result with ann_ prefix",
			args: json.RawMessage(`{"what":"command_result","correlation_id":"ann_1708300000_123"}`),
			want: "command_result:ann",
		},
		{
			name: "command_result without correlation_id",
			args: json.RawMessage(`{"what":"command_result"}`),
			want: "command_result",
		},
		{
			name: "command_result with empty correlation_id",
			args: json.RawMessage(`{"what":"command_result","correlation_id":""}`),
			want: "command_result",
		},
		{
			name: "command_result with no-underscore correlation_id",
			args: json.RawMessage(`{"what":"command_result","correlation_id":"plainid"}`),
			want: "command_result:plainid",
		},
		{
			name: "regular what param unaffected",
			args: json.RawMessage(`{"what":"page","correlation_id":"nav_123"}`),
			want: "page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := usageKey(tt.args)
			if got != tt.want {
				t.Errorf("usageKey(%s) = %q, want %q", string(tt.args), got, tt.want)
			}
		})
	}
}

func TestGetUsageTracker(t *testing.T) {
	t.Run("happy path returns counter", func(t *testing.T) {
		handler := createTestToolHandler(t)
		counter := telemetry.NewUsageTracker()
		handler.usageTracker = counter

		mcpHandler := NewMCPHandler(nil, "test")
		mcpHandler.SetToolBackend(buildMCPToolBackend(handler))

		got := mcpHandler.GetUsageTracker()
		if got != counter {
			t.Fatalf("GetUsageTracker() = %p, want %p", got, counter)
		}
	})

	t.Run("nil tool handler returns nil", func(t *testing.T) {
		mcpHandler := NewMCPHandler(nil, "test")
		// toolHandler is nil — should return nil.
		got := mcpHandler.GetUsageTracker()
		if got != nil {
			t.Fatalf("GetUsageTracker() = %p, want nil when toolHandler is nil", got)
		}
	})

	t.Run("test double returns nil", func(t *testing.T) {
		mcpHandler := NewMCPHandler(nil, "test")
		setFakeToolBackend(mcpHandler, &fakeToolHandlerForMCP{})
		got := mcpHandler.GetUsageTracker()
		if got != nil {
			t.Fatalf("GetUsageTracker() = %p, want nil for test double", got)
		}
	})
}

func TestHandleToolCall_NilUsageTracker(t *testing.T) {
	handler := createTestToolHandler(t)
	// usageTracker is nil by default.

	req := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	// Verify tool dispatch works when usageTracker is nil.
	args := json.RawMessage(`{"what":"errors"}`)
	resp, handled := handler.HandleToolCall(req, "observe", args)
	if !handled {
		t.Fatal("observe tool was not handled with nil usageTracker")
	}

	// Verify the response is a valid JSON-RPC response (tool ran, not a panic).
	if resp.JSONRPC != mcp.JSONRPCVersion {
		t.Errorf("response JSONRPC = %q, want %q", resp.JSONRPC, mcp.JSONRPCVersion)
	}
}

func TestHandleToolCall_IncrementsUsageTracker(t *testing.T) {
	handler := createTestToolHandler(t)

	counter := telemetry.NewUsageTracker()
	handler.usageTracker = counter

	req := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	// Call observe with what=errors — should increment "observe:errors".
	args := json.RawMessage(`{"what":"errors"}`)
	resp, handled := handler.HandleToolCall(req, "observe", args)
	if !handled {
		t.Fatal("observe tool was not handled")
	}
	// The tool may return an error (no extension) but the counter should still increment.
	_ = resp

	counts := counter.DebugCounts()
	if counts["observe:errors"] != 1 {
		t.Fatalf("observe:errors count = %d, want 1", counts["observe:errors"])
	}
}

func TestHandleToolCall_IncrementsUsageTracker_NoWhatParam(t *testing.T) {
	handler := createTestToolHandler(t)

	counter := telemetry.NewUsageTracker()
	handler.usageTracker = counter

	req := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	// Call configure with no "what" — should increment "configure:unknown".
	args := json.RawMessage(`{"key":"value"}`)
	resp, handled := handler.HandleToolCall(req, "configure", args)
	if !handled {
		t.Fatal("configure tool was not handled")
	}
	_ = resp

	counts := counter.DebugCounts()
	if counts["configure:unknown"] != 1 {
		t.Fatalf("configure:unknown count = %d, want 1", counts["configure:unknown"])
	}
}

func TestHandleToolCall_RecordsLatency(t *testing.T) {
	handler := createTestToolHandler(t)

	counter := telemetry.NewUsageTracker()
	handler.usageTracker = counter

	req := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"what":"errors"}`)
	handler.HandleToolCall(req, "observe", args)

	snapshot := counter.SwapAndReset()
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	found := false
	for _, s := range snapshot.ToolStats {
		if s.Tool == "observe:errors" {
			found = true
			// Latency should be >= 0ms.
			if s.LatencyAvgMs < 0 {
				t.Fatalf("LatencyAvgMs = %d, want >= 0", s.LatencyAvgMs)
			}
		}
	}
	if !found {
		t.Fatal("observe:errors not found in tool stats")
	}
}

func TestHandleToolCall_RecordsErrorRate(t *testing.T) {
	handler := createTestToolHandler(t)

	counter := telemetry.NewUsageTracker()
	handler.usageTracker = counter

	req := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	// Call interact with missing required params — guaranteed to produce an error.
	args := json.RawMessage(`{}`)
	handler.HandleToolCall(req, "interact", args)

	counts := counter.DebugCounts()
	if counts["interact:unknown"] != 1 {
		t.Fatalf("interact:unknown = %d, want 1", counts["interact:unknown"])
	}
	if counts["err:interact:unknown"] != 1 {
		t.Fatalf("err:interact:unknown = %d, want 1 (missing params = error)", counts["err:interact:unknown"])
	}
}

func TestHandleToolCall_TracksSessionDepth(t *testing.T) {
	handler := createTestToolHandler(t)

	counter := telemetry.NewUsageTracker()
	handler.usageTracker = counter

	req := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	// Make 3 tool calls.
	for i := 0; i < 3; i++ {
		handler.HandleToolCall(req, "observe", json.RawMessage(`{"what":"errors"}`))
	}

	if counter.SessionDepth() != 3 {
		t.Fatalf("session depth = %d, want 3", counter.SessionDepth())
	}

	snapshot := counter.SwapAndReset()
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	// SessionDepth is tracked on the tracker (not in snapshot — removed from contract).
	if counter.SessionDepth() != 3 {
		t.Fatalf("session depth = %d, want 3", counter.SessionDepth())
	}
}

func TestHandleToolCall_NilUsageTracker_NoPanic(t *testing.T) {
	handler := createTestToolHandler(t)
	// usageTracker is nil by default — should not panic.

	req := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
	}

	args := json.RawMessage(`{"what":"errors"}`)
	_, handled := handler.HandleToolCall(req, "observe", args)
	if !handled {
		t.Fatal("observe tool was not handled")
	}
	// If we got here without panic, the test passes.
}
