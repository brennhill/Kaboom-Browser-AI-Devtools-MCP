// Purpose: Validate auto_dismiss_overlays and wait_for_stable interact actions.
// Why: Prevents silent regressions in cookie consent auto-dismiss and DOM stability wait.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_auto_dismiss_test.go — Tests for auto_dismiss_overlays and wait_for_stable (#342, #344).
//
// Tests verify parameter validation, pilot gating, queuing behavior, composable params,
// and default timeout handling for auto_dismiss_overlays and wait_for_stable actions.
//
// Run: go test ./cmd/browser-agent -run "TestInteract_AutoDismiss|TestInteract_WaitForStable|TestInteract_Navigate" -v
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// auto_dismiss_overlays — standalone action
// ============================================

func TestInteract_AutoDismissOverlays_DispatchesPendingQuery(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"auto_dismiss_overlays"}`)
	if !ok {
		t.Fatal("auto_dismiss_overlays should return result")
	}
	if result.IsError {
		t.Fatalf("auto_dismiss_overlays should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("auto_dismiss_overlays should create a pending query")
	}
	if pq.Type != "dom_action" {
		t.Fatalf("pending query type = %q, want dom_action", pq.Type)
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["action"] != "auto_dismiss_overlays" {
		t.Fatalf("params action = %q, want auto_dismiss_overlays", params["action"])
	}

	// Verify correlation_id prefix
	data := extractResultJSON(t, result)
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "dom_auto_dismiss_overlays_") {
		t.Errorf("correlation_id should start with 'dom_auto_dismiss_overlays_', got: %s", corr)
	}
}

func TestInteract_AutoDismissOverlays_PilotDisabled(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	// Pilot disabled by default

	result, ok := env.callInteract(t, `{"what":"auto_dismiss_overlays"}`)
	if !ok {
		t.Fatal("auto_dismiss_overlays should return result")
	}
	if !result.IsError {
		t.Error("auto_dismiss_overlays with pilot disabled should return isError:true")
	}
	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "pilot") {
			t.Errorf("error should mention pilot\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestInteract_AutoDismissOverlays_NoSelectorRequired(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	// auto_dismiss_overlays should NOT require selector — it's an intent action.
	result, ok := env.callInteract(t, `{"what":"auto_dismiss_overlays"}`)
	if !ok {
		t.Fatal("auto_dismiss_overlays should return result")
	}
	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if strings.Contains(text, "selector") && strings.Contains(text, "missing") {
			t.Errorf("auto_dismiss_overlays should NOT require selector\nGot: %s", result.Content[0].Text)
		}
	}
}

// ============================================
// navigate + auto_dismiss composable param
// ============================================

func TestInteract_Navigate_AutoDismiss_Composes(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"navigate","url":"https://example.com","auto_dismiss":true}`)
	if !ok {
		t.Fatal("navigate with auto_dismiss should return result")
	}
	if result.IsError {
		t.Fatalf("navigate with auto_dismiss should not error, got: %s", result.Content[0].Text)
	}

	// The navigate action should have created a pending query for navigation
	queries := env.capture.Queries().GetPendingQueries()
	if len(queries) < 2 {
		t.Fatalf("navigate with auto_dismiss should create at least 2 pending queries (nav + dismiss), got %d", len(queries))
	}

	// Find the auto_dismiss query
	foundDismiss := false
	for _, q := range queries {
		if q.Type == "dom_action" {
			var params map[string]any
			if err := json.Unmarshal(q.Params, &params); err != nil {
				continue
			}
			if params["action"] == "auto_dismiss_overlays" {
				foundDismiss = true
				break
			}
		}
	}
	if !foundDismiss {
		t.Error("navigate with auto_dismiss=true should queue an auto_dismiss_overlays dom_action")
	}
}

func TestInteract_Navigate_AutoDismiss_False_NoDismissQuery(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	_, ok := env.callInteract(t, `{"what":"navigate","url":"https://example.com","auto_dismiss":false}`)
	if !ok {
		t.Fatal("navigate with auto_dismiss=false should return result")
	}

	// Should NOT have a dismiss query
	queries := env.capture.Queries().GetPendingQueries()
	for _, q := range queries {
		if q.Type == "dom_action" {
			var params map[string]any
			if err := json.Unmarshal(q.Params, &params); err != nil {
				continue
			}
			if params["action"] == "auto_dismiss_overlays" {
				t.Error("navigate with auto_dismiss=false should NOT queue auto_dismiss_overlays")
			}
		}
	}
}

// ============================================
// wait_for_stable — standalone action
// ============================================

