// asyncresult_test.go — Tests for async command result enrichment,
// normalization and the agent-facing lifecycle_status vocabulary.

package asyncresult

import (
	"encoding/json"
	"testing"
)

func TestEnrichCommandResponseData_SameURLOmitsRedundantFields(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": true,
		"resolved_tab_id": 42,
		"resolved_url": "https://example.com/page",
		"effective_tab_id": 42,
		"effective_url": "https://example.com/page",
		"effective_title": "Example Page",
		"final_url": "https://example.com/page",
		"title": "Example Page",
		"timing": {"duration_ms": 50}
	}`)

	responseData := map[string]any{}
	embeddedErr, hasErr := EnrichCommandResponseData(result, responseData)
	if hasErr {
		t.Fatalf("unexpected embedded error: %s", embeddedErr)
	}

	// When URLs match, resolved_* and final_url/title should be omitted
	if _, ok := responseData["resolved_tab_id"]; ok {
		t.Error("resolved_tab_id should be omitted when URLs match")
	}
	if _, ok := responseData["resolved_url"]; ok {
		t.Error("resolved_url should be omitted when URLs match")
	}
	if _, ok := responseData["final_url"]; ok {
		t.Error("final_url should be omitted when same as effective_url")
	}
	if _, ok := responseData["title"]; ok {
		t.Error("title should be omitted when same as effective_title")
	}

	// effective_* should still be present
	if responseData["effective_url"] != "https://example.com/page" {
		t.Errorf("effective_url should be present, got %v", responseData["effective_url"])
	}
	if responseData["effective_title"] != "Example Page" {
		t.Errorf("effective_title should be present, got %v", responseData["effective_title"])
	}

	// tab_changed should not be present
	if _, ok := responseData["tab_changed"]; ok {
		t.Error("tab_changed should not be present when URLs match")
	}
	// navigation_detected should not be present
	if _, ok := responseData["navigation_detected"]; ok {
		t.Error("navigation_detected should not be present when URLs match")
	}
}

func TestEnrichCommandResponseData_DifferentURLIncludesNavFields(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": true,
		"resolved_tab_id": 42,
		"resolved_url": "https://example.com/old",
		"effective_tab_id": 42,
		"effective_url": "https://example.com/new",
		"effective_title": "New Page",
		"final_url": "https://example.com/new",
		"title": "New Page",
		"timing": {"duration_ms": 50}
	}`)

	responseData := map[string]any{}
	embeddedErr, hasErr := EnrichCommandResponseData(result, responseData)
	if hasErr {
		t.Fatalf("unexpected embedded error: %s", embeddedErr)
	}

	// When URLs differ, tab_changed and navigation_detected should be true
	if responseData["tab_changed"] != true {
		t.Error("tab_changed should be true when URLs differ")
	}
	if responseData["navigation_detected"] != true {
		t.Error("navigation_detected should be true when URLs differ")
	}

	// navigation_note should explain the change
	note, ok := responseData["navigation_note"].(string)
	if !ok || note == "" {
		t.Error("navigation_note should be present when URLs differ")
	}

	// All URL fields should be present when they differ
	if responseData["resolved_url"] != "https://example.com/old" {
		t.Errorf("resolved_url should be present when URLs differ, got %v", responseData["resolved_url"])
	}
	if responseData["effective_url"] != "https://example.com/new" {
		t.Errorf("effective_url should be present, got %v", responseData["effective_url"])
	}
}

func TestEnrichCommandResponseData_NoTabFields(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": true,
		"timing": {"duration_ms": 50}
	}`)

	responseData := map[string]any{}
	embeddedErr, hasErr := EnrichCommandResponseData(result, responseData)
	if hasErr {
		t.Fatalf("unexpected embedded error: %s", embeddedErr)
	}

	// No tab-related fields should be present
	if _, ok := responseData["tab_changed"]; ok {
		t.Error("tab_changed should not be present when no tab fields")
	}
	if _, ok := responseData["navigation_detected"]; ok {
		t.Error("navigation_detected should not be present when no tab fields")
	}

	// timing should still come through
	if responseData["timing"] == nil {
		t.Error("timing should still be surfaced")
	}
}

func TestEnrichCommandResponseData_ErrorStillSurfaced(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": false,
		"error": "element not found",
		"resolved_url": "https://example.com",
		"effective_url": "https://example.com"
	}`)

	responseData := map[string]any{}
	embeddedErr, hasErr := EnrichCommandResponseData(result, responseData)
	if !hasErr {
		t.Fatal("expected embedded error")
	}
	if embeddedErr != "element not found" {
		t.Errorf("expected 'element not found', got %q", embeddedErr)
	}
}

