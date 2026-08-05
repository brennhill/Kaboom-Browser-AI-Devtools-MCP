// tools_configure_runtime_actions_test.go — Tests health, telemetry, and buffer actions.
// Docs: docs/features/feature/buffer-clearing/index.md

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

func TestToolsConfigureHealth_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"health"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("health should succeed, got: %s", result.Content[0].Text)
	}

	// Health response should have content
	if len(result.Content) == 0 {
		t.Fatal("health should return content block")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content type = %q, want 'text'", result.Content[0].Type)
	}

	// Health response text should contain status info
	text := result.Content[0].Text
	if !strings.Contains(strings.ToLower(text), "status") &&
		!strings.Contains(strings.ToLower(text), "health") {
		t.Errorf("health response should mention status/health, got: %s", text)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureTelemetry_DefaultStatus(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"telemetry"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("telemetry status should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if mode, _ := data["telemetry_mode"].(string); mode != telemetryModeAuto {
		t.Errorf("telemetry_mode = %q, want %q", mode, telemetryModeAuto)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureTelemetry_SetMode(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"telemetry","telemetry_mode":"full"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("telemetry set mode should succeed, got: %s", result.Content[0].Text)
	}
	data := extractResultJSON(t, result)
	if mode, _ := data["telemetry_mode"].(string); mode != telemetryModeFull {
		t.Errorf("telemetry_mode = %q, want %q", mode, telemetryModeFull)
	}
}

func TestToolsConfigureTelemetry_InvalidMode(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"telemetry","telemetry_mode":"verbose"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid telemetry mode should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_param") {
		t.Errorf("error code should be 'invalid_param', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "telemetry_mode") {
		t.Errorf("error should mention telemetry_mode, got: %s", result.Content[0].Text)
	}
}

// ============================================
// configure(action:"clear") — Response Fields & State Changes
// ============================================

func TestToolsConfigureClear_AllBuffers_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"clear","buffer":"all"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("clear all should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if data["buffer"] != "all" {
		t.Errorf("buffer = %v, want 'all'", data["buffer"])
	}
	if _, ok := data["cleared"]; !ok {
		t.Error("response missing 'cleared' field")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureClear_DefaultsToAll(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"clear"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("clear default should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["buffer"] != "all" {
		t.Errorf("default buffer = %v, want 'all'", data["buffer"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureClear_SpecificBuffers(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	buffers := []string{"network", "websocket", "actions", "logs", "inbox"}
	for _, buffer := range buffers {
		t.Run(buffer, func(t *testing.T) {
			resp := callConfigureRaw(h, `{"what":"clear","buffer":"`+buffer+`"}`)
			result := parseToolResult(t, resp)
			if result.IsError {
				t.Fatalf("clear %s should succeed, got: %s", buffer, result.Content[0].Text)
			}

			data := extractResultJSON(t, result)
			if data["status"] != "ok" {
				t.Errorf("status = %v, want 'ok'", data["status"])
			}
			if data["buffer"] != buffer {
				t.Errorf("buffer = %v, want %q", data["buffer"], buffer)
			}

			assertSnakeCaseFields(t, string(resp.Result))
		})
	}
}

func TestToolsConfigureClear_AllDrainsInbox(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// Enqueue a push event into the inbox.
	h.server.pushInbox.Enqueue(push.PushEvent{ID: "ss-1", Type: "screenshot", PageURL: "https://example.com"})
	if h.server.pushInbox.Len() != 1 {
		t.Fatal("precondition: inbox should have 1 event")
	}

	resp := callConfigureRaw(h, `{"what":"clear","buffer":"all"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("clear all should succeed, got: %s", result.Content[0].Text)
	}

	if h.server.pushInbox.Len() != 0 {
		t.Fatalf("inbox should be empty after clear all, got %d", h.server.pushInbox.Len())
	}

	data := extractResultJSON(t, result)
	cleared, _ := data["cleared"].(map[string]any)
	if cleared == nil {
		t.Fatal("cleared should be a map")
	}
	if _, ok := cleared["push_events_drained"]; !ok {
		t.Error("cleared should report push_events_drained count")
	}
}

func TestToolsConfigureClear_InboxBuffer(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	h.server.pushInbox.Enqueue(push.PushEvent{ID: "ss-1", Type: "screenshot"})
	h.server.pushInbox.Enqueue(push.PushEvent{ID: "ss-2", Type: "chat"})

	resp := callConfigureRaw(h, `{"what":"clear","buffer":"inbox"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("clear inbox should succeed, got: %s", result.Content[0].Text)
	}

	if h.server.pushInbox.Len() != 0 {
		t.Fatalf("inbox should be empty, got %d", h.server.pushInbox.Len())
	}

	data := extractResultJSON(t, result)
	if data["buffer"] != "inbox" {
		t.Errorf("buffer = %v, want 'inbox'", data["buffer"])
	}
	cleared, _ := data["cleared"].(map[string]any)
	if cleared == nil {
		t.Fatal("cleared should be a map")
	}
	count, _ := cleared["push_events"].(float64) // JSON numbers are float64
	if count != 2 {
		t.Errorf("push_events = %v, want 2", count)
	}
}

func TestToolsConfigureClear_UnknownBuffer(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"clear","buffer":"invalid_buf"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("clear invalid buffer should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_buf") {
		t.Error("error should mention the invalid buffer name")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsConfigureClear_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := toolconfigure.HandleClear(toolconfigure.ClearTargets{
		Capture:  h.capture,
		Resetter: newRuntimeResetter(h.capture),
		ClearLogs: func() int {
			count := h.server.logs.EntryCount()
			h.server.logs.ClearEntries()
			return count
		},
		Inbox:       h.server.pushInbox,
		Annotations: h.annotationStore,
	}, req, json.RawMessage(`{bad}`))
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return isError:true")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}
