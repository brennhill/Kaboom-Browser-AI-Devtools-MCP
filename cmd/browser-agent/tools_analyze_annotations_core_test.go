// Purpose: Tests for analyze annotation processing.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_annotations_core_test.go — Core annotation list, flush, and response-shape tests.
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestToolGetAnnotations_NoSession(t *testing.T) {
	h := createTestToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotations","background":true}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "No annotation") {
		t.Errorf("expected no annotation message, got %q", text)
	}
}

func TestToolGetAnnotations_WithSession(t *testing.T) {
	h := createTestToolHandler(t)

	// Store a session
	session := &annotation.Session{
		Annotations: []annotation.Annotation{
			{
				ID:             "ann_1",
				Text:           "make this darker",
				ElementSummary: "button.primary 'Submit'",
				CorrelationID:  "detail_1",
				Rect:           annotation.Rect{X: 100, Y: 200, Width: 150, Height: 50},
			},
		},
		ScreenshotPath: "/tmp/test.png",
		PageURL:        "https://example.com",
		TabID:          1,
		Timestamp:      time.Now().UnixMilli(),
	}
	h.annotationStore.StoreSession(1, session)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations"}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "make this darker") {
		t.Errorf("expected annotation text in result, got %q", text)
	}
	if !strings.Contains(text, "/tmp/test.png") {
		t.Errorf("expected screenshot path in result, got %q", text)
	}
}

func TestToolGetAnnotationDetail_Missing(t *testing.T) {
	h := createTestToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "nonexistent"}`)

	resp := h.annotationAnalysis.GetAnnotationDetail(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "not found") && !strings.Contains(text, "expired") {
		t.Errorf("expected not found error, got %q", text)
	}
}

func TestToolGetAnnotationDetail_Found(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID:  "detail_1",
		Selector:       "button.primary",
		Tag:            "button",
		TextContent:    "Submit",
		Classes:        []string{"primary"},
		ComputedStyles: map[string]string{"color": "rgb(255,255,255)"},
	}
	h.annotationStore.StoreDetail("detail_1", detail)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_1"}`)

	resp := h.annotationAnalysis.GetAnnotationDetail(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "button.primary") {
		t.Errorf("expected selector in result, got %q", text)
	}
}

func TestToolGetAnnotations_FullResponseShape(t *testing.T) {
	h := createTestToolHandler(t)

	session := &annotation.Session{
		Annotations: []annotation.Annotation{
			{
				ID:             "ann_1",
				Text:           "make this darker",
				ElementSummary: "button.primary 'Submit'",
				CorrelationID:  "detail_1",
				Rect:           annotation.Rect{X: 100, Y: 200, Width: 150, Height: 50},
				PageURL:        "https://example.com",
				Timestamp:      1700000000000,
			},
			{
				ID:             "ann_2",
				Text:           "increase font",
				ElementSummary: "p.body 'Lorem'",
				CorrelationID:  "detail_2",
				Rect:           annotation.Rect{X: 300, Y: 400, Width: 200, Height: 30},
				PageURL:        "https://example.com",
				Timestamp:      1700000001000,
			},
		},
		ScreenshotPath: "/tmp/annotated.png",
		PageURL:        "https://example.com",
		TabID:          1,
		Timestamp:      1700000002000,
	}
	h.annotationStore.StoreSession(1, session)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations"}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v\nraw text: %s", err, text)
	}

	// Verify top-level fields
	if data["count"] != float64(2) {
		t.Errorf("Expected count=2, got %v", data["count"])
	}
	if data["page_url"] != "https://example.com" {
		t.Errorf("Expected page_url, got %v", data["page_url"])
	}
	if data["screenshot"] != "/tmp/annotated.png" {
		t.Errorf("Expected screenshot path, got %v", data["screenshot"])
	}

	// Verify annotations array
	anns, ok := data["annotations"].([]any)
	if !ok || len(anns) != 2 {
		t.Fatalf("Expected annotations array with 2 items, got %v", data["annotations"])
	}

	first := anns[0].(map[string]any)
	for _, field := range []string{"id", "text", "element_summary", "correlation_id", "rect"} {
		if _, exists := first[field]; !exists {
			t.Errorf("Missing field %q in annotation", field)
		}
	}
	if first["text"] != "make this darker" {
		t.Errorf("Expected text 'make this darker', got %v", first["text"])
	}
	if first["correlation_id"] != "detail_1" {
		t.Errorf("Expected correlation_id 'detail_1', got %v", first["correlation_id"])
	}

	// Verify rect sub-object
	rect, ok := first["rect"].(map[string]any)
	if !ok {
		t.Fatal("Expected rect to be an object")
	}
	if rect["x"] != float64(100) || rect["width"] != float64(150) {
		t.Errorf("Unexpected rect values: %v", rect)
	}
}

