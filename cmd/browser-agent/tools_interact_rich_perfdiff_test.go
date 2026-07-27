// Purpose: Tests for interact rich performance-diff formatting.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_rich_perfdiff_test.go — TDD tests for Rich Action perf_diff enrichment.
// Tests the daemon-side contract for perf_diff computation on command_result,
// including before-snapshot stashing, Web Vitals, and timing.
//
// Run: go test ./cmd/browser-agent -run "TestRichAction_(Refresh|Navigate|CommandResult|PerfDiff)" -v
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

// ============================================
// PerfDiff enrichment: refresh command_result
// ============================================

func TestRichAction_RefreshStoresBeforeSnapshot(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SetTrackingStatusForTest(1, "https://example.com/dashboard")

	// Seed a perf snapshot for the tracked URL's path
	env.capture.AddPerformanceSnapshots([]performance.PerformanceSnapshot{{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing:    performance.PerformanceTiming{TimeToFirstByte: 120, DomContentLoaded: 800, Load: 1500},
	}})

	// Call refresh — should stash the before-snapshot
	result, ok := env.callInteract(t, `{"what":"refresh","background":true}`)
	if !ok {
		t.Fatal("refresh should return result")
	}
	if result.IsError {
		t.Fatalf("refresh with pilot enabled should not error. Got: %s", result.Content[0].Text)
	}

	// Extract correlation_id from result
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}
	corrID, _ := resultData["correlation_id"].(string)
	if corrID == "" {
		t.Fatal("No correlation_id in refresh result")
	}

	// Verify before-snapshot was stored
	snap, ok := env.capture.GetAndDeleteBeforeSnapshot(corrID)
	if !ok {
		t.Fatal("Before-snapshot should have been stored for refresh correlation_id")
	}
	if snap.URL != "/dashboard" {
		t.Errorf("Before-snapshot URL = %q, want /dashboard", snap.URL)
	}
	if snap.Timing.TimeToFirstByte != 120 {
		t.Errorf("Before-snapshot TTFB = %v, want 120", snap.Timing.TimeToFirstByte)
	}
}

func TestRichAction_NavigateStoresBeforeSnapshot(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SetTrackingStatusForTest(1, "https://example.com/dashboard")

	// Seed a perf snapshot for the tracked URL's path
	env.capture.AddPerformanceSnapshots([]performance.PerformanceSnapshot{{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing:    performance.PerformanceTiming{TimeToFirstByte: 115, DomContentLoaded: 700, Load: 1200},
	}})

	result, ok := env.callInteract(t, `{"what":"navigate","url":"https://example.com/settings","background":true}`)
	if !ok {
		t.Fatal("navigate should return result")
	}
	if result.IsError {
		t.Fatalf("navigate with pilot enabled should not error. Got: %s", result.Content[0].Text)
	}

	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}
	corrID, _ := resultData["correlation_id"].(string)
	if corrID == "" {
		t.Fatal("No correlation_id in navigate result")
	}

	// Verify before-snapshot was stored for perf_diff computation.
	snap, ok := env.capture.GetAndDeleteBeforeSnapshot(corrID)
	if !ok {
		t.Fatal("Before-snapshot should have been stored for navigate correlation_id")
	}
	if snap.URL != "/dashboard" {
		t.Errorf("Before-snapshot URL = %q, want /dashboard", snap.URL)
	}
	if snap.Timing.TimeToFirstByte != 115 {
		t.Errorf("Before-snapshot TTFB = %v, want 115", snap.Timing.TimeToFirstByte)
	}
}

