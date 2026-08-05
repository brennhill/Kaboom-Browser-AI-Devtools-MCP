// session_transients_test.go — Tests transient and enhanced-action observation filters.
package session

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/testsupport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func seedTransientActions(c *capture.Capture) {
	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "https://example.com"},
		{Type: "transient", Timestamp: 2000, URL: "https://example.com", Classification: "toast", Value: "Saved", Role: "status"},
		{Type: "transient", Timestamp: 3000, URL: "https://example.com", Classification: "alert", Value: "Error occurred", Role: "alert"},
		{Type: "input", Timestamp: 4000, URL: "https://example.com"},
		{Type: "transient", Timestamp: 5000, URL: "https://other.com", Classification: "snackbar", Value: "Undo?", Role: "status"},
	})
}

func TestGetTransients_FiltersTransientType(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetTransients(testsupport.Deps(c), req, json.RawMessage(`{}`))
	data := testsupport.ExtractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 3 {
		t.Errorf("count = %v, want 3", count)
	}
}

func TestGetTransients_FiltersByClassification(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetTransients(testsupport.Deps(c), req, json.RawMessage(`{"classification":"toast"}`))
	data := testsupport.ExtractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 1 {
		t.Errorf("count = %v, want 1 (only toast)", count)
	}
}

func TestGetTransients_FiltersByURL(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetTransients(testsupport.Deps(c), req, json.RawMessage(`{"url":"other.com"}`))
	data := testsupport.ExtractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 1 {
		t.Errorf("count = %v, want 1 (only other.com)", count)
	}
}

func TestGetTransients_EmptyBuffer(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetTransients(testsupport.Deps(c), req, json.RawMessage(`{}`))
	data := testsupport.ExtractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 0 {
		t.Errorf("count = %v, want 0", count)
	}
}

func TestGetTransients_Limit(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetTransients(testsupport.Deps(c), req, json.RawMessage(`{"limit":2}`))
	data := testsupport.ExtractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 2 {
		t.Errorf("count = %v, want 2 (limited)", count)
	}
}

func TestGetTransients_SummaryMode(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetTransients(testsupport.Deps(c), req, json.RawMessage(`{"summary":true}`))
	data := testsupport.ExtractMCPJSON(t, resp)

	total, _ := data["total"].(float64)
	if int(total) != 3 {
		t.Errorf("total = %v, want 3", total)
	}

	byCls, ok := data["by_classification"].(map[string]any)
	if !ok {
		t.Fatal("by_classification not present")
	}
	toastCount, _ := byCls["toast"].(float64)
	if int(toastCount) != 1 {
		t.Errorf("toast count = %v, want 1", toastCount)
	}
}

func TestGetTransients_CombinedClassificationAndURL(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	// Only snackbar on other.com should match
	resp := GetTransients(testsupport.Deps(c), req, json.RawMessage(`{"classification":"snackbar","url":"other.com"}`))
	data := testsupport.ExtractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 1 {
		t.Errorf("count = %v, want 1 (snackbar on other.com)", count)
	}
}

func TestGetTransients_CombinedFilterNoMatch(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	// toast is only on example.com, not other.com
	resp := GetTransients(testsupport.Deps(c), req, json.RawMessage(`{"classification":"toast","url":"other.com"}`))
	data := testsupport.ExtractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 0 {
		t.Errorf("count = %v, want 0 (no toast on other.com)", count)
	}
}

func TestGetEnhancedActions_TypeFilter(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetEnhancedActions(testsupport.Deps(c), req, json.RawMessage(`{"type":"click"}`))
	data := testsupport.ExtractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 1 {
		t.Errorf("count = %v, want 1 (only click)", count)
	}
}

func TestGetEnhancedActions_TypeFilterTransient(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetEnhancedActions(testsupport.Deps(c), req, json.RawMessage(`{"type":"transient"}`))
	data := testsupport.ExtractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 3 {
		t.Errorf("count = %v, want 3 (only transient)", count)
	}
}
