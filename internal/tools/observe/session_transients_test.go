// handlers_transients_test.go — Tests for GetTransients and GetEnhancedActions type filter.
package observe

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// mockTransientDeps provides a minimal Deps implementation for transient tests.
type mockTransientDeps struct {
	cap *capture.Store
}

func (m *mockTransientDeps) DiagnosticHintString() string                 { return "" }
func (m *mockTransientDeps) GetCapture() *capture.Store                   { return m.cap }
func (m *mockTransientDeps) GetLogEntries() ([]mcp.LogEntry, []time.Time) { return nil, nil }
func (m *mockTransientDeps) GetLogTotalAdded() int64                      { return 0 }
func (m *mockTransientDeps) IsConsoleNoise(_ mcp.LogEntry) bool           { return false }
func (m *mockTransientDeps) ExecuteA11yQuery(_ string, _ []string, _ any, _ bool) (json.RawMessage, error) {
	return nil, nil
}

func seedTransientActions(c *capture.Store) {
	c.AddEnhancedActionsForTest([]capture.EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "https://example.com"},
		{Type: "transient", Timestamp: 2000, URL: "https://example.com", Classification: "toast", Value: "Saved", Role: "status"},
		{Type: "transient", Timestamp: 3000, URL: "https://example.com", Classification: "alert", Value: "Error occurred", Role: "alert"},
		{Type: "input", Timestamp: 4000, URL: "https://example.com"},
		{Type: "transient", Timestamp: 5000, URL: "https://other.com", Classification: "snackbar", Value: "Undo?", Role: "status"},
	})
}

// extractMCPJSON parses the MCP tool result text which is "summary\n{json}" format.
func extractMCPJSON(t *testing.T, resp mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("No content blocks in response")
	}
	text := result.Content[0].Text
	// Split on first newline — summary is before, JSON is after
	idx := strings.Index(text, "\n")
	if idx < 0 {
		t.Fatalf("No newline in response text: %s", text)
	}
	jsonText := text[idx+1:]
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Unmarshal JSON from text: %v\nText: %s", err, jsonText)
	}
	return data
}

func TestGetTransients_FiltersTransientType(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	deps := &mockTransientDeps{cap: c}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetTransients(deps, req, json.RawMessage(`{}`))
	data := extractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 3 {
		t.Errorf("count = %v, want 3", count)
	}
}

func TestGetTransients_FiltersByClassification(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	deps := &mockTransientDeps{cap: c}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetTransients(deps, req, json.RawMessage(`{"classification":"toast"}`))
	data := extractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 1 {
		t.Errorf("count = %v, want 1 (only toast)", count)
	}
}

func TestGetTransients_FiltersByURL(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	deps := &mockTransientDeps{cap: c}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetTransients(deps, req, json.RawMessage(`{"url":"other.com"}`))
	data := extractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 1 {
		t.Errorf("count = %v, want 1 (only other.com)", count)
	}
}

func TestGetTransients_EmptyBuffer(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	deps := &mockTransientDeps{cap: c}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetTransients(deps, req, json.RawMessage(`{}`))
	data := extractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 0 {
		t.Errorf("count = %v, want 0", count)
	}
}

func TestGetTransients_Limit(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	deps := &mockTransientDeps{cap: c}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetTransients(deps, req, json.RawMessage(`{"limit":2}`))
	data := extractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 2 {
		t.Errorf("count = %v, want 2 (limited)", count)
	}
}

func TestGetTransients_SummaryMode(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	deps := &mockTransientDeps{cap: c}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetTransients(deps, req, json.RawMessage(`{"summary":true}`))
	data := extractMCPJSON(t, resp)

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
	deps := &mockTransientDeps{cap: c}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	// Only snackbar on other.com should match
	resp := GetTransients(deps, req, json.RawMessage(`{"classification":"snackbar","url":"other.com"}`))
	data := extractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 1 {
		t.Errorf("count = %v, want 1 (snackbar on other.com)", count)
	}
}

func TestGetTransients_CombinedFilterNoMatch(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	deps := &mockTransientDeps{cap: c}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	// toast is only on example.com, not other.com
	resp := GetTransients(deps, req, json.RawMessage(`{"classification":"toast","url":"other.com"}`))
	data := extractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 0 {
		t.Errorf("count = %v, want 0 (no toast on other.com)", count)
	}
}

func TestGetEnhancedActions_TypeFilter(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	deps := &mockTransientDeps{cap: c}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetEnhancedActions(deps, req, json.RawMessage(`{"type":"click"}`))
	data := extractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 1 {
		t.Errorf("count = %v, want 1 (only click)", count)
	}
}

func TestGetEnhancedActions_TypeFilterTransient(t *testing.T) {
	t.Parallel()
	c := capture.NewCapture()
	seedTransientActions(c)
	deps := &mockTransientDeps{cap: c}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetEnhancedActions(deps, req, json.RawMessage(`{"type":"transient"}`))
	data := extractMCPJSON(t, resp)

	count, _ := data["count"].(float64)
	if int(count) != 3 {
		t.Errorf("count = %v, want 3 (only transient)", count)
	}
}
