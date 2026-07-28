// Purpose: Tests for interact draw-mode annotation.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_draw_test.go — Tests for draw_mode_start interact handler.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestHandleDrawModeStart_PilotDisabled(t *testing.T) {
	h := createTestToolHandler(t)

	// Pilot is disabled by default in test handler
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{}`)

	resp := h.interactActionHandler.HandleDrawModeStart(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "disabled") || !strings.Contains(text, "Pilot") {
		t.Errorf("expected pilot disabled error mentioning both 'disabled' and 'Pilot', got %q", text)
	}
}

func TestHandleDrawModeStart_Success(t *testing.T) {
	h := createTestToolHandler(t)

	// Enable pilot
	h.capture.Extension().SetPilotEnabled(true)
	mockConnectedTrackedTab(t, h.capture)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{}`)

	resp := h.interactActionHandler.HandleDrawModeStart(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "queued") || !strings.Contains(text, "correlation_id") {
		t.Errorf("expected queued response with both 'queued' and 'correlation_id', got %q", text)
	}
}

func TestHandleDrawModeStart_WithSession(t *testing.T) {
	h := createTestToolHandler(t)
	h.capture.Extension().SetPilotEnabled(true)
	mockConnectedTrackedTab(t, h.capture)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"annot_session":"my-review"}`)

	resp := h.interactActionHandler.HandleDrawModeStart(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "queued") {
		t.Errorf("expected queued response, got %q", text)
	}
	if !strings.Contains(text, "correlation_id") {
		t.Errorf("expected correlation_id in response, got %q", text)
	}
}

func TestGetAnnotationDetail_MalformedJSON(t *testing.T) {
	h := createTestToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{not valid json`)

	resp := h.annotationAnalysis.GetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "Invalid JSON") {
		t.Errorf("expected Invalid JSON error, got %q", text)
	}
}