func TestToolGetAnnotationDetail_FullResponseShape(t *testing.T) {
	h := createTestToolHandler(t)

	detail := annotation.Detail{
		CorrelationID:  "detail_full",
		Selector:       "button#submit-btn",
		Tag:            "button",
		TextContent:    "Submit Order",
		Classes:        []string{"primary", "rounded"},
		ID:             "submit-btn",
		ComputedStyles: map[string]string{"background-color": "rgb(59, 130, 246)", "font-size": "14px"},
		ParentSelector: "form.checkout > div.actions > button#submit-btn",
		BoundingRect:   annotation.Rect{X: 100, Y: 200, Width: 150, Height: 50},
	}
	h.annotationStore.StoreDetail("detail_full", detail)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_full"}`)

	resp := h.annotationAnalysis.GetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v\nraw text: %s", err, text)
	}

	checks := map[string]any{
		"correlation_id":  "detail_full",
		"selector":        "button#submit-btn",
		"tag":             "button",
		"text_content":    "Submit Order",
		"id":              "submit-btn",
		"parent_selector": "form.checkout > div.actions > button#submit-btn",
	}
	for field, expected := range checks {
		if data[field] != expected {
			t.Errorf("Field %q: expected %v, got %v", field, expected, data[field])
		}
	}

	// Verify classes array
	classes, ok := data["classes"].([]any)
	if !ok || len(classes) != 2 {
		t.Fatalf("Expected classes array with 2 items, got %v", data["classes"])
	}

	// Verify computed_styles
	styles, ok := data["computed_styles"].(map[string]any)
	if !ok {
		t.Fatal("Expected computed_styles to be an object")
	}
	if styles["background-color"] != "rgb(59, 130, 246)" {
		t.Errorf("Expected background-color, got %v", styles["background-color"])
	}

	// Verify bounding_rect
	rect, ok := data["bounding_rect"].(map[string]any)
	if !ok {
		t.Fatal("Expected bounding_rect to be an object")
	}
	if rect["x"] != float64(100) || rect["width"] != float64(150) {
		t.Errorf("Unexpected bounding_rect: %v", rect)
	}
}

func TestToolGetAnnotations_ZeroAnnotationsFlow(t *testing.T) {
	h := createTestToolHandler(t)

	// Use a fresh store to avoid cross-test contamination
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
	args := json.RawMessage(`{"what": "annotations"}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if data["count"] != float64(0) {
		t.Errorf("Expected count=0, got %v", data["count"])
	}
	anns, ok := data["annotations"].([]any)
	if !ok {
		t.Fatal("Expected annotations to be an array")
	}
	if len(anns) != 0 {
		t.Errorf("Expected empty annotations array, got %d items", len(anns))
	}
	if data["screenshot"] != "/tmp/empty.png" {
		t.Errorf("Expected screenshot path, got %v", data["screenshot"])
	}
}

func TestToolGetAnnotations_WaitTrue_ImmediateReturn(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	// Mark draw started, then store session
	h.annotationStore.MarkDrawStarted()
	time.Sleep(1 * time.Millisecond)
	h.annotationStore.StoreSession(1, &annotation.Session{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		Annotations: []annotation.Annotation{{Text: "wait-immediate"}},
		PageURL:     "https://example.com",
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "background":false, "timeout_ms": 10}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "wait-immediate") {
		t.Errorf("expected annotation text, got %q", text)
	}
}

func TestToolGetAnnotations_WaitTrue_ReturnsCorrelationID(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	h.annotationStore.MarkDrawStarted()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "background":false, "timeout_ms": 10}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if data["status"] != "waiting_for_user" {
		t.Errorf("expected status 'waiting_for_user', got %v", data["status"])
	}
	corrID, ok := data["correlation_id"].(string)
	if !ok || corrID == "" {
		t.Error("expected non-empty correlation_id")
	}
	if !strings.HasPrefix(corrID, "ann_") {
		t.Errorf("expected correlation_id prefix 'ann_', got %q", corrID)
	}
	if !strings.Contains(text, "observe") {
		t.Error("expected polling instructions in message")
	}
}

func TestToolGetAnnotations_Flush_CompletesPendingCommand_WithEmptyResultReason(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	h.annotationStore.MarkDrawStarted()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}

	waitResp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what":"annotations","background":false,"timeout_ms":10}`))
	waitText := unmarshalMCPText(t, waitResp.Result)
	waitJSON := extractJSONFromText(waitText)

	var waiting map[string]any
	if err := json.Unmarshal([]byte(waitJSON), &waiting); err != nil {
		t.Fatalf("failed to parse waiting response: %v", err)
	}
	corrID, ok := waiting["correlation_id"].(string)
	if !ok || corrID == "" {
		t.Fatalf("expected correlation_id in wait response, got: %v", waiting["correlation_id"])
	}

	flushResp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what":"annotations","operation":"flush","correlation_id":"`+corrID+`"}`))
	flushText := unmarshalMCPText(t, flushResp.Result)
	flushJSON := extractJSONFromText(flushText)

	var flushed map[string]any
	if err := json.Unmarshal([]byte(flushJSON), &flushed); err != nil {
		t.Fatalf("failed to parse flush response: %v", err)
	}

	if flushed["status"] != "complete" {
		t.Fatalf("flush status = %v, want complete", flushed["status"])
	}
	if final, _ := flushed["final"].(bool); !final {
		t.Fatalf("flush should produce final=true, got: %v", flushed["final"])
	}
	if flushed["terminal_reason"] != "abandoned" {
		t.Fatalf("terminal_reason = %v, want abandoned", flushed["terminal_reason"])
	}
	resultPayload, ok := flushed["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result payload object, got: %T", flushed["result"])
	}
	if resultPayload["count"] != float64(0) {
		t.Fatalf("result.count = %v, want 0", resultPayload["count"])
	}
	if resultPayload["filter_applied"] != "none" {
		t.Fatalf("result.filter_applied = %v, want none", resultPayload["filter_applied"])
	}

	cmd, found := h.capture.Queries().GetCommandResult(corrID)
	if !found || cmd == nil {
		t.Fatal("flushed command should exist in command tracker")
	}
	if cmd.Status != "complete" {
		t.Fatalf("flushed command status = %q, want complete", cmd.Status)
	}
}

