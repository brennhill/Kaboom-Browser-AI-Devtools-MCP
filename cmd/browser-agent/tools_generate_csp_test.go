// Purpose: Tests for generate CSP policy output.
// Docs: docs/features/feature/test-generation/index.md

// tools_generate_csp_test.go — Coverage tests for toolGenerateCSP handler.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// toolGenerateCSP — 29% → 100%
// ============================================

func TestToolGenerateCSP_NoNetworkBodies(t *testing.T) {
	t.Parallel()
	env := newObserveTestEnv(t)

	args := json.RawMessage(`{"mode":"strict"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolGenerateCSP(req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("CSP with no network bodies should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "unavailable" {
		t.Fatalf("status = %q, want unavailable", status)
	}
	if mode, _ := data["mode"].(string); mode != "strict" {
		t.Fatalf("mode = %q, want strict", mode)
	}
}

func TestToolGenerateCSP_DefaultMode(t *testing.T) {
	t.Parallel()
	env := newObserveTestEnv(t)

	args := json.RawMessage(`{}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolGenerateCSP(req, args)

	result := parseToolResult(t, resp)

	data := parseResponseJSON(t, result)
	if mode, _ := data["mode"].(string); mode != "moderate" {
		t.Fatalf("mode = %q, want moderate (default)", mode)
	}
}

func TestToolGenerateCSP_WithNetworkBodies(t *testing.T) {
	t.Parallel()
	env := newObserveTestEnv(t)

	env.capture.AddNetworkBodiesForTest([]capture.NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", Method: "GET", Status: 200},
		{URL: "https://cdn.example.com/style.css", ContentType: "text/css", Method: "GET", Status: 200},
		{URL: "https://api.example.com/data", ContentType: "application/json", Method: "GET", Status: 200},
		{URL: "https://cdn.example.com/logo.png", ContentType: "image/png", Method: "GET", Status: 200},
	})

	args := json.RawMessage(`{"mode":"strict"}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolGenerateCSP(req, args)

	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("CSP should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if status, _ := data["status"].(string); status != "ok" {
		t.Fatalf("status = %q, want ok", status)
	}

	policy, _ := data["policy"].(string)
	if !strings.Contains(policy, "default-src") {
		t.Errorf("policy should contain default-src, got: %s", policy)
	}
	if !strings.Contains(policy, "'self'") {
		t.Errorf("policy should contain 'self', got: %s", policy)
	}

	originsObserved, _ := data["origins_observed"].(float64)
	if originsObserved != 4 {
		t.Fatalf("origins_observed = %v, want 4", originsObserved)
	}

	directives, _ := data["directives"].(map[string]any)
	if directives == nil {
		t.Fatal("response should contain directives map")
	}
}

func TestToolGenerateCSP_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := newObserveTestEnv(t)

	args := json.RawMessage(`{bad json}`)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := env.handler.toolGenerateCSP(req, args)

	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return error")
	}
}