func TestEnrichCommandResponseData_ReturnValueSurfaced(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": true,
		"result": {"links": ["https://a.com", "https://b.com"]},
		"effective_url": "https://example.com"
	}`)

	responseData := map[string]any{}
	embeddedErr, hasErr := EnrichCommandResponseData(result, responseData, "exec_test_123")
	if hasErr {
		t.Fatalf("unexpected embedded error: %s", embeddedErr)
	}

	// return_value should be surfaced at top level
	rv, ok := responseData["return_value"]
	if !ok {
		t.Fatal("return_value should be surfaced at top level for execute_js results")
	}
	rvMap, ok := rv.(map[string]any)
	if !ok {
		t.Fatalf("return_value should be a map, got %T", rv)
	}
	links, ok := rvMap["links"].([]any)
	if !ok || len(links) != 2 {
		t.Errorf("return_value.links should have 2 items, got %v", rvMap["links"])
	}
}

func TestEnrichCommandResponseData_ReturnValueNil(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": true,
		"result": null,
		"effective_url": "https://example.com"
	}`)

	responseData := map[string]any{}
	EnrichCommandResponseData(result, responseData, "exec_test_456")

	// return_value should still be surfaced even when null
	if _, ok := responseData["return_value"]; !ok {
		t.Error("return_value should be surfaced even when null")
	}
}

func TestEnrichCommandResponseData_NoResultField(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": true,
		"action": "click",
		"selector": "#btn"
	}`)

	responseData := map[string]any{}
	EnrichCommandResponseData(result, responseData)

	// return_value should NOT be surfaced when there's no "result" field in extension response
	if _, ok := responseData["return_value"]; ok {
		t.Error("return_value should not be surfaced when no result field in extension response")
	}
}

func TestEnrichCommandResponseData_MatchedSurfacedTopLevel(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": true,
		"action": "click",
		"matched": {
			"tag": "button",
			"role": "button",
			"classes": ["btn", "btn-primary"],
			"text_preview": "Submit",
			"bbox": {"x": 12, "y": 34, "width": 120, "height": 32}
		}
	}`)

	responseData := map[string]any{}
	embeddedErr, hasErr := EnrichCommandResponseData(result, responseData)
	if hasErr {
		t.Fatalf("unexpected embedded error: %s", embeddedErr)
	}

	matched, ok := responseData["matched"].(map[string]any)
	if !ok {
		t.Fatal("matched should be surfaced at top level")
	}
	if matched["tag"] != "button" {
		t.Fatalf("matched.tag = %v, want button", matched["tag"])
	}
	if matched["text_preview"] != "Submit" {
		t.Fatalf("matched.text_preview = %v, want Submit", matched["text_preview"])
	}
	bbox, ok := matched["bbox"].(map[string]any)
	if !ok {
		t.Fatal("matched.bbox should be present")
	}
	if bbox["width"] != float64(120) {
		t.Fatalf("matched.bbox.width = %v, want 120", bbox["width"])
	}
}

func TestEnrichCommandResponseDataPromotesRichActionFields(t *testing.T) {
	t.Parallel()
	responseData := map[string]any{"result": json.RawMessage(`{
		"success":true,
		"dom_summary":"2 added",
		"timing":{"total_ms":42},
		"dom_changes":{"added":2,"summary":"2 added"},
		"analysis":"click completed in 42ms"
	}`)}
	if embeddedError, failed := EnrichCommandResponseData(responseData["result"].(json.RawMessage), responseData); failed || embeddedError != "" {
		t.Fatalf("enrichment error = %q, %t", embeddedError, failed)
	}
	StripEnrichedFieldsFromResult(responseData)
	for _, key := range []string{"dom_summary", "timing", "dom_changes", "analysis"} {
		if responseData[key] == nil {
			t.Fatalf("rich field %q missing: %#v", key, responseData)
		}
	}
	var nested map[string]any
	if err := json.Unmarshal(responseData["result"].(json.RawMessage), &nested); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"dom_summary", "timing", "dom_changes", "analysis"} {
		if _, duplicated := nested[key]; duplicated {
			t.Fatalf("nested result duplicates %q: %#v", key, nested)
		}
	}
}

// ============================================
// StripEnrichedFieldsFromResult tests
// ============================================

func TestEnrichCommandResponseData_AmbiguousCandidatesSurfacedTopLevel(t *testing.T) {
	result := json.RawMessage(`{
		"success":false,
		"error":"ambiguous_target",
		"match_count":3,
		"candidates":[
			{"element_id":"el_1","visible":true},
			{"element_id":"el_2","visible":true},
			{"element_id":"el_3","visible":false}
		]
	}`)
	responseData := map[string]any{}
	if embeddedErr, hasErr := EnrichCommandResponseData(result, responseData); !hasErr || embeddedErr != "ambiguous_target" {
		t.Fatalf("embedded error = %q, %v", embeddedErr, hasErr)
	}
	candidates, ok := responseData["candidates"].([]any)
	if !ok || len(candidates) != 3 || responseData["match_count"] != float64(3) {
		t.Fatalf("ambiguous promotion = %#v", responseData)
	}
}

