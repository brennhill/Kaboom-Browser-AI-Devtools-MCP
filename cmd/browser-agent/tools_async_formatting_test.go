// tools_async_formatting_test.go — Tests for the response envelope built around
// the async result pipeline (trace summary slimming).

package main

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestAttachTraceSummary_OmitsEvents(t *testing.T) {
	t.Parallel()

	cmd := queries.CommandResult{
		TraceID:       "dom_click_123",
		TraceTimeline: "queued -> sent -> started -> resolved",
		TraceEvents: []queries.CommandTraceEvent{
			{Stage: "queued"},
			{Stage: "sent"},
			{Stage: "started"},
			{Stage: "resolved"},
		},
		QueryID: "q-22",
	}

	responseData := map[string]any{}
	attachTraceSummary(responseData, cmd)

	trace, ok := responseData["trace"].(map[string]any)
	if !ok {
		t.Fatal("trace should be present")
	}

	if trace["trace_id"] != "dom_click_123" {
		t.Errorf("expected trace_id dom_click_123, got %v", trace["trace_id"])
	}
	if trace["timeline"] != "queued -> sent -> started -> resolved" {
		t.Errorf("unexpected timeline: %v", trace["timeline"])
	}
	if trace["query_id"] != "q-22" {
		t.Errorf("expected query_id q-22, got %v", trace["query_id"])
	}
	if trace["last_stage"] != "resolved" {
		t.Errorf("expected last_stage resolved, got %v", trace["last_stage"])
	}

	// events must NOT be present (token savings)
	if _, ok := trace["events"]; ok {
		t.Error("trace.events should be omitted for token efficiency")
	}
}

// ============================================
// stripSuccessOnlyFields tests
// ============================================
