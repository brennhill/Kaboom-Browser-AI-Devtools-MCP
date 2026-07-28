// coverage_topup_test.go — Small unit tests for pure helpers in the main package.

package main

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/asynccommand"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func mkToolResp(t *testing.T, text string) mcp.JSONRPCResponse {
	t.Helper()
	inner, err := json.Marshal(map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return mcp.JSONRPCResponse{Result: json.RawMessage(inner)}
}

func TestExtractErrorMessage_Branches(t *testing.T) {
	// Invalid result JSON -> "unknown error".
	if got := extractErrorMessage(mcp.JSONRPCResponse{Result: json.RawMessage("{bad")}); got != "unknown error" {
		t.Fatalf("invalid result: got %q, want %q", got, "unknown error")
	}
	// Well-formed result with empty content -> "unknown error".
	empty, _ := json.Marshal(map[string]any{"content": []map[string]any{}})
	if got := extractErrorMessage(mcp.JSONRPCResponse{Result: json.RawMessage(empty)}); got != "unknown error" {
		t.Fatalf("empty content: got %q, want %q", got, "unknown error")
	}
	// Text containing a structured error JSON -> the message field.
	if got := extractErrorMessage(mkToolResp(t, `oops: {"message":"boom"}`)); got != "boom" {
		t.Fatalf("structured: got %q, want %q", got, "boom")
	}
	// Plain text with no embedded JSON -> the text verbatim.
	if got := extractErrorMessage(mkToolResp(t, "plain failure")); got != "plain failure" {
		t.Fatalf("plain: got %q, want %q", got, "plain failure")
	}
}

func TestValidatePort_InRangeDoesNotExit(t *testing.T) {
	// In-range ports return normally; out-of-range calls os.Exit and is not exercised here.
	validatePort(1)
	validatePort(7890)
	validatePort(65535)
}

func TestAttachTraceSummary_Branches(t *testing.T) {
	// No trace identity and no events -> nothing attached.
	rd := map[string]any{}
	asynccommand.AttachTraceSummary(rd, queries.CommandResult{})
	if _, ok := rd["trace"]; ok {
		t.Fatalf("expected no trace for an empty command, got %+v", rd["trace"])
	}
	// TraceID + QueryID -> a trace map carrying both.
	rd = map[string]any{}
	asynccommand.AttachTraceSummary(rd, queries.CommandResult{TraceID: "t1", QueryID: "q1"})
	trace, ok := rd["trace"].(map[string]any)
	if !ok || trace["trace_id"] != "t1" || trace["query_id"] != "q1" {
		t.Fatalf("trace = %+v, want trace_id=t1 query_id=q1", rd["trace"])
	}
	// CorrelationID is used as the trace id when TraceID is empty.
	rd = map[string]any{}
	asynccommand.AttachTraceSummary(rd, queries.CommandResult{CorrelationID: "c1"})
	trace, _ = rd["trace"].(map[string]any)
	if trace["trace_id"] != "c1" {
		t.Fatalf("fallback trace_id = %v, want c1", trace["trace_id"])
	}
}

func TestCalcUtilization(t *testing.T) {
	// Returns a percentage: entries/capacity*100.
	if got := health.CalcUtilization(5, 10); got != 50 {
		t.Fatalf("health.CalcUtilization(5,10) = %v, want 50", got)
	}
	// capacity <= 0 short-circuits to 0.
	if got := health.CalcUtilization(3, 0); got != 0 {
		t.Fatalf("health.CalcUtilization(3,0) = %v, want 0", got)
	}
}
