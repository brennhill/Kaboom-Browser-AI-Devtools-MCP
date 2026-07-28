// Purpose: Tests for analyze annotation processing.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_annotations_test.go — Tests for analyze annotations handlers.
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

// unmarshalMCPText extracts the text from an MCP tool response.
func unmarshalMCPText(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	return result.Content[0].Text
}

func TestToolGetAnnotations_NoSession(t *testing.T) {
	h := createTestToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotations","wait":false}`)

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
	h.annotationStore = annotation.NewStore(10 * time.Minute)
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
	h.annotationStore = annotation.NewStore(10 * time.Minute)
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
	args := json.RawMessage(`{"what": "annotations", "wait": true, "timeout_ms": 10}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "wait-immediate") {
		t.Errorf("expected annotation text, got %q", text)
	}
}

func TestToolGetAnnotations_WaitTrue_ReturnsCorrelationID(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	h.annotationStore.MarkDrawStarted()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "wait": true, "timeout_ms": 10}`)

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
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	h.annotationStore.MarkDrawStarted()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}

	waitResp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what":"annotations","wait":true,"timeout_ms":10}`))
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

	cmd, found := h.capture.GetCommandResult(corrID)
	if !found || cmd == nil {
		t.Fatal("flushed command should exist in command tracker")
	}
	if cmd.Status != "complete" {
		t.Fatalf("flushed command status = %q, want complete", cmd.Status)
	}
}

func TestToolGetAnnotations_Flush_IsIdempotent(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	// Seed currently-available data, then mark draw start so wait=true still returns pending.
	h.annotationStore.StoreSession(1, &annotation.Session{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		Annotations: []annotation.Annotation{{Text: "available-before-flush"}},
		PageURL:     "https://example.com",
	})
	h.annotationStore.MarkDrawStarted()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	waitResp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what":"annotations","wait":true,"timeout_ms":10}`))
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

	cmd, found := h.capture.GetCommandResult(corrID)
	if !found || cmd == nil {
		t.Fatal("command should still be queryable after repeated flush")
	}
	if cmd.Status != "complete" {
		t.Fatalf("command status after repeated flush = %q, want complete", cmd.Status)
	}
}

func TestToolGetAnnotations_Flush_MissingCorrelationID(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
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
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what":"annotations","operation":"invalid"}`))

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "Invalid annotations operation") {
		t.Fatalf("expected invalid operation error, got: %s", text)
	}
}

func TestToolGetAnnotations_WaitTrue_ImmediateIfDataReady(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	h.annotationStore.MarkDrawStarted()
	time.Sleep(2 * time.Millisecond) // ensure session timestamp > draw start

	// Store session BEFORE calling wait — should return immediately
	h.annotationStore.StoreSession(1, &annotation.Session{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		Annotations: []annotation.Annotation{{Text: "already-done"}},
		PageURL:     "https://example.com",
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "wait": true, "timeout_ms": 10}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "already-done") {
		t.Errorf("expected annotation text, got %q", text)
	}
}

func TestToolGetAnnotations_WaitTrue_BlocksAndReturnsSessionWithinTimeout(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	h.annotationStore.MarkDrawStarted()

	go func() {
		time.Sleep(15 * time.Millisecond)
		h.annotationStore.StoreSession(1, &annotation.Session{
			TabID:       1,
			Timestamp:   time.Now().UnixMilli(),
			Annotations: []annotation.Annotation{{Text: "arrived-during-blocking-wait"}},
			PageURL:     "https://example.com",
		})
	}()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotations","wait":true,"timeout_ms":250}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "arrived-during-blocking-wait") {
		t.Fatalf("expected blocking wait to return session payload, got: %s", text)
	}
	if strings.Contains(text, "waiting_for_user") {
		t.Fatalf("expected completed payload, got waiting response: %s", text)
	}
}

