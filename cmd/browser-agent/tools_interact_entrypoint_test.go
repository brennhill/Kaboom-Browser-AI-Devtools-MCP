// Purpose: Tests for canonical interact entrypoint parameter handling.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestInteractAsyncActionsReturnCorrelationIDs(t *testing.T) {
	t.Parallel()

	handler, _, captured := makeToolHandler(t)
	capturefixture.SetPilot(captured, true)
	mockConnectedTrackedTab(t, captured)
	cases := []struct {
		name string
		args string
	}{
		{"highlight", `{"what":"highlight","selector":".test"}`},
		{"execute_js", `{"what":"execute_js","script":"1+1"}`},
		{"navigate", `{"what":"navigate","url":"https://example.test"}`},
		{"refresh", `{"what":"refresh"}`},
		{"back", `{"what":"back"}`},
		{"forward", `{"what":"forward"}`},
		{"new_tab", `{"what":"new_tab","url":"https://example.test"}`},
		{"list_interactive", `{"what":"list_interactive"}`},
		{"click", `{"what":"click","selector":".btn"}`},
		{"type", `{"what":"type","selector":"input","text":"hello"}`},
		{"get_text", `{"what":"get_text","selector":".el"}`},
		{"scroll_to", `{"what":"scroll_to","selector":".el"}`},
		{"focus", `{"what":"focus","selector":".el"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseToolResult(t, callInteractRaw(handler, tc.args))
			if result.IsError {
				t.Fatalf("action returned error: %s", result.Content[0].Text)
			}
			found := false
			for _, block := range result.Content {
				found = found || strings.Contains(block.Text, "correlation_id")
			}
			if !found {
				t.Errorf("response missing correlation_id: %.500s", result.Content[0].Text)
			}
		})
	}
}

func TestInteractPreDispatch_DoesNotRewriteAsyncToBackground(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{"what":"click","async":true}`)
	h, _, _ := makeToolHandler(t)
	result, blocked := interactRegistry.PreDispatch(h, mcp.JSONRPCRequest{ID: 1}, input, "click")
	if blocked != nil {
		t.Fatalf("pre-dispatch unexpectedly blocked: %+v", blocked)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if async, ok := parsed["async"]; !ok || async != true {
		t.Errorf("unrecognized async input should remain untouched, got %v", parsed)
	}
	if _, ok := parsed["background"]; ok {
		t.Errorf("async must not be rewritten to canonical background: %v", parsed)
	}
}

func TestInteractPreDispatch_PreservesCanonicalBackground(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{"what":"click","background":false}`)
	h, _, _ := makeToolHandler(t)
	result, blocked := interactRegistry.PreDispatch(h, mcp.JSONRPCRequest{ID: 2}, input, "click")
	if blocked != nil {
		t.Fatalf("pre-dispatch unexpectedly blocked: %+v", blocked)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if bg, ok := parsed["background"]; !ok {
		t.Error("background should be preserved")
	} else if bg != false {
		t.Errorf("background = %v, want false", bg)
	}
}