func TestStripEnrichedFieldsFromResult_RemovesDuplicates(t *testing.T) {
	t.Parallel()

	responseData := map[string]any{
		"effective_url":   "https://example.com",
		"effective_title": "Example",
		"match_count":     float64(1),
		"match_strategy":  "selector",
		"matched":         map[string]any{"tag": "button"},
		"result": json.RawMessage(`{
			"success": true,
			"action": "click",
			"selector": "[data-testid='btn']",
			"timing": {"total_ms": 223},
			"effective_url": "https://example.com",
			"effective_title": "Example",
			"match_count": 1,
			"match_strategy": "selector",
			"matched": {"tag":"button"},
			"target_context": {"source": "tracked_tab"},
			"content_script_status": "loaded"
		}`),
	}

	StripEnrichedFieldsFromResult(responseData)

	var resultMap map[string]any
	if err := json.Unmarshal(responseData["result"].(json.RawMessage), &resultMap); err != nil {
		t.Fatalf("failed to unmarshal stripped result: %v", err)
	}

	// Non-enriched fields must survive
	if resultMap["success"] != true {
		t.Error("success should be preserved in result")
	}
	if resultMap["action"] != "click" {
		t.Error("action should be preserved in result")
	}
	if resultMap["selector"] != "[data-testid='btn']" {
		t.Error("selector should be preserved in result")
	}

	// Enriched fields must be stripped
	for _, key := range []string{"timing", "effective_url", "effective_title", "match_count", "match_strategy", "matched", "target_context", "content_script_status"} {
		if _, ok := resultMap[key]; ok {
			t.Errorf("%s should be stripped from result after enrichment", key)
		}
	}
}

func TestStripEnrichedFieldsFromResult_NoResultField(t *testing.T) {
	t.Parallel()
	responseData := map[string]any{"status": "complete"}
	// Should not panic
	StripEnrichedFieldsFromResult(responseData)
}

func TestStripEnrichedFieldsFromResult_EmptyResult(t *testing.T) {
	t.Parallel()
	responseData := map[string]any{"result": json.RawMessage(`{}`)}
	StripEnrichedFieldsFromResult(responseData)

	var resultMap map[string]any
	if err := json.Unmarshal(responseData["result"].(json.RawMessage), &resultMap); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(resultMap) != 0 {
		t.Errorf("expected empty result map, got %v", resultMap)
	}
}

func TestStripEnrichedFieldsFromResult_InvalidJSON(t *testing.T) {
	t.Parallel()
	responseData := map[string]any{"result": json.RawMessage(`not json`)}
	// Should not panic, should leave result unchanged
	StripEnrichedFieldsFromResult(responseData)
	if string(responseData["result"].(json.RawMessage)) != "not json" {
		t.Error("invalid JSON result should be left unchanged")
	}
}

// ============================================
// attachTraceSummary slim output tests
// ============================================

func TestStripSuccessOnlyFields_RemovesRoutingFields(t *testing.T) {
	t.Parallel()

	responseData := map[string]any{
		"status":                "complete",
		"target_context":        map[string]any{"source": "tracked_tab"},
		"content_script_status": "loaded",
		"created_at":            "2026-02-26T00:00:00Z",
		"trace_id":              "dom_click_123",
		"effective_url":         "https://example.com",
	}

	StripSuccessOnlyFields(responseData)

	for _, key := range []string{"target_context", "content_script_status", "created_at", "trace_id"} {
		if _, ok := responseData[key]; ok {
			t.Errorf("%s should be stripped from successful response", key)
		}
	}

	// effective_url should survive
	if responseData["effective_url"] != "https://example.com" {
		t.Error("effective_url should not be stripped")
	}
}

func TestStripRetryContextOnSuccess_RemovesAttempt1_Int(t *testing.T) {
	t.Parallel()

	responseData := map[string]any{
		"retry_context": map[string]any{
			"attempt":      1, // int from attachRetryContext
			"max_attempts": 2,
			"reason":       "success",
		},
	}

	StripRetryContextOnSuccess(responseData)

	if _, ok := responseData["retry_context"]; ok {
		t.Error("retry_context should be stripped when attempt=1 on success (int)")
	}
}