func TestToolGetAnnotations_WaitTrue_TimesOutToCorrelationFallback(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	h.annotationStore.MarkDrawStarted()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotations","wait":true,"timeout_ms":10}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if data["status"] != "waiting_for_user" {
		t.Fatalf("expected waiting_for_user fallback, got %v", data["status"])
	}
	if _, ok := data["correlation_id"].(string); !ok {
		t.Fatalf("expected correlation_id in fallback response, got %v", data["correlation_id"])
	}
}

func TestToolGetAnnotations_WaitTrue_WaiterCompletedOnStore(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	// Track completed commands
	var completedID string
	var completedResult json.RawMessage
	h.annotationStore.SetCommandCompleter(func(corrID string, result json.RawMessage) {
		completedID = corrID
		completedResult = result
	})

	h.annotationStore.MarkDrawStarted()

	// Call wait=true — returns correlation_id immediately
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "wait": true}`)
	resp := h.annotationAnalysis.GetAnnotations(req, args)

	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	corrID := data["correlation_id"].(string)

	// Now store annotations — should trigger the waiter completion
	h.annotationStore.StoreSession(1, &annotation.Session{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		Annotations: []annotation.Annotation{{Text: "async-result"}},
		PageURL:     "https://example.com",
	})

	if completedID != corrID {
		t.Errorf("expected completed correlation_id %q, got %q", corrID, completedID)
	}
	if !strings.Contains(string(completedResult), "async-result") {
		t.Errorf("expected result to contain annotation text, got %s", completedResult)
	}
}

func TestToolGetAnnotations_WaitTrue_WaiterCompletedOnStore_UsesURLFilter(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	var completedResult json.RawMessage
	h.annotationStore.SetCommandCompleter(func(_ string, result json.RawMessage) {
		completedResult = result
	})

	h.annotationStore.MarkDrawStarted()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotations","wait":true,"timeout_ms":10,"url":"http://localhost:3000/*"}`)
	resp := h.annotationAnalysis.GetAnnotations(req, args)

	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if data["status"] != "waiting_for_user" {
		t.Fatalf("expected waiting response, got %v", data["status"])
	}

	h.annotationStore.StoreSession(1, &annotation.Session{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		PageURL:     "http://localhost:5173/dashboard",
		Annotations: []annotation.Annotation{{Text: "not-in-scope"}},
	})

	var completed map[string]any
	if err := json.Unmarshal(completedResult, &completed); err != nil {
		t.Fatalf("failed to unmarshal completed result: %v", err)
	}
	if completed["count"] != float64(0) {
		t.Fatalf("expected filtered async result count 0, got %v", completed["count"])
	}
	if completed["filter_applied"] != "http://localhost:3000/*" {
		t.Fatalf("expected filter_applied in async result, got %v", completed["filter_applied"])
	}
}

func TestToolGetAnnotations_WaitTrue_NamedWaiterCompletedOnStore_UsesURLFilter(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	var completedResult json.RawMessage
	h.annotationStore.SetCommandCompleter(func(_ string, result json.RawMessage) {
		completedResult = result
	})

	h.annotationStore.MarkDrawStarted()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotations","annot_session":"qa","wait":true,"timeout_ms":10,"url_pattern":"http://localhost:3000/*"}`)
	resp := h.annotationAnalysis.GetAnnotations(req, args)

	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if data["status"] != "waiting_for_user" {
		t.Fatalf("expected waiting response, got %v", data["status"])
	}

	h.annotationStore.AppendToNamedSession("qa", &annotation.Session{
		TabID:       2,
		Timestamp:   time.Now().UnixMilli(),
		PageURL:     "http://localhost:5173/settings",
		Annotations: []annotation.Annotation{{Text: "wrong-project"}},
	})

	var completed map[string]any
	if err := json.Unmarshal(completedResult, &completed); err != nil {
		t.Fatalf("failed to unmarshal completed result: %v", err)
	}
	if completed["page_count"] != float64(0) {
		t.Fatalf("expected filtered named async page_count 0, got %v", completed["page_count"])
	}
	if completed["total_count"] != float64(0) {
		t.Fatalf("expected filtered named async total_count 0, got %v", completed["total_count"])
	}
	if completed["filter_applied"] != "http://localhost:3000/*" {
		t.Fatalf("expected filter_applied in named async result, got %v", completed["filter_applied"])
	}
}

