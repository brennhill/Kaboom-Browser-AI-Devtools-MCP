// Purpose: Tests for the live-page observe modes (accessibility audit, screenshot data URLs).
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Mock Deps for RunA11yAudit tests
// ============================================

type mockA11yDeps struct {
	cap           *capture.Capture
	a11yResult    json.RawMessage
	a11yErr       error
	diagnosticStr string
}

func (m *mockA11yDeps) DiagnosticHintString() string { return m.diagnosticStr }

func (m *mockA11yDeps) GetCapture() *capture.Capture { return m.cap }

func (m *mockA11yDeps) GetLogEntries() ([]types.LogEntry, []time.Time) {
	return nil, nil
}

func (m *mockA11yDeps) GetLogTotalAdded() int64 { return 0 }

func (m *mockA11yDeps) ExecuteA11yQuery(_ string, _ []string, _ any, _ bool) (json.RawMessage, error) {
	return m.a11yResult, m.a11yErr
}

func (m *mockA11yDeps) IsConsoleNoise(_ types.LogEntry) bool { return false }

// ============================================
// Waterfall Summary Tests
// ============================================

func TestBuildA11ySummary_Compact(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"passes": []any{
			map[string]any{"id": "rule1"},
			map[string]any{"id": "rule2"},
		},
		"violations": []any{
			map[string]any{
				"id":     "color-contrast",
				"impact": "serious",
				"nodes":  []any{map[string]any{}, map[string]any{}, map[string]any{}},
			},
			map[string]any{
				"id":     "image-alt",
				"impact": "critical",
				"nodes":  []any{map[string]any{}},
			},
		},
		"incomplete": []any{
			map[string]any{"id": "aria-label"},
		},
	}

	result := buildA11ySummary(auditResult)

	if result["pass"] != 2 {
		t.Errorf("pass = %v, want 2", result["pass"])
	}
	if result["violations"] != 2 {
		t.Errorf("violations = %v, want 2", result["violations"])
	}
	if result["incomplete"] != 1 {
		t.Errorf("incomplete = %v, want 1", result["incomplete"])
	}

	topIssues, ok := result["top_issues"].([]map[string]any)
	if !ok {
		t.Fatalf("top_issues wrong type: %T", result["top_issues"])
	}
	if len(topIssues) != 2 {
		t.Fatalf("expected 2 top issues, got %d", len(topIssues))
	}
	// Should be sorted by node count descending
	if topIssues[0]["rule"] != "color-contrast" {
		t.Errorf("first issue = %v, want color-contrast", topIssues[0]["rule"])
	}
	if topIssues[0]["count"] != 3 {
		t.Errorf("first issue count = %v, want 3", topIssues[0]["count"])
	}
	if topIssues[0]["severity"] != "serious" {
		t.Errorf("first issue severity = %v, want serious", topIssues[0]["severity"])
	}
}

func TestBuildA11ySummary_Empty(t *testing.T) {
	t.Parallel()
	result := buildA11ySummary(map[string]any{})
	if result["pass"] != 0 {
		t.Errorf("pass = %v, want 0", result["pass"])
	}
	if result["violations"] != 0 {
		t.Errorf("violations = %v, want 0", result["violations"])
	}
}

func TestBuildA11ySummary_TopIssuesLimitedTo5(t *testing.T) {
	t.Parallel()
	violations := make([]any, 7)
	for i := range violations {
		violations[i] = map[string]any{
			"id":     "rule-" + string(rune('a'+i)),
			"impact": "minor",
			"nodes":  []any{map[string]any{}},
		}
	}
	auditResult := map[string]any{"violations": violations}

	result := buildA11ySummary(auditResult)
	topIssues := result["top_issues"].([]map[string]any)
	if len(topIssues) != 5 {
		t.Errorf("expected 5 top issues (capped), got %d", len(topIssues))
	}
}

// ============================================
// Issue #276: A11y Audit Partial Results Tests
// ============================================

