// Purpose: Tests for analyze draw-mode annotation retrieval.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_annotations_draw_test.go — Tests for enriched annotation detail fields
// and draw history/session handlers.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mediaapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate/annotations"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestToolGetAnnotationDetail_EnrichedFields(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID:  "detail_enriched",
		Selector:       "button.primary",
		Tag:            "button",
		TextContent:    "Submit",
		Classes:        []string{"primary"},
		ComputedStyles: map[string]string{"color": "rgb(0,0,0)"},
		OuterHTML:      `<button class="primary">Submit</button>`,
		ShadowDOM:      json.RawMessage(`{"status":"open","children":2}`),
		AllElements:    json.RawMessage(`[{"tag":"button","text":"Submit"},{"tag":"div","text":"Wrapper"}]`),
		ElementCount:   2,
		IframeContent:  json.RawMessage(`[{"type":"same-origin","url":"https://example.com/frame"}]`),
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

	// Verify outer_html contains "button"
	outerHTML, ok := data["outer_html"].(string)
	if !ok {
		t.Fatal("expected outer_html to be a string")
	}
	if !strings.Contains(outerHTML, "button") {
		t.Errorf("expected outer_html to contain 'button', got %q", outerHTML)
	}

	// Verify shadow_dom is an object with "status"
	shadowDOM, ok := data["shadow_dom"].(map[string]any)
	if !ok {
		t.Fatal("expected shadow_dom to be an object")
	}
	if shadowDOM["status"] != "open" {
		t.Errorf("expected shadow_dom.status='open', got %v", shadowDOM["status"])
	}

	// Verify all_elements is an array with 2 items
	allElements, ok := data["all_elements"].([]any)
	if !ok {
		t.Fatal("expected all_elements to be an array")
	}
	if len(allElements) != 2 {
		t.Errorf("expected 2 all_elements items, got %d", len(allElements))
	}

	// Verify element_count
	if data["element_count"] != float64(2) {
		t.Errorf("expected element_count=2, got %v", data["element_count"])
	}

	// Verify iframe_content is an array
	iframeContent, ok := data["iframe_content"].([]any)
	if !ok {
		t.Fatal("expected iframe_content to be an array")
	}
	if len(iframeContent) != 1 {
		t.Errorf("expected 1 iframe_content item, got %d", len(iframeContent))
	}
}

func TestToolGetAnnotationDetail_OmitsEmptyEnrichedFields(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID:  "detail_basic",
		Selector:       "div.wrapper",
		Tag:            "div",
		TextContent:    "Hello",
		Classes:        []string{"wrapper"},
		ComputedStyles: map[string]string{"display": "block"},
	}
	h.annotationStore.StoreDetail("detail_basic", detail)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_basic"}`)

	resp := h.annotationAnalysis.GetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v\nraw text: %s", err, text)
	}

	for _, field := range []string{"outer_html", "shadow_dom", "all_elements", "element_count", "iframe_content"} {
		if _, exists := data[field]; exists {
			t.Errorf("expected field %q to be absent when empty, but it was present with value %v", field, data[field])
		}
	}
}

func TestToolListDrawHistory_EmptyDir(t *testing.T) {
	h := createTestToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "draw_history"}`)

	resp := h.analyzeDispatcher.DrawHistory(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v\nraw text: %s", err, text)
	}

	// count should be present (may be 0 or more depending on existing files)
	if _, exists := data["count"]; !exists {
		t.Error("expected 'count' field in response")
	}

	sessions, ok := data["sessions"].([]any)
	if !ok {
		t.Fatal("expected 'sessions' to be an array")
	}

	// Verify count matches sessions length
	count := data["count"].(float64)
	if int(count) != len(sessions) {
		t.Errorf("expected count=%d to match sessions length=%d", int(count), len(sessions))
	}
}

func TestToolListDrawHistory_WithSessions(t *testing.T) {
	h := createTestToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "draw_history"}`)

	resp := h.analyzeDispatcher.DrawHistory(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v\nraw text: %s", err, text)
	}

	// Verify response has the expected shape
	if _, exists := data["count"]; !exists {
		t.Error("expected 'count' field in response")
	}
	if _, exists := data["sessions"]; !exists {
		t.Error("expected 'sessions' field in response")
	}
	if _, exists := data["storage_dir"]; !exists {
		t.Error("expected 'storage_dir' field in response")
	}

	// Verify sessions is an array (may be empty or populated)
	if _, ok := data["sessions"].([]any); !ok {
		t.Error("expected 'sessions' to be an array")
	}
}