func TestToolGetAnnotations_WaitFalse_DefaultBehavior(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	// No session exists, wait=false — should return immediately with no-data message
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "wait": false}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "No annotation") {
		t.Errorf("expected no annotation message, got %q", text)
	}
}

func TestToolGetAnnotations_NamedSession(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	h.annotationStore.AppendToNamedSession("qa", &annotation.Session{
		TabID:       1,
		Timestamp:   100,
		PageURL:     "https://example.com/login",
		Annotations: []annotation.Annotation{{Text: "fix button"}},
	})
	h.annotationStore.AppendToNamedSession("qa", &annotation.Session{
		TabID:          1,
		Timestamp:      200,
		PageURL:        "https://example.com/dashboard",
		ScreenshotPath: "/tmp/dash.png",
		Annotations:    []annotation.Annotation{{Text: "wrong color"}, {Text: "misaligned"}},
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "annot_session": "qa"}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if data["annot_session_name"] != "qa" {
		t.Errorf("expected annot_session_name 'qa', got %v", data["annot_session_name"])
	}
	if data["page_count"] != float64(2) {
		t.Errorf("expected page_count 2, got %v", data["page_count"])
	}
	if data["total_count"] != float64(3) {
		t.Errorf("expected total_count 3, got %v", data["total_count"])
	}

	pages, ok := data["pages"].([]any)
	if !ok || len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %v", data["pages"])
	}
	p1 := pages[0].(map[string]any)
	if p1["page_url"] != "https://example.com/login" {
		t.Errorf("expected first page URL, got %v", p1["page_url"])
	}
	p2 := pages[1].(map[string]any)
	if p2["screenshot"] != "/tmp/dash.png" {
		t.Errorf("expected screenshot on page 2, got %v", p2["screenshot"])
	}
}

func TestToolGetAnnotations_NamedSession_NotFound(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotations","annot_session":"nonexistent","wait":false}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found message, got %q", text)
	}
}

