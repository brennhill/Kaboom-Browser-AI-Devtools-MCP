// Purpose: Tests for canonical interact entrypoint parameter handling.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

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