func TestToolGetDrawSession_MissingFile(t *testing.T) {
	h := createTestToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"file": "draw-session-nonexistent.json"}`)

	resp := h.analyzeDispatcher.DrawSession(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' error, got %q", text)
	}
}

func TestToolGetDrawSession_PathTraversal(t *testing.T) {
	h := createTestToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"file": "../../../etc/passwd"}`)

	resp := h.analyzeDispatcher.DrawSession(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "path traversal") {
		t.Errorf("expected 'path traversal' error, got %q", text)
	}
}

func TestToolGetDrawSession_MissingParam(t *testing.T) {
	h := createTestToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{}`)

	resp := h.analyzeDispatcher.DrawSession(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "Required parameter 'file'") {
		t.Errorf("expected missing 'file' parameter error, got %q", text)
	}
}

func TestToolGetDrawSession_HydratesStoreForGenerators(t *testing.T) {
	h := createTestToolHandler(t)

	t.Setenv("KABOOM_STATE_DIR", t.TempDir())
	dir, err := mediaapi.ScreenshotsDir()
	if err != nil {
		t.Fatalf("screenshotsDir: %v", err)
	}

	fileName := "draw-session-77-1700000000000.json"
	filePath := filepath.Join(dir, fileName)
	payload := `{
		"annotations": [{
			"id": "ann-1",
			"rect": {"x": 10, "y": 20, "width": 160, "height": 40},
			"text": "Button contrast is low",
			"timestamp": 1700000000000,
			"page_url": "https://example.com/checkout",
			"element_summary": "Checkout button",
			"correlation_id": "corr-qa-1"
		}],
		"element_details": {
			"corr-qa-1": {
				"selector": "button.checkout",
				"tag": "button",
				"text_content": "Checkout",
				"classes": ["checkout"],
				"computed_styles": {"color": "rgb(120,120,120)"},
				"parent_selector": "body",
				"bounding_rect": {"x": 10, "y": 20, "width": 160, "height": 40}
			}
		},
		"page_url": "https://example.com/checkout",
		"tab_id": 77,
		"screenshot": "/tmp/test.png",
		"timestamp": 1700000000000,
		"annot_session_name": "qa-review"
	}`
	if err := os.WriteFile(filePath, []byte(payload), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	loadResp := h.analyzeDispatcher.DrawSession(req, json.RawMessage(`{"file":"`+fileName+`"}`))
	loadText := unmarshalMCPText(t, loadResp.Result)
	if !strings.Contains(loadText, `"annot_session":"qa-review"`) {
		t.Fatalf("draw_session should expose annot_session alias, got: %s", loadText)
	}

	reportResp := annotations.HandleAnnotationReport(h, req, json.RawMessage(`{"annot_session":"qa-review"}`))
	reportText := unmarshalMCPText(t, reportResp.Result)
	if !strings.Contains(reportText, "# Annotation Report") {
		t.Fatalf("annotation_report should render report, got: %s", reportText)
	}
	if !strings.Contains(reportText, "Button contrast is low") {
		t.Fatalf("annotation_report missing loaded annotation text, got: %s", reportText)
	}

	detailResp := h.annotationAnalysis.GetAnnotationDetail(req, json.RawMessage(`{"correlation_id":"corr-qa-1"}`))
	detailText := unmarshalMCPText(t, detailResp.Result)
	if !strings.Contains(detailText, `"selector":"button.checkout"`) {
		t.Fatalf("annotation detail should be hydrated from file, got: %s", detailText)
	}
}