func TestRichAction_CommandResultEnrichedWithPerfDiff(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SetTrackingStatusForTest(1, "https://example.com/dashboard")

	// Seed "before" snapshot
	env.capture.AddPerformanceSnapshots([]performance.PerformanceSnapshot{{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing:    performance.PerformanceTiming{TimeToFirstByte: 200, DomContentLoaded: 1000, Load: 2000},
		Network:   performance.NetworkSummary{TransferSize: 500000, RequestCount: 40},
	}})

	// Call refresh to stash the before-snapshot
	result, _ := env.callInteract(t, `{"what":"refresh","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	// Simulate extension sending the "after" snapshot (overwrites the old one)
	env.capture.AddPerformanceSnapshots([]performance.PerformanceSnapshot{{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:05Z",
		Timing:    performance.PerformanceTiming{TimeToFirstByte: 100, DomContentLoaded: 600, Load: 1200},
		Network:   performance.NetworkSummary{TransferSize: 300000, RequestCount: 30},
	}})

	// Simulate extension completing the command
	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true,"action":"refresh"}`), "")

	// Now observe the command_result — should include perf_diff
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	if resp.Result == nil {
		t.Fatal("No result from toolObserveCommandResult")
	}

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse observe result: %v", err)
	}
	if observeResult.IsError {
		t.Fatalf("observe should not error. Got: %s", observeResult.Content[0].Text)
	}

	// Parse the JSON response to check for perf_diff
	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	perfDiff, exists := responseData["perf_diff"]
	if !exists {
		t.Fatal("perf_diff missing from command_result response")
	}

	diffMap, ok := perfDiff.(map[string]any)
	if !ok {
		t.Fatal("perf_diff is not an object")
	}

	// Verify verdict
	verdict, _ := diffMap["verdict"].(string)
	if verdict != "improved" {
		t.Errorf("verdict = %q, want 'improved' (all metrics got better)", verdict)
	}

	// Verify summary exists
	summary, _ := diffMap["summary"].(string)
	if summary == "" {
		t.Error("perf_diff.summary should not be empty")
	}

	// Verify metrics exist
	metrics, _ := diffMap["metrics"].(map[string]any)
	if metrics == nil {
		t.Fatal("perf_diff.metrics missing")
	}
	if _, hasTTFB := metrics["ttfb"]; !hasTTFB {
		t.Error("perf_diff.metrics should include ttfb")
	}
}

func TestRichAction_CommandResultNoPerfDiffWhenNoSnapshots(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)
	// No tracking status set, no snapshots

	// Call refresh — no before-snapshot available
	result, _ := env.callInteract(t, `{"what":"refresh","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	// Complete the command
	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true,"action":"refresh"}`), "")

	// Observe — should return without perf_diff (no crash)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if _, exists := responseData["perf_diff"]; exists {
		t.Error("perf_diff should NOT be present when no before-snapshot exists")
	}
}

func TestRichAction_CommandResultIncludesTimingMs(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Click action
	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	// Simulate extension completing the click after a small delay
	time.Sleep(10 * time.Millisecond)
	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true,"action":"click"}`), "")

	// Observe command_result
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)
	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	timingMs, exists := responseData["timing_ms"]
	if !exists {
		t.Fatal("timing_ms missing from completed command result")
	}
	tm, ok := timingMs.(float64)
	if !ok {
		t.Fatalf("timing_ms should be a number, got %T", timingMs)
	}
	if tm < 0 {
		t.Errorf("timing_ms should be non-negative, got %v", tm)
	}
}

func TestRichAction_PerfDiffWithFullWebVitals(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)
	env.capture.SetTrackingStatusForTest(1, "https://example.com/dashboard")

	fcp := 3500.0
	lcp := 4500.0
	cls := 0.3

	// Seed before snapshot with ALL Web Vitals populated
	env.capture.AddPerformanceSnapshots([]performance.PerformanceSnapshot{{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: performance.PerformanceTiming{
			TimeToFirstByte:        900,
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
			DomContentLoaded:       1200,
			Load:                   2500,
		},
		CLS:     &cls,
		Network: performance.NetworkSummary{TransferSize: 800000, RequestCount: 60},
	}})

	// Call refresh to stash before-snapshot
	result, _ := env.callInteract(t, `{"what":"refresh","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	// Simulate improved "after" snapshot with all Web Vitals
	fcp2 := 800.0
	lcp2 := 1200.0
	cls2 := 0.02
	env.capture.AddPerformanceSnapshots([]performance.PerformanceSnapshot{{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:05Z",
		Timing: performance.PerformanceTiming{
			TimeToFirstByte:        150,
			FirstContentfulPaint:   &fcp2,
			LargestContentfulPaint: &lcp2,
			DomContentLoaded:       500,
			Load:                   1000,
		},
		CLS:     &cls2,
		Network: performance.NetworkSummary{TransferSize: 300000, RequestCount: 25},
	}})

	env.capture.CompleteCommand(corrID, json.RawMessage(`{"success":true,"action":"refresh"}`), "")

	// Observe command_result
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	perfDiff, exists := responseData["perf_diff"]
	if !exists {
		t.Fatal("perf_diff missing from command_result with full Web Vitals")
	}
	diffMap := perfDiff.(map[string]any)
	metrics := diffMap["metrics"].(map[string]any)

	// FCP must be present with "good" rating
	fcpMetric, hasFCP := metrics["fcp"]
	if !hasFCP {
		t.Fatal("perf_diff.metrics missing 'fcp' — FCP not wired through snapshot→diff pipeline")
	}
	fcpMap := fcpMetric.(map[string]any)
	if fcpMap["rating"] != "good" {
		t.Errorf("FCP 800ms rating = %q, want 'good'", fcpMap["rating"])
	}

	// LCP must be present with "good" rating
	lcpMetric, hasLCP := metrics["lcp"]
	if !hasLCP {
		t.Fatal("perf_diff.metrics missing 'lcp'")
	}
	lcpMap := lcpMetric.(map[string]any)
	if lcpMap["rating"] != "good" {
		t.Errorf("LCP 1200ms rating = %q, want 'good'", lcpMap["rating"])
	}

	// CLS must be present with "good" rating
	clsMetric, hasCLS := metrics["cls"]
	if !hasCLS {
		t.Fatal("perf_diff.metrics missing 'cls' — CLS not wired through snapshot→diff pipeline")
	}
	clsMap := clsMetric.(map[string]any)
	if clsMap["rating"] != "good" {
		t.Errorf("CLS 0.02 rating = %q, want 'good'", clsMap["rating"])
	}

	// TTFB must be present with "good" rating
	ttfbMetric, hasTTFB := metrics["ttfb"]
	if !hasTTFB {
		t.Fatal("perf_diff.metrics missing 'ttfb'")
	}
	ttfbMap := ttfbMetric.(map[string]any)
	if ttfbMap["rating"] != "good" {
		t.Errorf("TTFB 150ms rating = %q, want 'good'", ttfbMap["rating"])
	}

	// Verdict must be "improved"
	if diffMap["verdict"] != "improved" {
		t.Errorf("verdict = %q, want 'improved'", diffMap["verdict"])
	}
}