func TestInteract_WaitForStable_DispatchesPendingQuery(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"wait_for_stable"}`)
	if !ok {
		t.Fatal("wait_for_stable should return result")
	}
	if result.IsError {
		t.Fatalf("wait_for_stable should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("wait_for_stable should create a pending query")
	}
	if pq.Type != "dom_action" {
		t.Fatalf("pending query type = %q, want dom_action", pq.Type)
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["action"] != "wait_for_stable" {
		t.Fatalf("params action = %q, want wait_for_stable", params["action"])
	}
}

func TestInteract_WaitForStable_DefaultTimeout(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	// Call without explicit stability_ms — should use default 500
	result, ok := env.callInteract(t, `{"what":"wait_for_stable"}`)
	if !ok {
		t.Fatal("wait_for_stable should return result")
	}
	if result.IsError {
		t.Fatalf("wait_for_stable should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("wait_for_stable should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}

	// Default stability_ms should be 500
	stabilityMs, ok := params["stability_ms"].(float64)
	if !ok {
		t.Fatal("params should have stability_ms")
	}
	if int(stabilityMs) != 500 {
		t.Errorf("default stability_ms = %v, want 500", stabilityMs)
	}

	// Default timeout_ms should be 5000
	timeoutMs, ok := params["timeout_ms"].(float64)
	if !ok {
		t.Fatal("params should have timeout_ms")
	}
	if int(timeoutMs) != 5000 {
		t.Errorf("default timeout_ms = %v, want 5000", timeoutMs)
	}
}

func TestInteract_WaitForStable_CustomParams(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"wait_for_stable","stability_ms":1000,"timeout_ms":10000}`)
	if !ok {
		t.Fatal("wait_for_stable should return result")
	}
	if result.IsError {
		t.Fatalf("wait_for_stable should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("wait_for_stable should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}

	if int(params["stability_ms"].(float64)) != 1000 {
		t.Errorf("stability_ms = %v, want 1000", params["stability_ms"])
	}
	if int(params["timeout_ms"].(float64)) != 10000 {
		t.Errorf("timeout_ms = %v, want 10000", params["timeout_ms"])
	}
}

func TestInteract_WaitForStable_PilotDisabled(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"wait_for_stable"}`)
	if !ok {
		t.Fatal("wait_for_stable should return result")
	}
	if !result.IsError {
		t.Error("wait_for_stable with pilot disabled should return isError:true")
	}
	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "pilot") {
			t.Errorf("error should mention pilot\nGot: %s", result.Content[0].Text)
		}
	}
}

// ============================================
// navigate + wait_for_stable composable param
// ============================================

func TestInteract_Navigate_WaitForStable_Composes(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"navigate","url":"https://example.com","wait_for_stable":true}`)
	if !ok {
		t.Fatal("navigate with wait_for_stable should return result")
	}
	if result.IsError {
		t.Fatalf("navigate with wait_for_stable should not error, got: %s", result.Content[0].Text)
	}

	// The navigate action should have created queries: navigation + wait_for_stable
	queries := env.capture.Queries().GetPendingQueries()
	if len(queries) < 2 {
		t.Fatalf("navigate with wait_for_stable should create at least 2 pending queries, got %d", len(queries))
	}

	// Find the wait_for_stable query
	foundStable := false
	for _, q := range queries {
		if q.Type == "dom_action" {
			var params map[string]any
			if err := json.Unmarshal(q.Params, &params); err != nil {
				continue
			}
			if params["action"] == "wait_for_stable" {
				foundStable = true
				break
			}
		}
	}
	if !foundStable {
		t.Error("navigate with wait_for_stable=true should queue a wait_for_stable dom_action")
	}
}

func TestInteract_Navigate_WaitForStable_False_NoStableQuery(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	_, ok := env.callInteract(t, `{"what":"navigate","url":"https://example.com","wait_for_stable":false}`)
	if !ok {
		t.Fatal("navigate should return result")
	}

	queries := env.capture.Queries().GetPendingQueries()
	for _, q := range queries {
		if q.Type == "dom_action" {
			var params map[string]any
			if err := json.Unmarshal(q.Params, &params); err != nil {
				continue
			}
			if params["action"] == "wait_for_stable" {
				t.Error("navigate with wait_for_stable=false should NOT queue wait_for_stable")
			}
		}
	}
}

// ============================================
// click + wait_for_stable composable param
// ============================================

func TestInteract_Click_WaitForStable_Composes(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#btn","wait_for_stable":true}`)
	if !ok {
		t.Fatal("click with wait_for_stable should return result")
	}
	if result.IsError {
		t.Fatalf("click with wait_for_stable should not error, got: %s", result.Content[0].Text)
	}

	// Find the wait_for_stable query
	queries := env.capture.Queries().GetPendingQueries()
	foundStable := false
	for _, q := range queries {
		if q.Type == "dom_action" {
			var params map[string]any
			if err := json.Unmarshal(q.Params, &params); err != nil {
				continue
			}
			if params["action"] == "wait_for_stable" {
				foundStable = true
				break
			}
		}
	}
	if !foundStable {
		t.Error("click with wait_for_stable=true should queue a wait_for_stable dom_action")
	}
}

// ============================================
// NoPanic coverage for new actions
// ============================================

func TestInteract_AutoDismiss_NoPanic(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("interact(auto_dismiss_overlays) PANICKED: %v", r)
		}
	}()

	args := json.RawMessage(`{"what":"auto_dismiss_overlays"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolInteract(req, args)
	if resp.Result == nil && resp.Error == nil {
		t.Error("interact(auto_dismiss_overlays) returned nil response")
	}
}

func TestInteract_WaitForStable_NoPanic(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("interact(wait_for_stable) PANICKED: %v", r)
		}
	}()

	args := json.RawMessage(`{"what":"wait_for_stable"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolInteract(req, args)
	if resp.Result == nil && resp.Error == nil {
		t.Error("interact(wait_for_stable) returned nil response")
	}
}
