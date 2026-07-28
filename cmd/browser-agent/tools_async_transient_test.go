// tools_async_transient_test.go — Tests for attachTransientElements.
package main

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/asynccommand"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestAttachTransientElements_AttachesWhenPresent(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	h := asynccommand.New(asynccommand.Deps{Capture: cap})

	since := time.Now()
	sinceMs := since.UnixMilli()
	cap.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
		{Type: "click", Timestamp: sinceMs + 100, URL: "https://example.com"},
		{Type: "transient", Timestamp: sinceMs + 200, URL: "https://example.com", Classification: "toast", Value: "Saved", Role: "status"},
		{Type: "transient", Timestamp: sinceMs + 300, URL: "https://example.com", Classification: "alert", Value: "Error!", Role: "alert"},
	})

	responseData := map[string]any{}
	h.AttachTransientElements(responseData, since)

	transients, ok := responseData["transient_elements"].([]map[string]any)
	if !ok {
		t.Fatal("transient_elements not present or wrong type")
	}
	if len(transients) != 2 {
		t.Errorf("transient count = %d, want 2", len(transients))
	}
	// Verify url field is included in output
	for _, tr := range transients {
		if tr["url"] != "https://example.com" {
			t.Errorf("url = %q, want %q", tr["url"], "https://example.com")
		}
	}
}

func TestAttachTransientElements_OmitsWhenEmpty(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	h := asynccommand.New(asynccommand.Deps{Capture: cap})

	since := time.Now()
	sinceMs := since.UnixMilli()
	cap.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
		{Type: "click", Timestamp: sinceMs + 100, URL: "https://example.com"},
		{Type: "input", Timestamp: sinceMs + 200, URL: "https://example.com"},
	})

	responseData := map[string]any{}
	h.AttachTransientElements(responseData, since)

	if _, ok := responseData["transient_elements"]; ok {
		t.Error("transient_elements should not be present when no transients exist")
	}
}

func TestAttachTransientElements_CapsAtMax(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	h := asynccommand.New(asynccommand.Deps{Capture: cap})

	since := time.Now()
	sinceMs := since.UnixMilli()
	actions := make([]types.EnhancedAction, 15)
	for i := 0; i < 15; i++ {
		actions[i] = types.EnhancedAction{
			Type:           "transient",
			Timestamp:      sinceMs + int64(i*100),
			URL:            "https://example.com",
			Classification: "toast",
			Value:          "msg",
			Role:           "status",
		}
	}
	cap.Telemetry().AddEnhancedActionsForTest(actions)

	responseData := map[string]any{}
	h.AttachTransientElements(responseData, since)

	transients, ok := responseData["transient_elements"].([]map[string]any)
	if !ok {
		t.Fatal("transient_elements not present or wrong type")
	}
	if len(transients) != asynccommand.MaxTransientsPerResult {
		t.Errorf("transient count = %d, want %d (max cap)", len(transients), asynccommand.MaxTransientsPerResult)
	}
}

func TestAttachTransientElements_FiltersByTimestamp(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	h := asynccommand.New(asynccommand.Deps{Capture: cap})

	since := time.Now()
	sinceMs := since.UnixMilli()
	cap.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
		{Type: "transient", Timestamp: sinceMs - 1000, URL: "https://example.com", Classification: "toast", Value: "Old", Role: "status"},
		{Type: "transient", Timestamp: sinceMs + 200, URL: "https://example.com", Classification: "alert", Value: "New", Role: "alert"},
	})

	responseData := map[string]any{}
	h.AttachTransientElements(responseData, since)

	transients, ok := responseData["transient_elements"].([]map[string]any)
	if !ok {
		t.Fatal("transient_elements not present or wrong type")
	}
	if len(transients) != 1 {
		t.Errorf("transient count = %d, want 1 (only after since)", len(transients))
	}
	if transients[0]["value"] != "New" {
		t.Errorf("value = %q, want %q", transients[0]["value"], "New")
	}
}

func TestAttachTransientElements_ClockSkewTolerance(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	h := asynccommand.New(asynccommand.Deps{Capture: cap})

	since := time.Now()
	sinceMs := since.UnixMilli()
	// Actions must be in chronological order (append-only buffer invariant)
	cap.Telemetry().AddEnhancedActionsForTest([]types.EnhancedAction{
		// 1000ms before server time — outside 500ms tolerance, should be EXCLUDED
		{Type: "transient", Timestamp: sinceMs - 1000, URL: "https://example.com", Classification: "alert", Value: "TooOld", Role: "alert"},
		// 300ms before server time — within 500ms tolerance, should be INCLUDED
		{Type: "transient", Timestamp: sinceMs - 300, URL: "https://example.com", Classification: "toast", Value: "Skewed", Role: "status"},
	})

	responseData := map[string]any{}
	h.AttachTransientElements(responseData, since)

	transients, ok := responseData["transient_elements"].([]map[string]any)
	if !ok {
		t.Fatal("transient_elements not present or wrong type")
	}
	if len(transients) != 1 {
		t.Errorf("transient count = %d, want 1 (only within 500ms tolerance)", len(transients))
	}
	if transients[0]["value"] != "Skewed" {
		t.Errorf("value = %q, want %q", transients[0]["value"], "Skewed")
	}
}

func TestAttachTransientElements_NilSafety(t *testing.T) {
	t.Parallel()
	// Nil handler should not panic
	var h *asynccommand.Handler
	responseData := map[string]any{}
	h.AttachTransientElements(responseData, time.Now())
	// Should not have added anything
	if _, ok := responseData["transient_elements"]; ok {
		t.Error("should not add transient_elements with nil handler")
	}
}