func TestToolGetAnnotations_NamedSession_MultiProjectScopeWarningWithoutFilter(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	h.annotationStore.AppendToNamedSession("qa", &annotation.Session{
		TabID:       1,
		Timestamp:   100,
		PageURL:     "http://localhost:3000/dashboard",
		Annotations: []annotation.Annotation{{Text: "fix dashboard spacing"}},
	})
	h.annotationStore.AppendToNamedSession("qa", &annotation.Session{
		TabID:       2,
		Timestamp:   200,
		PageURL:     "http://localhost:5173/settings",
		Annotations: []annotation.Annotation{{Text: "fix settings contrast"}},
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what":"annotations","annot_session":"qa"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if data["scope_ambiguous"] != true {
		t.Fatalf("expected scope_ambiguous=true, got %v", data["scope_ambiguous"])
	}
	if _, ok := data["scope_warning"].(map[string]any); !ok {
		t.Fatalf("expected scope_warning object, got %T", data["scope_warning"])
	}
	projects, ok := data["projects"].([]any)
	if !ok || len(projects) != 2 {
		t.Fatalf("expected 2 project summaries, got %v", data["projects"])
	}
}

func TestToolGetAnnotations_NamedSession_URLFilterScoped(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	h.annotationStore.AppendToNamedSession("qa", &annotation.Session{
		TabID:       1,
		Timestamp:   100,
		PageURL:     "http://localhost:3000/dashboard",
		Annotations: []annotation.Annotation{{Text: "fix dashboard spacing"}},
	})
	h.annotationStore.AppendToNamedSession("qa", &annotation.Session{
		TabID:       2,
		Timestamp:   200,
		PageURL:     "http://localhost:5173/settings",
		Annotations: []annotation.Annotation{{Text: "fix settings contrast"}, {Text: "fix settings tooltip"}},
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what":"annotations","annot_session":"qa","url":"http://localhost:5173/*"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if data["filter_applied"] != "http://localhost:5173/*" {
		t.Fatalf("expected filter_applied, got %v", data["filter_applied"])
	}
	if data["scope_ambiguous"] == true {
		t.Fatalf("scope_ambiguous should be false when filter is applied")
	}
	if data["page_count"] != float64(1) {
		t.Fatalf("expected page_count=1, got %v", data["page_count"])
	}
	if data["total_count"] != float64(2) {
		t.Fatalf("expected total_count=2 for filtered page, got %v", data["total_count"])
	}
}

func TestToolGetAnnotations_AnonymousURLFilterNoMatch(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	h.annotationStore.StoreSession(1, &annotation.Session{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		PageURL:     "http://localhost:3000/dashboard",
		Annotations: []annotation.Annotation{{Text: "fix dashboard spacing"}},
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what":"annotations","url":"http://localhost:5173/*"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if data["count"] != float64(0) {
		t.Fatalf("expected filtered count=0, got %v", data["count"])
	}
	if _, ok := data["message"].(string); !ok {
		t.Fatalf("expected no-match message when url filter does not match")
	}
}

func TestToolGetAnnotations_AnonymousBaseURLFilter_DoesNotCrossPortPrefix(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	h.annotationStore.StoreSession(1, &annotation.Session{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		PageURL:     "http://localhost:30001/dashboard",
		Annotations: []annotation.Annotation{{Text: "wrong project by port"}},
	})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what":"annotations","url":"http://localhost:3000"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if data["count"] != float64(0) {
		t.Fatalf("expected base-url filter to reject different port, got count %v", data["count"])
	}
}

func TestToolGetAnnotations_ConflictingURLFilterParams(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what":"annotations","url":"http://localhost:3000/*","url_pattern":"http://localhost:5173/*"}`))
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "Conflicting annotation scope filters") {
		t.Fatalf("expected conflicting filter validation error, got: %s", text)
	}
}

func TestToolGetAnnotations_Flush_UsesExplicitURLFilterWhenWaiterMissing(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = annotation.NewStore(10 * time.Minute)
	defer h.annotationStore.Close()

	h.annotationStore.StoreSession(1, &annotation.Session{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		PageURL:     "http://localhost:5173/dashboard",
		Annotations: []annotation.Annotation{{Text: "wrong project"}},
	})

	corrID := "ann_flush_filter_fallback"
	h.capture.RegisterCommand(corrID, "", 10*time.Minute)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	resp := h.annotationAnalysis.GetAnnotations(req, json.RawMessage(`{"what":"annotations","operation":"flush","correlation_id":"`+corrID+`","url":"http://localhost:3000/*"}`))
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("failed to parse flush response: %v", err)
	}
	resultPayload, ok := data["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result payload object, got: %T", data["result"])
	}
	if resultPayload["count"] != float64(0) {
		t.Fatalf("expected explicit flush filter to scope result, got count %v", resultPayload["count"])
	}
	if resultPayload["filter_applied"] != "http://localhost:3000/*" {
		t.Fatalf("expected filter_applied from explicit flush filter, got %v", resultPayload["filter_applied"])
	}
}

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
	h.annotationStore = annotation.NewStore(10 * time.Minute)
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
	h.annotationStore = annotation.NewStore(10 * time.Minute)
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
	h.annotationStore = annotation.NewStore(10 * time.Minute)
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
	h.annotationStore = annotation.NewStore(10 * time.Minute)
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
	h.annotationStore = annotation.NewStore(10 * time.Minute)
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
