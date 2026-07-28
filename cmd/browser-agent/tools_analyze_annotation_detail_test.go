// Purpose: Tests for analyze annotation processing.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_annotation_detail_test.go — Annotation detail enrichment and correlation tests.
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestToolGetAnnotationDetail_WithA11yFlags(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID: "detail_a11y",
		Selector:      "div.clickable",
		Tag:           "div",
		A11yFlags:     []string{"interactive_without_role", "low_contrast:2.1:1", "small_touch_target:32x28"},
	}
	h.annotationStore.StoreDetail("detail_a11y", detail)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_a11y"}`)

	resp := h.annotationAnalysis.GetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	flags, ok := data["a11y_flags"].([]any)
	if !ok {
		t.Fatal("expected a11y_flags array")
	}
	if len(flags) != 3 {
		t.Errorf("expected 3 a11y flags, got %d", len(flags))
	}
	if flags[0] != "interactive_without_role" {
		t.Errorf("expected first flag, got %v", flags[0])
	}
}

func TestToolGetAnnotationDetail_NoA11yFlags(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID: "detail_clean",
		Selector:      "button.primary",
		Tag:           "button",
		A11yFlags:     nil,
	}
	h.annotationStore.StoreDetail("detail_clean", detail)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_clean"}`)

	resp := h.annotationAnalysis.GetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// a11y_flags should be absent when empty
	if _, exists := data["a11y_flags"]; exists {
		t.Error("expected a11y_flags to be absent when empty")
	}
}

func TestToolGetAnnotationDetail_MissingCorrelationID(t *testing.T) {
	h := createTestToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail"}`)

	resp := h.annotationAnalysis.GetAnnotationDetail(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "correlation_id") {
		t.Errorf("expected missing param error, got %q", text)
	}
}

func TestToolGetAnnotationDetail_NewEnrichmentFields(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID:      "detail_enriched",
		Selector:           "button#submit-btn",
		Tag:                "button",
		TextContent:        "Submit",
		Classes:            []string{"btn-primary"},
		ComputedStyles:     map[string]string{"color": "rgb(255,255,255)"},
		ParentContext:      json.RawMessage(`{"parent":{"tag":"div","classes":["actions"],"id":"","role":"group"},"grandparent":{"tag":"form","classes":["checkout"],"id":"checkout","role":""}}`),
		Siblings:           json.RawMessage(`[{"tag":"button","classes":["btn-secondary"],"text":"Cancel","position":"before"},{"tag":"a","classes":["help-link"],"text":"Help","position":"after"}]`),
		CSSFramework:       "tailwind",
		JSFramework:        "react",
		SelectorCandidates: []string{"testid=checkout-submit", "role=button|Submit", "css=button#submit-btn"},
		Component:          json.RawMessage(`{"framework":"react","component":"SubmitButton","source_file":"/src/components/SubmitButton.tsx","source_line":42}`),
	}
	h.annotationStore.StoreDetail("detail_enriched", detail)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_enriched"}`)

	resp := h.annotationAnalysis.GetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v\nraw text: %s", err, text)
	}

	// Verify parent_context present
	pc, ok := data["parent_context"].(map[string]any)
	if !ok {
		t.Fatal("expected parent_context to be an object")
	}
	parent, ok := pc["parent"].(map[string]any)
	if !ok {
		t.Fatal("expected parent_context.parent to be an object")
	}
	if parent["tag"] != "div" {
		t.Errorf("expected parent tag 'div', got %v", parent["tag"])
	}

	// Verify siblings present
	sibs, ok := data["siblings"].([]any)
	if !ok {
		t.Fatal("expected siblings to be an array")
	}
	if len(sibs) != 2 {
		t.Fatalf("expected 2 siblings, got %d", len(sibs))
	}

	// Verify css_framework present
	if data["css_framework"] != "tailwind" {
		t.Errorf("expected css_framework 'tailwind', got %v", data["css_framework"])
	}
	if data["js_framework"] != "react" {
		t.Errorf("expected js_framework 'react', got %v", data["js_framework"])
	}
	candidates, ok := data["selector_candidates"].([]any)
	if !ok || len(candidates) != 3 {
		t.Fatalf("expected selector_candidates array with 3 entries, got %v", data["selector_candidates"])
	}
	component, ok := data["component"].(map[string]any)
	if !ok {
		t.Fatalf("expected component object, got %T", data["component"])
	}
	if component["framework"] != "react" {
		t.Errorf("expected component.framework='react', got %v", component["framework"])
	}
}

