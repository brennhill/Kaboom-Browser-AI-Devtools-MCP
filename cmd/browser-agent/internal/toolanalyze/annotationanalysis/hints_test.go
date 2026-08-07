// Purpose: Tests for analyze annotation processing.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_annotation_hints_test.go — Annotation design, accessibility, and runtime hint tests.
package annotationanalysis

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestToolGetAnnotations_SessionHints_WithScreenshot(t *testing.T) {
	h := createTestToolHandler(t)

	session := &annotation.Session{
		Annotations: []annotation.Annotation{
			{ID: "ann_1", Text: "fix this", CorrelationID: "d1"},
		},
		ScreenshotPath: "/tmp/screenshot.png",
		PageURL:        "https://example.com",
		TabID:          1,
		Timestamp:      time.Now().UnixMilli(),
	}
	h.annotationStore.StoreSession(1, session)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what": "annotations"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	hints, ok := data["hints"].(map[string]any)
	if !ok {
		t.Fatal("expected hints object in session response")
	}
	checklist, ok := hints["checklist"].([]any)
	if !ok || len(checklist) == 0 {
		t.Fatal("expected non-empty checklist in hints")
	}
	if _, ok := hints["screenshot_baseline"].(string); !ok {
		t.Error("expected screenshot_baseline hint when screenshot present")
	}
}

func TestToolGetAnnotations_SessionHints_NoScreenshot(t *testing.T) {
	h := createTestToolHandler(t)

	session := &annotation.Session{
		Annotations: []annotation.Annotation{
			{ID: "ann_1", Text: "fix this", CorrelationID: "d1"},
		},
		PageURL:   "https://example.com",
		TabID:     1,
		Timestamp: time.Now().UnixMilli(),
	}
	h.annotationStore.StoreSession(1, session)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what": "annotations"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	hints, ok := data["hints"].(map[string]any)
	if !ok {
		t.Fatal("expected hints object")
	}
	if _, exists := hints["screenshot_baseline"]; exists {
		t.Error("expected screenshot_baseline to be absent when no screenshot")
	}
}

func TestToolGetAnnotationDetail_Hints_DesignSystem(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID:  "detail_tw",
		Selector:       "div.tw",
		Tag:            "div",
		Classes:        []string{"flex"},
		ComputedStyles: map[string]string{},
		CSSFramework:   "tailwind",
	}
	h.annotationStore.StoreDetail("detail_tw", detail)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_tw"}`)

	resp := h.annotationAnalysis.GetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	hints, ok := data["hints"].(map[string]any)
	if !ok {
		t.Fatal("expected hints object in detail response")
	}
	ds, ok := hints["design_system"].(string)
	if !ok || ds == "" {
		t.Error("expected design_system hint for tailwind framework")
	}
}

func TestToolGetAnnotationDetail_Hints_Accessibility(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID:  "detail_a11y_hint",
		Selector:       "div.bad",
		Tag:            "div",
		Classes:        []string{},
		ComputedStyles: map[string]string{},
		A11yFlags:      []string{"low_contrast:2.1:1"},
	}
	h.annotationStore.StoreDetail("detail_a11y_hint", detail)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_a11y_hint"}`)

	resp := h.annotationAnalysis.GetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	hints, ok := data["hints"].(map[string]any)
	if !ok {
		t.Fatal("expected hints object")
	}
	if _, ok := hints["accessibility"].(string); !ok {
		t.Error("expected accessibility hint when a11y_flags present")
	}
}

