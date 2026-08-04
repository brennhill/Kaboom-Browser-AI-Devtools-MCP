// Purpose: Coverage-expansion tests for browser-agent edge cases and branch paths.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_coverage_test.go — Coverage tests for interact sub-handlers.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// handleSubtitle — 0% → 100%
// ============================================

func TestHandleSubtitle_SetText(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"subtitle","text":"Hello world"}`)
	if !ok {
		t.Fatal("subtitle should return result")
	}
	if result.IsError {
		t.Fatalf("subtitle should not error, got: %s", result.Content[0].Text)
	}

	// Verify pending query was created
	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("subtitle should create a pending query")
	}
	if pq.Type != "subtitle" {
		t.Fatalf("pending query type = %q, want subtitle", pq.Type)
	}

	// Verify response mentions subtitle
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "subtitle") {
		t.Errorf("response should mention subtitle, got: %s", text)
	}
}

func TestHandleSubtitle_ClearText(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"subtitle","text":""}`)
	if !ok {
		t.Fatal("subtitle clear should return result")
	}
	if result.IsError {
		t.Fatalf("subtitle clear should not error, got: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "clear") {
		t.Errorf("response should mention clear, got: %s", text)
	}
}

func TestHandleSubtitle_MissingText(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"subtitle"}`)
	if !ok {
		t.Fatal("subtitle missing text should return result")
	}
	if !result.IsError {
		t.Fatal("subtitle without text should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "text") {
		t.Errorf("error should mention 'text' param, got: %s", text)
	}
}

func TestHandleSubtitle_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	args := json.RawMessage(`{invalid}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.browserActions.Handle("subtitle", req, args)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

// ============================================
// handleListInteractive — 40% → 80%+
// ============================================

func TestHandleListInteractive_PilotEnabled(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.Extension().SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"list_interactive"}`)
	if !ok {
		t.Fatal("list_interactive should return result")
	}
	if result.IsError {
		t.Fatalf("list_interactive with pilot enabled should not error, got: %s", result.Content[0].Text)
	}

	// Verify pending query was created
	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("list_interactive should create a pending query")
	}
	if pq.Type != "dom_action" {
		t.Fatalf("pending query type = %q, want dom_action", pq.Type)
	}
	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}
	if got, _ := params["action"].(string); got != "list_interactive" {
		t.Fatalf("pending query action = %#v, want list_interactive", params["action"])
	}

	// Verify response mentions queued
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "queued") {
		t.Errorf("response should mention queued, got: %s", text)
	}
}

func TestHandleListInteractive_PilotDisabled(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	// Pilot disabled by default

	result, ok := env.callInteract(t, `{"what":"list_interactive"}`)
	if !ok {
		t.Fatal("list_interactive should return result")
	}
	if !result.IsError {
		t.Fatal("list_interactive with pilot disabled should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "pilot") {
		t.Errorf("error should mention pilot, got: %s", text)
	}
}

func TestHandleListInteractive_WithTabID(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.Extension().SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"list_interactive","tab_id":42}`)
	if !ok {
		t.Fatal("list_interactive with tab_id should return result")
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("should create pending query")
	}
	if pq.TabID != 42 {
		t.Fatalf("pending query TabID = %d, want 42", pq.TabID)
	}
}

func TestHandleListInteractive_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	args := json.RawMessage(`{bad json}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.domActions.HandleListInteractive(req, args)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}

// ============================================
// handleHighlight — 50% → 80%+
// ============================================

func TestHandlePilotHighlight_Success(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.Extension().SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"highlight","selector":"#main"}`)
	if !ok {
		t.Fatal("highlight should return result")
	}
	if result.IsError {
		t.Fatalf("highlight with pilot enabled should not error, got: %s", result.Content[0].Text)
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("highlight should create a pending query")
	}
	if pq.Type != "highlight" {
		t.Fatalf("pending query type = %q, want highlight", pq.Type)
	}
}

func TestHandlePilotHighlight_MissingSelector(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"highlight"}`)
	if !ok {
		t.Fatal("highlight without selector should return result")
	}
	if !result.IsError {
		t.Fatal("highlight without selector should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "selector") {
		t.Errorf("error should mention selector, got: %s", text)
	}
}

func TestHandlePilotHighlight_PilotDisabledWithSelector(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	// Pilot disabled by default

	result, ok := env.callInteract(t, `{"what":"highlight","selector":"div"}`)
	if !ok {
		t.Fatal("highlight should return result")
	}
	if !result.IsError {
		t.Fatal("highlight with pilot disabled should return error")
	}
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "pilot") {
		t.Errorf("error should mention pilot, got: %s", text)
	}
}

func TestHandlePilotHighlight_WithTabID(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.Extension().SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"highlight","selector":".btn","tab_id":99}`)
	if !ok {
		t.Fatal("highlight with tab_id should return result")
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].Text)
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("should create pending query")
	}
	if pq.TabID != 99 {
		t.Fatalf("TabID = %d, want 99", pq.TabID)
	}
}

func TestHandlePilotHighlight_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)

	args := json.RawMessage(`{bad}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.browserActions.Handle("highlight", req, args)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}