func TestToolGetAnnotations_Flush_IsIdempotent(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	// Seed currently-available data, then mark draw start so blocking mode still returns pending.
	h.annotationStore.StoreSession(1, &annotation.Session{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		Annotations: []annotation.Annotation{{Text: "available-before-flush"}},
		PageURL:     "https://example.com",
	})
	h.annotationStore.MarkDrawStarted()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	waitResp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what":"annotations","background":false,"timeout_ms":10}`))
	waitText := unmarshalMCPText(t, waitResp.Result)
	waitJSON := extractJSONFromText(waitText)

	var waiting map[string]any
	if err := json.Unmarshal([]byte(waitJSON), &waiting); err != nil {
		t.Fatalf("failed to parse waiting response: %v", err)
	}
	corrID := waiting["correlation_id"].(string)

	flushArgs := json.RawMessage(`{"what":"annotations","operation":"flush","correlation_id":"` + corrID + `"}`)
	first := h.annotationAnalysis.GetAnnotations(req, flushArgs)
	second := h.annotationAnalysis.GetAnnotations(req, flushArgs)

	firstText := unmarshalMCPText(t, first.Result)
	secondText := unmarshalMCPText(t, second.Result)

	var firstData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(firstText)), &firstData); err != nil {
		t.Fatalf("failed to parse first flush response: %v", err)
	}
	var secondData map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(secondText)), &secondData); err != nil {
		t.Fatalf("failed to parse second flush response: %v", err)
	}

	if firstData["status"] != "complete" || secondData["status"] != "complete" {
		t.Fatalf("flush should be complete both times, got first=%v second=%v", firstData["status"], secondData["status"])
	}
	if firstData["terminal_reason"] != "flushed" {
		t.Fatalf("first terminal_reason = %v, want flushed", firstData["terminal_reason"])
	}
	if secondData["terminal_reason"] != "flushed" {
		t.Fatalf("second terminal_reason = %v, want flushed", secondData["terminal_reason"])
	}

	cmd, found := h.capture.Queries().GetCommandResult(corrID)
	if !found || cmd == nil {
		t.Fatal("command should still be queryable after repeated flush")
	}
	if cmd.Status != "complete" {
		t.Fatalf("command status after repeated flush = %q, want complete", cmd.Status)
	}
}

func TestToolGetAnnotations_Flush_MissingCorrelationID(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what":"annotations","operation":"flush"}`))

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "correlation_id") {
		t.Fatalf("expected missing correlation_id error, got: %s", text)
	}
}

func TestToolGetAnnotations_InvalidOperation(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what":"annotations","operation":"invalid"}`))

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "Invalid annotations operation") {
		t.Fatalf("expected invalid operation error, got: %s", text)
	}
}
