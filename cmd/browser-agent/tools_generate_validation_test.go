// Purpose: Tests for generate tool input validation.
// Docs: docs/features/feature/test-generation/index.md

// tools_generate_validation_test.go — Tests for generate API strict parameter validation (#57).
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestGenerateRejectsUnknownParams(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   map[string]any
	}{
		{
			name:   "reproduction rejects bogus param",
			format: "reproduction",
			args:   map[string]any{"what": "reproduction", "bogus": true},
		},
		{
			name:   "test rejects unknown scope param",
			format: "test",
			args:   map[string]any{"what": "test", "scope": "page"},
		},
		{
			name:   "har rejects include_passes",
			format: "har",
			args:   map[string]any{"what": "har", "include_passes": true},
		},
		{
			name:   "csp rejects resource_types",
			format: "csp",
			args:   map[string]any{"what": "csp", "resource_types": []string{"script"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsJSON, _ := json.Marshal(tt.args)
			resp := toolgenerate.ValidateGenerateParams(
				mcp.JSONRPCRequest{ID: 1},
				tt.format,
				argsJSON,
			)
			if resp == nil {
				t.Fatal("expected error response for unknown params, got nil")
			}
			var result mcp.MCPToolResult
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}
			if !result.IsError {
				t.Error("expected isError=true")
			}
			if !strings.Contains(result.Content[0].Text, "invalid_param") {
				t.Errorf("expected invalid_param error code, got: %s", result.Content[0].Text)
			}
		})
	}
}

func TestGenerateAcceptsValidParams(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   map[string]any
	}{
		{
			name:   "reproduction with valid params",
			format: "reproduction",
			args:   map[string]any{"what": "reproduction", "error_message": "404", "last_n": 5},
		},
		{
			name:   "test with telemetry_mode",
			format: "test",
			args:   map[string]any{"what": "test", "test_name": "login", "telemetry_mode": "auto"},
		},
		{
			name:   "har with filters",
			format: "har",
			args:   map[string]any{"what": "har", "url": "/api", "method": "GET"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsJSON, _ := json.Marshal(tt.args)
			resp := toolgenerate.ValidateGenerateParams(
				mcp.JSONRPCRequest{ID: 1},
				tt.format,
				argsJSON,
			)
			if resp != nil {
				t.Fatalf("expected nil (no error) for valid params, got: %+v", resp)
			}
		})
	}
}

func TestGenerateEmptyOutputIncludesReason(t *testing.T) {
	h := newTestToolHandler()

	// Call generate(test) with no actions captured → should include reason
	args, _ := json.Marshal(map[string]any{"what": "test"})
	resp := h.generateDispatcher.Handle(mcp.JSONRPCRequest{ID: 1}, args)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, `"reason"`) {
		t.Error("expected reason field in empty output")
	}
	if !strings.Contains(text, "no_actions_captured") {
		t.Error("expected no_actions_captured reason")
	}
}

func TestGeneratePRSummaryEmptyIncludesReason(t *testing.T) {
	h := newTestToolHandler()

	args, _ := json.Marshal(map[string]any{"what": "pr_summary"})
	resp := h.generateDispatcher.Handle(mcp.JSONRPCRequest{ID: 1}, args)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, `"reason"`) {
		t.Error("expected reason field in empty pr_summary output")
	}
	if !strings.Contains(text, "no_activity_captured") {
		t.Error("expected no_activity_captured reason")
	}
}
