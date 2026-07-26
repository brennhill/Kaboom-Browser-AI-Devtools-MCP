package toolresp

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestSucceedRawPassesResultThrough(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{"content":[{"type":"text","text":"hi"}]}`)
	resp := SucceedRaw(mcp.JSONRPCRequest{ID: float64(7)}, raw)

	if resp.JSONRPC != mcp.JSONRPCVersion {
		t.Fatalf("jsonrpc mismatch: got %q want %q", resp.JSONRPC, mcp.JSONRPCVersion)
	}
	if resp.ID != float64(7) {
		t.Fatalf("id not echoed: got %v", resp.ID)
	}
	if string(resp.Result) != string(raw) {
		t.Fatalf("result mutated: got %q want %q", string(resp.Result), string(raw))
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error field: %+v", resp.Error)
	}
}

func TestFailJSONMarksIsError(t *testing.T) {
	t.Parallel()

	resp := FailJSON(mcp.JSONRPCRequest{ID: "a"}, "broke", map[string]any{"why": "test"})

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("result is not an MCPToolResult: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected is_error=true on a FailJSON response")
	}
	if !strings.Contains(string(resp.Result), "broke") {
		t.Fatalf("summary missing from payload: %s", string(resp.Result))
	}
	// The JSON-RPC envelope stays successful; the error lives in the tool result.
	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error field: %+v", resp.Error)
	}
}

func TestNewCorrelationIDShape(t *testing.T) {
	t.Parallel()

	id := NewCorrelationID("nav")
	parts := strings.Split(id, "_")
	if len(parts) != 3 {
		t.Fatalf("want prefix_timestamp_random, got %q", id)
	}
	if parts[0] != "nav" {
		t.Fatalf("prefix not preserved: got %q", parts[0])
	}
	// usageKey() splits on the first '_' to recover the prefix, so the prefix must
	// be the only thing before it.
	if _, err := strconv.ParseInt(parts[1], 10, 64); err != nil {
		t.Fatalf("timestamp component not an int64: %q", parts[1])
	}
	if _, err := strconv.ParseInt(parts[2], 10, 64); err != nil {
		t.Fatalf("random component not an int64: %q", parts[2])
	}
}

func TestNewCorrelationIDIsUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := NewCorrelationID("x")
		if seen[id] {
			t.Fatalf("duplicate correlation ID after %d draws: %q", i, id)
		}
		seen[id] = true
	}
}

func TestRandomInt63IsNonNegative(t *testing.T) {
	t.Parallel()

	// The three pre-consolidation copies of this helper disagreed here: two of them
	// never masked the sign bit and could return a negative random component.
	for i := 0; i < 2000; i++ {
		if n := RandomInt63(); n < 0 {
			t.Fatalf("RandomInt63 returned a negative value: %d", n)
		}
	}
}

func TestRequireString(t *testing.T) {
	t.Parallel()

	if _, blocked := RequireString(mcp.JSONRPCRequest{ID: 1}, "value", "name", "hint"); blocked {
		t.Fatal("a non-empty value must not block")
	}

	resp, blocked := RequireString(mcp.JSONRPCRequest{ID: 1}, "", "name", "add it")
	if !blocked {
		t.Fatal("an empty value must block")
	}
	if !strings.Contains(string(resp.Result), "name") {
		t.Fatalf("error must name the missing param: %s", string(resp.Result))
	}
}

func TestRequireOneOf(t *testing.T) {
	t.Parallel()

	valid := []string{"a", "b"}
	if _, blocked := RequireOneOf(mcp.JSONRPCRequest{ID: 1}, "b", "mode", valid, "hint"); blocked {
		t.Fatal("a listed value must not block")
	}

	resp, blocked := RequireOneOf(mcp.JSONRPCRequest{ID: 1}, "z", "mode", valid, "pick one")
	if !blocked {
		t.Fatal("an unlisted value must block")
	}
	// The message enumerates the accepted values so the caller can self-correct.
	if !strings.Contains(string(resp.Result), "a, b") {
		t.Fatalf("error must list the valid values: %s", string(resp.Result))
	}
}