// ============================================
// Bug reproduction: rAF hang → missing dom_summary
// ============================================

// TestRichAction_CompactClickMissingDomSummary_WhenExtensionHangs demonstrates
// the downstream impact of the rAF hang bug in dom-primitives.ts.
// When the extension's withMutationTracking Promise never resolves, the command
// expires and neither timing_ms nor dom_summary are available.
// This is exactly what smoke test 9.2 observes: "timeout waiting for click".
func TestRichAction_CompactClickMissingDomSummary_WhenExtensionHangs(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Queue a click command
	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	if !ok || result.IsError {
		t.Fatal("click should succeed with pilot enabled")
	}

	pq := env.capture.GetLastPendingQuery()
	corrID := pq.CorrelationID

	// BUG SCENARIO: Extension never responds because withMutationTracking
	// is stuck waiting for requestAnimationFrame in a backgrounded tab.
	// Simulate this by expiring the command (no CompleteCommand call).
	env.capture.ExpireCommand(corrID)

	// Observe the expired command
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// The command expired — IsError should be true
	if !observeResult.IsError {
		t.Error("Expired command (rAF hang) MUST set IsError=true")
	}

	text := observeResult.Content[0].Text

	// KEY ASSERTION: timing_ms and dom_summary are both missing
	// This is exactly what smoke test 9.2 observes as NO_JSON/timeout
	if strings.Contains(text, "timing_ms") {
		t.Error("Expired command should NOT contain timing_ms — extension never completed")
	}
	if strings.Contains(text, "dom_summary") {
		t.Error("Expired command should NOT contain dom_summary — extension never completed")
	}
}

// TestRichAction_CompactClickHasDomSummary_WhenExtensionResponds verifies that
// when the extension DOES respond with dom_summary (rAF works), it passes through.
// This test passes today — the Go server correctly passes through extension fields.
// It exists to confirm the server isn't the problem.
func TestRichAction_CompactClickHasDomSummary_WhenExtensionResponds(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Click without analyze:true (compact mode)
	result, _ := env.callInteract(t, `{"what":"click","selector":"#btn","background":true}`)
	var resultData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[0].Text)), &resultData); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := resultData["correlation_id"].(string)

	// Simulate extension responding WITH dom_summary (compact mode, no analyze)
	extensionResult := json.RawMessage(`{
		"success": true,
		"action": "click",
		"selector": "#btn",
		"dom_summary": "1 added"
	}`)
	env.capture.CompleteCommand(corrID, extensionResult, "")

	// Observe command_result
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)

	var observeResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &observeResult); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	var responseData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(observeResult.Content[0].Text)), &responseData); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// timing_ms should be at the top level (Go server adds this)
	if _, exists := responseData["timing_ms"]; !exists {
		t.Error("timing_ms should be present at top level (added by Go server)")
	}

	// dom_summary should be promoted to top-level and stripped from nested result.
	if responseData["dom_summary"] != "1 added" {
		t.Errorf("dom_summary = %q, want '1 added'", responseData["dom_summary"])
	}

	extResult, ok := responseData["result"].(map[string]any)
	if !ok {
		t.Fatal("response should contain 'result' object")
	}
	if _, exists := extResult["dom_summary"]; exists {
		t.Fatal("nested result should not duplicate dom_summary after enrichment")
	}
}