func TestToolGetAnnotationDetail_ErrorCorrelation(t *testing.T) {
	h := createTestToolHandler(t)

	// Store annotation with known timestamp
	annotTS := time.Now()
	session := &annotation.Session{
		Annotations: []annotation.Annotation{
			{
				ID:            "ann_corr",
				Text:          "broken button",
				CorrelationID: "detail_corr",
				Timestamp:     annotTS.UnixMilli(),
			},
		},
		TabID:     1,
		Timestamp: annotTS.UnixMilli(),
		PageURL:   "https://example.com",
	}
	h.annotationStore.StoreSession(1, session)

	// Store the detail
	detail := annotation.Detail{
		CorrelationID:  "detail_corr",
		Selector:       "button.broken",
		Tag:            "button",
		Classes:        []string{"broken"},
		ComputedStyles: map[string]string{},
	}
	h.annotationStore.StoreDetail("detail_corr", detail)

	// Inject log entries: errors near the annotation timestamp
	h.server.logs.SeedEntries([]types.LogEntry{
		types.LogEntry{"level": "error", "message": "TypeError: Cannot read property 'click'", "ts": annotTS.Add(-2 * time.Second).UTC().Format(time.RFC3339)},
		types.LogEntry{"level": "error", "message": "Uncaught ReferenceError: x is not defined", "ts": annotTS.Add(3 * time.Second).UTC().Format(time.RFC3339)},
		types.LogEntry{"level": "info", "message": "page loaded", "ts": annotTS.Add(-1 * time.Second).UTC().Format(time.RFC3339)},
		// not error
		types.LogEntry{"level": "error", "message": "far away error", "ts": annotTS.Add(-30 * time.Second).UTC().Format(time.RFC3339)},
		// outside window,
	}, []time.Time{
		annotTS.Add(-2 * time.Second),
		annotTS.Add(3 * time.Second),
		annotTS.Add(-1 * time.Second),
		annotTS.Add(-30 * time.Second),
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_corr"}`)

	resp := h.annotationAnalysis.GetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v\nraw: %s", err, text)
	}

	errors, ok := data["correlated_errors"].([]any)
	if !ok {
		t.Fatal("expected correlated_errors array")
	}
	if len(errors) != 2 {
		t.Fatalf("expected 2 correlated errors (within ±5s), got %d", len(errors))
	}

	// Verify the window seconds is present
	if data["error_correlation_window_seconds"] != float64(5) {
		t.Errorf("expected error_correlation_window_seconds=5, got %v", data["error_correlation_window_seconds"])
	}
}

func TestToolGetAnnotationDetail_ErrorCorrelation_NoErrors(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID:  "detail_no_err",
		Selector:       "div.clean",
		Tag:            "div",
		Classes:        []string{},
		ComputedStyles: map[string]string{},
	}
	h.annotationStore.StoreDetail("detail_no_err", detail)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_no_err"}`)

	resp := h.annotationAnalysis.GetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Should not be present when no errors match
	if _, exists := data["correlated_errors"]; exists {
		t.Error("expected correlated_errors to be absent when no matching errors")
	}
	if _, exists := data["error_correlation_window_seconds"]; exists {
		t.Error("expected error_correlation_window_seconds to be absent when no matching errors")
	}
}

