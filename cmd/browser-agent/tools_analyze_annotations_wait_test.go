// Purpose: Tests for analyze annotation processing.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_annotations_wait_test.go — Annotation waiting, filtering, and named-session tests.
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestToolGetAnnotations_WaitTrue_ImmediateIfDataReady(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
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
	args := json.RawMessage(`{"what": "annotations", "background":false, "timeout_ms": 10}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "already-done") {
		t.Errorf("expected annotation text, got %q", text)
	}
}

func TestToolGetAnnotations_WaitTrue_BlocksAndReturnsSessionWithinTimeout(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
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
	args := json.RawMessage(`{"what":"annotations","background":false,"timeout_ms":250}`)

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
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	h.annotationStore.MarkDrawStarted()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotations","background":false,"timeout_ms":10}`)

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
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	// Track completed commands
	var completedID string
	var completedResult json.RawMessage
	h.annotationStore.SetCommandCompleter(func(corrID string, result json.RawMessage) {
		completedID = corrID
		completedResult = result
	})

	h.annotationStore.MarkDrawStarted()

	// Call with background=false — returns correlation_id after the bounded wait.
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "background":false}`)
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
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	var completedResult json.RawMessage
	h.annotationStore.SetCommandCompleter(func(_ string, result json.RawMessage) {
		completedResult = result
	})

	h.annotationStore.MarkDrawStarted()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotations","background":false,"timeout_ms":10,"url":"http://localhost:3000/*"}`)
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
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	var completedResult json.RawMessage
	h.annotationStore.SetCommandCompleter(func(_ string, result json.RawMessage) {
		completedResult = result
	})

	h.annotationStore.MarkDrawStarted()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotations","annot_session":"qa","background":false,"timeout_ms":10,"url":"http://localhost:3000/*"}`)
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
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	// No session exists, background=true — return immediately with no-data message.
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "background":true}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "No annotation") {
		t.Errorf("expected no annotation message, got %q", text)
	}
}

func TestToolGetAnnotations_NamedSession(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
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
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what":"annotations","annot_session":"nonexistent","background":true}`)

	resp := h.annotationAnalysis.GetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found message, got %q", text)
	}
}

func TestToolGetAnnotations_NamedSession_MultiProjectScopeWarningWithoutFilter(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
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
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
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
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
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
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
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

func TestToolGetAnnotations_Flush_UsesExplicitURLFilterWhenWaiterMissing(t *testing.T) {
	h := createTestToolHandler(t)
	replaceAnnotationStoreForTest(h, annotation.NewStore(10*time.Minute))
	defer h.annotationStore.Close()

	h.annotationStore.StoreSession(1, &annotation.Session{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		PageURL:     "http://localhost:5173/dashboard",
		Annotations: []annotation.Annotation{{Text: "wrong project"}},
	})

	corrID := "ann_flush_filter_fallback"
	h.capture.Queries().RegisterCommand(corrID, "", 10*time.Minute)

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
