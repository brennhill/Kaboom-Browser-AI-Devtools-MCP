// Purpose: Coverage-expansion tests for browser-agent edge cases and branch paths.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_observe_coverage_test.go — Coverage tests for observe sub-handlers.
package main

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
)

// ============================================
// toolGetWebVitals — 0% → 100%
// ============================================

func TestToolGetWebVitals_Empty(t *testing.T) {
	t.Parallel()
	env := newObserveTestEnv(t)

	result, ok := env.callObserve(t, "vitals")
	if !ok {
		t.Fatal("vitals should return result")
	}
	if result.IsError {
		t.Fatalf("vitals should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	metrics, _ := data["metrics"].(map[string]any)
	if metrics == nil {
		t.Fatal("expected metrics in response")
	}
	if hasData, _ := metrics["has_data"].(bool); hasData {
		t.Fatal("has_data should be false with no snapshots")
	}
}

func TestToolGetWebVitals_WithData(t *testing.T) {
	t.Parallel()
	env := newObserveTestEnv(t)

	lcp := 2500.0
	fcp := 1200.0
	cls := 0.05
	env.capture.Performance().Add([]performance.PerformanceSnapshot{
		{
			URL:       "https://example.com",
			Timestamp: "2024-01-01T00:00:00Z",
			Timing: performance.PerformanceTiming{
				LargestContentfulPaint: &lcp,
				FirstContentfulPaint:   &fcp,
				DomContentLoaded:       800,
				Load:                   3000,
			},
			CLS: &cls,
		},
	})

	result, ok := env.callObserve(t, "vitals")
	if !ok {
		t.Fatal("vitals should return result")
	}
	if result.IsError {
		t.Fatalf("vitals should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	metrics, _ := data["metrics"].(map[string]any)
	if metrics == nil {
		t.Fatal("expected metrics in response")
	}
	if hasData, _ := metrics["has_data"].(bool); !hasData {
		t.Fatal("has_data should be true with snapshots")
	}
	if _, ok := metrics["lcp"]; !ok {
		t.Error("expected lcp in metrics")
	}
	if _, ok := metrics["fcp"]; !ok {
		t.Error("expected fcp in metrics")
	}
	if _, ok := metrics["cls"]; !ok {
		t.Error("expected cls in metrics")
	}
}

// ============================================
// toolAnalyzeErrors — 0% → 80%+
// ============================================

func TestToolAnalyzeErrors_NoErrors(t *testing.T) {
	t.Parallel()
	env := newObserveTestEnv(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := observe.AnalyzeErrors(env.handler, req, nil)

	result := parseToolResult(t, resp)
	data := parseResponseJSON(t, result)
	count, _ := data["total_count"].(float64)
	if count != 0 {
		t.Fatalf("total_count = %v, want 0", count)
	}
}

func TestToolAnalyzeErrors_WithClusters(t *testing.T) {
	t.Parallel()
	env := newObserveTestEnv(t)

	env.addLogEntry(types.LogEntry{
		"level":     "error",
		"message":   "TypeError: Cannot read property 'foo' of null",
		"timestamp": "2024-01-01T00:00:01Z",
		"url":       "https://example.com/app.js",
	})
	env.addLogEntry(types.LogEntry{
		"level":     "error",
		"message":   "TypeError: Cannot read property 'foo' of null",
		"timestamp": "2024-01-01T00:00:02Z",
		"url":       "https://example.com/app.js",
	})
	env.addLogEntry(types.LogEntry{
		"level":   "info",
		"message": "Application loaded",
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := observe.AnalyzeErrors(env.handler, req, nil)

	result := parseToolResult(t, resp)
	data := parseResponseJSON(t, result)
	count, _ := data["total_count"].(float64)
	if count != 1 {
		t.Fatalf("total_count = %v, want 1 (2 errors should cluster into 1)", count)
	}

	clusters, _ := data["clusters"].([]any)
	if len(clusters) != 1 {
		t.Fatalf("clusters len = %d, want 1", len(clusters))
	}
	cluster, _ := clusters[0].(map[string]any)
	clusterCount, _ := cluster["count"].(float64)
	if clusterCount != 2 {
		t.Fatalf("cluster count = %v, want 2", clusterCount)
	}
}

// ============================================
// toolAnalyzeHistory — 0% → 100%
// ============================================

func TestToolAnalyzeHistory_Empty(t *testing.T) {
	t.Parallel()
	env := newObserveTestEnv(t)

	args := json.RawMessage(`{"what":"history"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := observe.AnalyzeHistory(env.handler, req, args)

	result := parseToolResult(t, resp)
	data := parseResponseJSON(t, result)
	count, _ := data["count"].(float64)
	if count != 0 {
		t.Fatalf("count = %v, want 0", count)
	}
}

func TestToolAnalyzeHistory_WithNavigations(t *testing.T) {
	t.Parallel()
	env := newObserveTestEnv(t)

	env.capture.AddEnhancedActionsForTest([]types.EnhancedAction{
		{Type: "navigate", Timestamp: 1704067201000, ToURL: "https://example.com/page1", FromURL: "https://example.com"},
		{Type: "click", Timestamp: 1704067202000, URL: "https://example.com/page2"},
		{Type: "navigate", Timestamp: 1704067203000, ToURL: "https://example.com/page1"}, // Duplicate URL — should be deduped
	})

	args := json.RawMessage(`{"what":"history"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := observe.AnalyzeHistory(env.handler, req, args)

	result := parseToolResult(t, resp)
	data := parseResponseJSON(t, result)
	count, _ := data["count"].(float64)
	if count != 2 {
		t.Fatalf("count = %v, want 2 (navigate + click URL, duplicate navigate deduped)", count)
	}
}

// ============================================
// toolGetScreenshot — 0% → partial (tracking disabled path)
// ============================================

func TestToolGetScreenshot_TrackingDisabled(t *testing.T) {
	t.Parallel()
	env := newObserveTestEnv(t)

	args := json.RawMessage(`{"what":"screenshot"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := observe.GetScreenshot(env.handler, req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("screenshot without tracking should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "tab") {
		t.Errorf("error should mention tab tracking, got: %s", text)
	}
}

// ============================================
// toolRunA11yAudit — 0% → partial (tracking disabled path)
// ============================================

func TestToolRunA11yAudit_TrackingDisabled(t *testing.T) {
	t.Parallel()
	env := newObserveTestEnv(t)

	args := json.RawMessage(`{"what":"accessibility"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := observe.RunA11yAudit(env.handler, req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("a11y without tracking should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "tab") {
		t.Errorf("error should mention tab tracking, got: %s", text)
	}
}