func TestToolGetAnnotationDetail_ErrorCorrelation_NamedSession(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	annotTS := time.Now()

	// Store annotation in a NAMED session (not anonymous)
	h.annotationStore.AppendToNamedSession("pm-review", &annotation.Session{
		TabID:     1,
		Timestamp: annotTS.UnixMilli(),
		PageURL:   "https://example.com",
		Annotations: []annotation.Annotation{
			{ID: "ann_ns", Text: "broken layout", CorrelationID: "detail_ns", Timestamp: annotTS.UnixMilli()},
		},
	})

	detail := annotation.Detail{
		CorrelationID:  "detail_ns",
		Selector:       "div.layout",
		Tag:            "div",
		Classes:        []string{},
		ComputedStyles: map[string]string{},
	}
	h.annotationStore.StoreDetail("detail_ns", detail)

	// Inject error near annotation time
	h.server.logs.SeedEntries([]types.LogEntry{
		types.LogEntry{"level": "error", "message": "Layout shift error", "ts": annotTS.Add(-1 * time.Second).UTC().Format(time.RFC3339)},
	}, []time.Time{
		annotTS.Add(-1 * time.Second),
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotationDetail(req, json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_ns"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	errors, ok := data["correlated_errors"].([]any)
	if !ok || len(errors) == 0 {
		t.Fatal("expected correlated_errors for annotation in named session")
	}
}

func TestToolGetAnnotationDetail_ErrorCorrelation_NonLatestTab(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	annotTS := time.Now()

	// Store annotation on tab 1
	h.annotationStore.StoreSession(1, &annotation.Session{
		TabID:     1,
		Timestamp: annotTS.UnixMilli(),
		PageURL:   "https://example.com/page1",
		Annotations: []annotation.Annotation{
			{ID: "ann_t1", Text: "old tab issue", CorrelationID: "detail_t1", Timestamp: annotTS.UnixMilli()},
		},
	})

	// Store a NEWER session on tab 2 (this becomes the "latest")
	h.annotationStore.StoreSession(2, &annotation.Session{
		TabID:     2,
		Timestamp: annotTS.Add(1 * time.Second).UnixMilli(),
		PageURL:   "https://example.com/page2",
		Annotations: []annotation.Annotation{
			{ID: "ann_t2", Text: "newer tab", CorrelationID: "detail_t2", Timestamp: annotTS.Add(1 * time.Second).UnixMilli()},
		},
	})

	detail := annotation.Detail{
		CorrelationID:  "detail_t1",
		Selector:       "div.old",
		Tag:            "div",
		Classes:        []string{},
		ComputedStyles: map[string]string{},
	}
	h.annotationStore.StoreDetail("detail_t1", detail)

	// Inject error near tab 1's annotation time
	h.server.logs.SeedEntries([]types.LogEntry{
		types.LogEntry{"level": "error", "message": "Tab1 error", "ts": annotTS.Add(-2 * time.Second).UTC().Format(time.RFC3339)},
	}, []time.Time{
		annotTS.Add(-2 * time.Second),
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotationDetail(req, json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_t1"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	errors, ok := data["correlated_errors"].([]any)
	if !ok || len(errors) == 0 {
		t.Fatal("expected correlated_errors for annotation in non-latest tab session")
	}
}

func TestToolGetAnnotationDetail_NewFieldsAbsentWhenEmpty(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID:  "detail_minimal",
		Selector:       "div.plain",
		Tag:            "div",
		Classes:        []string{},
		ComputedStyles: map[string]string{},
	}
	h.annotationStore.StoreDetail("detail_minimal", detail)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_minimal"}`)

	resp := h.annotationAnalysis.GetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// These fields should be absent when empty
	if _, exists := data["parent_context"]; exists {
		t.Error("expected parent_context to be absent when empty")
	}
	if _, exists := data["siblings"]; exists {
		t.Error("expected siblings to be absent when empty")
	}
	if _, exists := data["css_framework"]; exists {
		t.Error("expected css_framework to be absent when empty")
	}
	if _, exists := data["js_framework"]; exists {
		t.Error("expected js_framework to be absent when empty")
	}
	if _, exists := data["selector_candidates"]; exists {
		t.Error("expected selector_candidates to be absent when empty")
	}
	if _, exists := data["component"]; exists {
		t.Error("expected component to be absent when empty")
	}
}

// --- LLM Hints tests ---