func TestRunA11yAudit_TimeoutReturnsPartialResults(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	cap.SetTrackingStatusForTest(1, "https://example.com")

	deps := &mockA11yDeps{
		cap:     cap,
		a11yErr: errors.New("context deadline exceeded"),
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := RunA11yAudit(deps, req, json.RawMessage(`{}`))

	// Should NOT be an error response — should return partial results gracefully
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result.IsError {
		t.Fatal("timeout should return partial results, not isError:true")
	}

	// Should contain an error field in the data
	text := result.Content[0].Text
	if !strings.Contains(text, "error") {
		t.Errorf("partial result should contain 'error' field, got: %s", text)
	}
	if !strings.Contains(text, "timeout") && !strings.Contains(text, "deadline") {
		t.Errorf("partial result should mention timeout or deadline, got: %s", text)
	}

	// Should have empty violations/passes arrays and partial flag (partial result structure)
	var data map[string]any
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatal("partial result text should contain JSON object")
	}
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("failed to parse partial result JSON: %v", err)
	}
	if _, ok := data["violations"]; !ok {
		t.Error("partial result should include 'violations' field")
	}
	if _, ok := data["summary"]; !ok {
		t.Error("partial result should include 'summary' field")
	}
	summary, ok := data["summary"].(map[string]any)
	if !ok {
		t.Fatalf("partial result summary should be object, got %T", data["summary"])
	}
	if summary["violations"] != float64(0) {
		t.Errorf("expected zero violations summary, got %+v", summary)
	}
	if summary["passes"] != float64(0) {
		t.Errorf("expected zero passes summary, got %+v", summary)
	}
	if data["partial"] != true {
		t.Errorf("partial result should have partial=true, got: %v", data["partial"])
	}
}

func TestRunA11yAudit_AlreadyRunningReturnsPartialResults(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	cap.SetTrackingStatusForTest(1, "https://example.com")

	deps := &mockA11yDeps{
		cap:     cap,
		a11yErr: errors.New("Axe is already running"),
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := RunA11yAudit(deps, req, json.RawMessage(`{}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result.IsError {
		t.Fatal("already-running should return partial results, not isError:true")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "error") {
		t.Errorf("partial result should contain error field, got: %s", text)
	}
	if !strings.Contains(text, "already running") {
		t.Errorf("partial result should mention 'already running', got: %s", text)
	}

	// Verify partial flag is set
	var data map[string]any
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatal("partial result text should contain JSON object")
	}
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("failed to parse partial result JSON: %v", err)
	}
	if data["partial"] != true {
		t.Errorf("partial result should have partial=true, got: %v", data["partial"])
	}
}

// ============================================
// parseDataURL Tests
// ============================================

func TestParseDataURL_ValidJPEG(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("data:image/jpeg;base64,/9j/4AAQSkZJRg==")
	if data != "/9j/4AAQSkZJRg==" {
		t.Errorf("base64Data = %q, want %q", data, "/9j/4AAQSkZJRg==")
	}
	if mime != "image/jpeg" {
		t.Errorf("mimeType = %q, want %q", mime, "image/jpeg")
	}
}

func TestParseDataURL_ValidPNG(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("data:image/png;base64,iVBORw0KGgo=")
	if data != "iVBORw0KGgo=" {
		t.Errorf("base64Data = %q, want %q", data, "iVBORw0KGgo=")
	}
	if mime != "image/png" {
		t.Errorf("mimeType = %q, want %q", mime, "image/png")
	}
}

func TestParseDataURL_MalformedNoDataPrefix(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("image/jpeg;base64,/9j/4AAQ")
	if data != "" || mime != "" {
		t.Errorf("expected empty strings for missing data: prefix, got data=%q mime=%q", data, mime)
	}
}

func TestParseDataURL_MalformedNoBase64Marker(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("data:image/jpeg;charset=utf-8,sometext")
	if data != "" || mime != "" {
		t.Errorf("expected empty strings for missing base64 marker, got data=%q mime=%q", data, mime)
	}
}

func TestParseDataURL_EmptyString(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("")
	if data != "" || mime != "" {
		t.Errorf("expected empty strings for empty input, got data=%q mime=%q", data, mime)
	}
}

func TestRunA11yAudit_ResultWithErrorFieldReturnsGracefully(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	cap.SetTrackingStatusForTest(1, "https://example.com")

	// Simulate extension returning partial results with an error field
	partialResult := map[string]any{
		"violations":   []any{},
		"passes":       []any{},
		"incomplete":   []any{},
		"inapplicable": []any{},
		"summary": map[string]any{
			"violations":   0,
			"passes":       0,
			"incomplete":   0,
			"inapplicable": 0,
		},
		"error": "Accessibility audit timeout",
	}
	resultJSON, _ := json.Marshal(partialResult)

	deps := &mockA11yDeps{
		cap:        cap,
		a11yResult: resultJSON,
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := RunA11yAudit(deps, req, json.RawMessage(`{}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	// Should NOT be isError — partial results are still useful
	if result.IsError {
		t.Fatal("result with error field should not be isError:true")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "error") {
		t.Errorf("response should preserve error field, got: %s", text)
	}
}