func TestToolGetAnnotationDetail_Hints_ErrorContext(t *testing.T) {
	h := createTestToolHandler(t)

	annotTS := time.Now()
	session := &annotation.Session{
		Annotations: []annotation.Annotation{
			{ID: "ann_ec", Text: "broken", CorrelationID: "detail_ec", Timestamp: annotTS.UnixMilli()},
		},
		TabID: 1, Timestamp: annotTS.UnixMilli(), PageURL: "https://example.com",
	}
	h.annotationStore.StoreSession(1, session)

	detail := annotation.Detail{
		CorrelationID:  "detail_ec",
		Selector:       "button.err",
		Tag:            "button",
		Classes:        []string{},
		ComputedStyles: map[string]string{},
	}
	h.annotationStore.StoreDetail("detail_ec", detail)

	h.server.logs.SeedEntries([]types.LogEntry{
		types.LogEntry{"level": "error", "message": "ReferenceError", "ts": annotTS.Add(-1 * time.Second).UTC().Format(time.RFC3339)},
	}, []time.Time{
		annotTS.Add(-1 * time.Second),
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotationDetail(req, json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_ec"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	hints, ok := data["hints"].(map[string]any)
	if !ok {
		t.Fatal("expected hints object")
	}
	if _, ok := hints["error_context"].(string); !ok {
		t.Error("expected error_context hint when correlated_errors present")
	}
}

func TestToolGetAnnotationDetail_NoHints_WhenNoSpecialData(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID:  "detail_plain",
		Selector:       "div.plain",
		Tag:            "div",
		Classes:        []string{},
		ComputedStyles: map[string]string{},
	}
	h.annotationStore.StoreDetail("detail_plain", detail)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotationDetail(req, json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_plain"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// No hints when there's no special data (no framework, no a11y flags, no errors)
	if _, exists := data["hints"]; exists {
		t.Error("expected hints to be absent when no special data")
	}
}

func TestToolGetAnnotations_NamedSessionHints(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	h.annotationStore.AppendToNamedSession("pm-review", &annotation.Session{
		TabID:          1,
		Timestamp:      100,
		PageURL:        "https://example.com",
		ScreenshotPath: "/tmp/ss.png",
		Annotations:    []annotation.Annotation{{Text: "fix layout"}},
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what": "annotations", "annot_session": "pm-review"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	hints, ok := data["hints"].(map[string]any)
	if !ok {
		t.Fatal("expected hints object in named session response")
	}
	if _, ok := hints["checklist"].([]any); !ok {
		t.Error("expected checklist in named session hints")
	}
}

func TestToolGetAnnotationDetail_ErrorCorrelation_CapsAt5(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	annotTS := time.Now()
	h.annotationStore.StoreSession(1, &annotation.Session{
		TabID: 1, Timestamp: annotTS.UnixMilli(), PageURL: "https://example.com",
		Annotations: []annotation.Annotation{
			{ID: "ann_cap", Text: "many errors", CorrelationID: "detail_cap", Timestamp: annotTS.UnixMilli()},
		},
	})
	h.annotationStore.StoreDetail("detail_cap", annotation.Detail{
		CorrelationID: "detail_cap", Selector: "div", Tag: "div",
		Classes: []string{}, ComputedStyles: map[string]string{},
	})

	// Inject 8 error-level entries within the window
	for i := 0; i < 8; i++ {
		offset := time.Duration(i-4) * time.Second
		h.server.logs.SeedEntries(
			[]types.LogEntry{{"level": "error", "message": "Error " + strings.Repeat("X", i), "ts": annotTS.Add(offset).UTC().Format(time.RFC3339)}},
			[]time.Time{annotTS.Add(offset)},
		)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotationDetail(req, json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_cap"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	errors, ok := data["correlated_errors"].([]any)
	if !ok {
		t.Fatal("expected correlated_errors array")
	}
	if len(errors) != 5 {
		t.Fatalf("expected exactly 5 correlated errors (capped), got %d", len(errors))
	}
}

func TestToolGetAnnotationDetail_Hints_BootstrapFramework(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID: "detail_bs", Selector: "div", Tag: "div",
		Classes: []string{}, ComputedStyles: map[string]string{},
		CSSFramework: "bootstrap",
	}
	h.annotationStore.StoreDetail("detail_bs", detail)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotationDetail(req, json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_bs"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	hintsRaw, ok := data["hints"].(map[string]any)
	if !ok {
		t.Fatal("expected hints map in response")
	}
	ds, ok := hintsRaw["design_system"].(string)
	if !ok {
		t.Fatal("expected design_system string in hints")
	}
	if !strings.Contains(ds, "Bootstrap") {
		t.Errorf("expected Bootstrap hint, got %q", ds)
	}
}

func TestToolGetAnnotationDetail_Hints_RuntimeFramework(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID:  "detail_runtime",
		Selector:       "button.save",
		Tag:            "button",
		Classes:        []string{"save"},
		ComputedStyles: map[string]string{},
		JSFramework:    "react",
	}
	h.annotationStore.StoreDetail("detail_runtime", detail)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotationDetail(req, json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_runtime"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	hintsRaw, ok := data["hints"].(map[string]any)
	if !ok {
		t.Fatal("expected hints object in annotation detail response")
	}
	runtimeHint, ok := hintsRaw["runtime_framework"].(string)
	if !ok {
		t.Fatal("expected runtime_framework string in hints")
	}
	if !strings.Contains(strings.ToLower(runtimeHint), "react") {
		t.Fatalf("expected runtime framework hint to mention react, got %q", runtimeHint)
	}
}

func TestToolGetAnnotationDetail_Hints_UnknownFramework(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID: "detail_unk", Selector: "div", Tag: "div",
		Classes: []string{}, ComputedStyles: map[string]string{},
		CSSFramework: "bulma",
	}
	h.annotationStore.StoreDetail("detail_unk", detail)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotationDetail(req, json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_unk"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	hintsRaw, ok := data["hints"].(map[string]any)
	if !ok {
		t.Fatal("expected hints map in response")
	}
	ds, ok := hintsRaw["design_system"].(string)
	if !ok {
		t.Fatal("expected design_system string in hints")
	}
	if !strings.Contains(ds, "bulma") {
		t.Errorf("expected framework name in default hint, got %q", ds)
	}
}

func TestToolGetAnnotationDetail_ErrorCorrelation_BoundaryAndShape(t *testing.T) {
	h := createTestToolHandler(t)

	// Use second-aligned time to match RFC3339 precision used by log entries
	annotTS := time.Now().Truncate(time.Second)
	session := &annotation.Session{
		Annotations: []annotation.Annotation{
			{ID: "ann_bnd", Text: "boundary test", CorrelationID: "detail_bnd", Timestamp: annotTS.UnixMilli()},
		},
		TabID: 1, Timestamp: annotTS.UnixMilli(), PageURL: "https://example.com",
	}
	h.annotationStore.StoreSession(1, session)
	h.annotationStore.StoreDetail("detail_bnd", annotation.Detail{
		CorrelationID: "detail_bnd", Selector: "div", Tag: "div",
		Classes: []string{}, ComputedStyles: map[string]string{},
	})

	// Inject errors at exactly ±5s (boundary, inclusive) and ±6s (outside window)
	h.server.logs.SeedEntries([]types.LogEntry{
		types.LogEntry{"level": "error", "message": "at minus 5s", "ts": annotTS.Add(-5 * time.Second).UTC().Format(time.RFC3339)},
		types.LogEntry{"level": "error", "message": "at plus 5s", "ts": annotTS.Add(5 * time.Second).UTC().Format(time.RFC3339)},
		types.LogEntry{"level": "error", "message": "at minus 6s", "ts": annotTS.Add(-6 * time.Second).UTC().Format(time.RFC3339)},
		types.LogEntry{"level": "error", "message": "at plus 6s", "ts": annotTS.Add(6 * time.Second).UTC().Format(time.RFC3339)},
	}, []time.Time{
		annotTS.Add(-5 * time.Second),
		annotTS.Add(5 * time.Second),
		annotTS.Add(-6 * time.Second),
		annotTS.Add(6 * time.Second),
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotationDetail(req, json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_bnd"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	errors, ok := data["correlated_errors"].([]any)
	if !ok {
		t.Fatal("expected correlated_errors array")
	}
	if len(errors) != 2 {
		t.Fatalf("expected 2 correlated errors (boundary inclusive, ±6s excluded), got %d", len(errors))
	}

	// Verify shape of each error entry
	for i, e := range errors {
		entry, ok := e.(map[string]any)
		if !ok {
			t.Fatalf("error entry %d is not a map", i)
		}
		if _, ok := entry["message"].(string); !ok {
			t.Errorf("error entry %d missing 'message' string field", i)
		}
		if _, ok := entry["ts"].(string); !ok {
			t.Errorf("error entry %d missing 'ts' string field", i)
		}
	}
}

func TestToolGetAnnotationDetail_ErrorCorrelation_TimestampFoundEmptyLogs(t *testing.T) {
	h := createTestToolHandler(t)

	annotTS := time.Now()
	session := &annotation.Session{
		Annotations: []annotation.Annotation{
			{ID: "ann_el", Text: "empty logs", CorrelationID: "detail_el", Timestamp: annotTS.UnixMilli()},
		},
		TabID: 1, Timestamp: annotTS.UnixMilli(), PageURL: "https://example.com",
	}
	h.annotationStore.StoreSession(1, session)
	h.annotationStore.StoreDetail("detail_el", annotation.Detail{
		CorrelationID: "detail_el", Selector: "div", Tag: "div",
		Classes: []string{}, ComputedStyles: map[string]string{},
	})
	// No log entries injected — h.server.logs.entries is empty

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotationDetail(req, json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_el"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if _, exists := data["correlated_errors"]; exists {
		t.Error("expected correlated_errors absent when log entries are empty")
	}
}

func TestToolGetAnnotations_ZeroAnnotations_NoHints(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	session := &annotation.Session{
		Annotations:    []annotation.Annotation{},
		ScreenshotPath: "/tmp/empty.png",
		PageURL:        "https://example.com/empty",
		TabID:          5,
		Timestamp:      time.Now().UnixMilli(),
	}
	h.annotationStore.StoreSession(5, session)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what": "annotations"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if _, exists := data["hints"]; exists {
		t.Error("expected hints to be absent for zero-annotation session")
	}
}