func TestStripRetryContextOnSuccess_RemovesAttempt1_Float(t *testing.T) {
	t.Parallel()

	responseData := map[string]any{
		"retry_context": map[string]any{
			"attempt":      float64(1), // float64 from JSON unmarshal
			"max_attempts": float64(2),
			"reason":       "success",
		},
	}

	StripRetryContextOnSuccess(responseData)

	if _, ok := responseData["retry_context"]; ok {
		t.Error("retry_context should be stripped when attempt=1 on success (float64)")
	}
}

func TestStripRetryContextOnSuccess_KeepsAttempt2(t *testing.T) {
	t.Parallel()

	responseData := map[string]any{
		"retry_context": map[string]any{
			"attempt":      2, // int from attachRetryContext
			"max_attempts": 2,
			"reason":       "success",
		},
	}

	StripRetryContextOnSuccess(responseData)

	if _, ok := responseData["retry_context"]; !ok {
		t.Error("retry_context should be kept when attempt > 1")
	}
}

func TestStripRetryContextOnSuccess_NoRetryContext(t *testing.T) {
	t.Parallel()
	responseData := map[string]any{"status": "complete"}
	// Should not panic
	StripRetryContextOnSuccess(responseData)
}

// ============================================
// blocked_by_overlay playbook tests (#319)
// ============================================

func TestAnnotateInteractFailureRecovery_BlockedByOverlay(t *testing.T) {
	t.Parallel()

	result := json.RawMessage(`{
		"success": false,
		"error": "blocked_by_overlay",
		"message": "Element is behind a modal overlay. Use dismiss_top_overlay first."
	}`)

	responseData := map[string]any{}
	AnnotateInteractFailureRecovery(responseData, "blocked_by_overlay", result)

	if responseData["error_code"] != "blocked_by_overlay" {
		t.Errorf("expected error_code 'blocked_by_overlay', got %v", responseData["error_code"])
	}
	if responseData["retryable"] != true {
		t.Error("blocked_by_overlay should be marked retryable")
	}

	retry, ok := responseData["retry"].(string)
	if !ok || retry == "" {
		t.Error("retry suggestion should be present for blocked_by_overlay")
	}

	hint, ok := responseData["hint"].(string)
	if !ok || hint == "" {
		t.Error("hint should be present for blocked_by_overlay")
	}
}

// ============================================
// StripSummaryModeFields tests (#447)
// ============================================

func TestStripSummaryModeFields_RemovesVerboseFields(t *testing.T) {
	t.Parallel()
	responseData := map[string]any{
		"status":             "complete",
		"dom_summary":        "added 3 elements, removed 1",
		"dom_mutations":      []any{map[string]any{"type": "added", "tag": "div"}},
		"dom_changes":        map[string]any{"added": 3, "removed": 1, "modified": 0, "summary": "x"},
		"perf_diff":          map[string]any{"fcp_delta_ms": 100},
		"evidence":           map[string]any{"before": "data:...", "after": "data:..."},
		"transient_elements": []any{map[string]any{"type": "toast"}},
		"trace":              map[string]any{"trace_id": "t-1", "timeline": "queued→sent→complete"},
		"timing":             map[string]any{"total_ms": 150},
		"matched":            map[string]any{"tag": "button", "text_preview": "Submit"},
		"effective_url":      "https://example.com",
	}

	StripSummaryModeFields(responseData)

	for _, key := range []string{"dom_summary", "dom_mutations", "perf_diff", "evidence", "transient_elements", "trace"} {
		if _, ok := responseData[key]; ok {
			t.Errorf("%s should be stripped in summary mode", key)
		}
	}

	// Keep essential fields
	for _, key := range []string{"status", "dom_changes", "timing", "matched", "effective_url"} {
		if _, ok := responseData[key]; !ok {
			t.Errorf("%s should survive summary mode stripping", key)
		}
	}
}

func TestStripSummaryModeFields_KeepsErrorFields(t *testing.T) {
	t.Parallel()
	responseData := map[string]any{
		"status":  "error",
		"error":   "element_not_found",
		"message": "No element matches selector",
		"retry":   "Try a different selector",
		"hint":    "diagnostic hint",
	}

	StripSummaryModeFields(responseData)

	for _, key := range []string{"error", "message", "retry", "hint"} {
		if _, ok := responseData[key]; !ok {
			t.Errorf("%s should survive summary mode (needed for error recovery)", key)
		}
	}
}

func TestStripSummaryModeFields_NoOpOnEmptyData(t *testing.T) {
	t.Parallel()
	responseData := map[string]any{"status": "complete"}
	// Should not panic
	StripSummaryModeFields(responseData)
	if _, ok := responseData["status"]; !ok {
		t.Error("status should survive")
	}
}
