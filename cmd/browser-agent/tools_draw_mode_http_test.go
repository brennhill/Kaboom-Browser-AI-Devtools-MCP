// Purpose: Tests for draw-mode HTTP endpoint handling.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_draw_mode_http_test.go — HTTP endpoint tests for draw mode completion.
// Tests the POST /draw-mode/complete handler end-to-end: JSON parsing,
// screenshot decoding/saving, annotation + detail storage.
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// createDrawModeTestServer creates an httptest server with just the draw-mode endpoint.
// Bypasses httpguard.ExtensionOnly middleware for unit testing the handler directly.
func createDrawModeTestServer(t *testing.T) (*httptest.Server, *AnnotationStore) {
	t.Helper()

	server, err := NewServer(t.TempDir()+"/test.jsonl", 100)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	store := server.getAnnotationStore()
	t.Cleanup(func() { store.Close() })

	cap := capture.NewCapture()
	mux := http.NewServeMux()
	mux.HandleFunc("/draw-mode/complete", func(w http.ResponseWriter, r *http.Request) {
		server.handleDrawModeComplete(w, r, cap)
	})
	return httptest.NewServer(mux), store
}

// Minimal valid 1x1 transparent PNG as base64.
const testPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="

func TestDrawModeComplete_EndToEnd(t *testing.T) {
	ts, store := createDrawModeTestServer(t)
	defer ts.Close()

	screenshotDataURL := "data:image/png;base64," + testPNGBase64

	payload := map[string]any{
		"screenshot_data_url": screenshotDataURL,
		"annotations": []map[string]any{
			{
				"id":              "ann_001",
				"text":            "make this darker",
				"element_summary": "button.primary 'Submit'",
				"correlation_id":  "detail_001",
				"rect":            map[string]any{"x": 100, "y": 200, "width": 150, "height": 50},
				"page_url":        "https://example.com",
				"timestamp":       1700000000000,
			},
			{
				"id":              "ann_002",
				"text":            "increase font size",
				"element_summary": "p.body-text 'Lorem ipsum'",
				"correlation_id":  "detail_002",
				"rect":            map[string]any{"x": 300, "y": 400, "width": 200, "height": 30},
				"page_url":        "https://example.com",
				"timestamp":       1700000001000,
			},
		},
		"element_details": map[string]any{
			"detail_001": map[string]any{
				"selector":        "button#submit-btn",
				"tag":             "button",
				"text_content":    "Submit",
				"classes":         []string{"primary", "rounded"},
				"id":              "submit-btn",
				"computed_styles": map[string]string{"background-color": "rgb(59, 130, 246)", "color": "#fff"},
				"parent_selector": "form.checkout > div.actions > button#submit-btn",
				"bounding_rect":   map[string]any{"x": 100, "y": 200, "width": 150, "height": 50},
			},
		},
		"page_url": "https://example.com",
		"tab_id":   42,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(ts.URL+"/draw-mode/complete", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /draw-mode/complete failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response shape
	if result["status"] != "stored" {
		t.Errorf("Expected status 'stored', got %v", result["status"])
	}
	if result["annotation_count"] != float64(2) {
		t.Errorf("Expected annotation_count 2, got %v", result["annotation_count"])
	}
	screenshotPath, _ := result["screenshot"].(string)
	if screenshotPath == "" {
		t.Error("Expected screenshot path in response")
	}

	// Verify screenshot file was written
	if _, err := os.Stat(screenshotPath); os.IsNotExist(err) {
		t.Errorf("Screenshot file does not exist at %s", screenshotPath)
	} else {
		os.Remove(screenshotPath)
	}

	// Verify session stored in annotation store.
	session := store.GetSession(42)
	if session == nil {
		t.Fatal("Expected session in annotation store for tabID 42")
	}
	if len(session.Annotations) != 2 {
		t.Fatalf("Expected 2 annotations in session, got %d", len(session.Annotations))
	}
	if session.Annotations[0].Text != "make this darker" {
		t.Errorf("Expected first annotation text 'make this darker', got %q", session.Annotations[0].Text)
	}
	if session.Annotations[1].CorrelationID != "detail_002" {
		t.Errorf("Expected second annotation correlation_id 'detail_002', got %q", session.Annotations[1].CorrelationID)
	}
	if session.PageURL != "https://example.com" {
		t.Errorf("Expected page URL 'https://example.com', got %q", session.PageURL)
	}

	// Verify element detail stored
	detail, found := store.GetDetail("detail_001")
	if !found {
		t.Fatal("Expected detail for 'detail_001' in annotation store")
	}
	if detail.Selector != "button#submit-btn" {
		t.Errorf("Expected selector 'button#submit-btn', got %q", detail.Selector)
	}
	if detail.Tag != "button" {
		t.Errorf("Expected tag 'button', got %q", detail.Tag)
	}
	if detail.ComputedStyles["background-color"] != "rgb(59, 130, 246)" {
		t.Errorf("Expected computed style 'rgb(59, 130, 246)', got %v", detail.ComputedStyles)
	}

	// detail_002 was NOT in element_details, should not be stored
	_, found002 := store.GetDetail("detail_002")
	if found002 {
		t.Error("Did not expect detail_002 to be stored (not in element_details)")
	}
}

func TestDrawModeComplete_InvalidJSON(t *testing.T) {
	ts, _ := createDrawModeTestServer(t)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/draw-mode/complete", "application/json", strings.NewReader(`{invalid`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid JSON, got %d", resp.StatusCode)
	}
}

func TestDrawModeComplete_MethodNotAllowed(t *testing.T) {
	ts, _ := createDrawModeTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/draw-mode/complete")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for GET, got %d", resp.StatusCode)
	}
}

func TestDrawModeComplete_ZeroAnnotations(t *testing.T) {
	ts, store := createDrawModeTestServer(t)
	defer ts.Close()

	payload := map[string]any{
		"annotations":     []any{},
		"element_details": map[string]any{},
		"page_url":        "https://example.com",
		"tab_id":          99,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(ts.URL+"/draw-mode/complete", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	if result["annotation_count"] != float64(0) {
		t.Errorf("Expected annotation_count 0, got %v", result["annotation_count"])
	}

	session := store.GetSession(99)
	if session == nil {
		t.Fatal("Expected session in store even with 0 annotations")
	}
	if len(session.Annotations) != 0 {
		t.Errorf("Expected 0 annotations, got %d", len(session.Annotations))
	}
}

func TestDrawModeComplete_NoScreenshot(t *testing.T) {
	ts, store := createDrawModeTestServer(t)
	defer ts.Close()

	payload := map[string]any{
		"annotations": []map[string]any{
			{
				"id":   "ann_1",
				"text": "fix this",
				"rect": map[string]any{"x": 10, "y": 20, "width": 100, "height": 50},
			},
		},
		"element_details": map[string]any{},
		"page_url":        "https://example.com",
		"tab_id":          55,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(ts.URL+"/draw-mode/complete", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	// Screenshot path should be empty when no screenshot provided
	if result["screenshot"] != "" {
		t.Errorf("Expected empty screenshot path, got %v", result["screenshot"])
	}

	session := store.GetSession(55)
	if session == nil {
		t.Fatal("Expected session in store")
	}
	if len(session.Annotations) != 1 {
		t.Errorf("Expected 1 annotation, got %d", len(session.Annotations))
	}
}

func TestDrawModeComplete_InvalidBase64(t *testing.T) {
	ts, _ := createDrawModeTestServer(t)
	defer ts.Close()

	payload := map[string]any{
		"screenshot_data_url": "data:image/png;base64,!!!not-valid-base64!!!",
		"annotations":         []any{},
		"element_details":     map[string]any{},
		"page_url":            "https://example.com",
		"tab_id":              77,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(ts.URL+"/draw-mode/complete", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 (invalid base64 should be handled gracefully), got %d", resp.StatusCode)
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	// Screenshot should be empty when base64 is invalid
	if result["screenshot"] != "" {
		t.Errorf("Expected empty screenshot for invalid base64, got %v", result["screenshot"])
	}
}

func TestDrawModeComplete_MissingTabID(t *testing.T) {
	ts, _ := createDrawModeTestServer(t)
	defer ts.Close()

	payload := map[string]any{
		"annotations":     []any{},
		"element_details": map[string]any{},
		"page_url":        "https://example.com",
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(ts.URL+"/draw-mode/complete", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing tab_id, got %d", resp.StatusCode)
	}
}

func TestDrawModeComplete_WithSessionName(t *testing.T) {
	ts, store := createDrawModeTestServer(t)
	defer ts.Close()

	screenshotDataURL := "data:image/png;base64," + testPNGBase64

	payload := map[string]any{
		"screenshot_data_url": screenshotDataURL,
		"annotations": []map[string]any{
			{
				"id":              "ann_sn_001",
				"text":            "fix header spacing",
				"element_summary": "header.main 'Logo'",
				"correlation_id":  "detail_sn_001",
				"rect":            map[string]any{"x": 0, "y": 0, "width": 400, "height": 60},
				"page_url":        "https://example.com/home",
				"timestamp":       1700000000000,
			},
		},
		"element_details": map[string]any{
			"detail_sn_001": map[string]any{
				"selector":     "header.main",
				"tag":          "header",
				"text_content": "Logo",
				"classes":      []string{"main"},
			},
		},
		"page_url":           "https://example.com/home",
		"tab_id":             200,
		"annot_session_name": "qa-review",
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(ts.URL+"/draw-mode/complete", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "stored" {
		t.Errorf("Expected status 'stored', got %v", result["status"])
	}
	if result["annotation_count"] != float64(1) {
		t.Errorf("Expected annotation_count 1, got %v", result["annotation_count"])
	}

	// Verify session stored in anonymous store by tab ID
	session := store.GetSession(200)
	if session == nil {
		t.Fatal("Expected session in annotation store for tabID 200")
	}

	// Verify session also stored in named session
	ns := store.GetNamedSession("qa-review")
	if ns == nil {
		t.Fatal("Expected named session 'qa-review' in annotation store")
	}
	if len(ns.Pages) != 1 {
		t.Fatalf("Expected 1 page in named session, got %d", len(ns.Pages))
	}
	if ns.Pages[0].Annotations[0].Text != "fix header spacing" {
		t.Errorf("Expected annotation text 'fix header spacing', got %q", ns.Pages[0].Annotations[0].Text)
	}

	// Verify element detail stored
	detail, found := store.GetDetail("detail_sn_001")
	if !found {
		t.Fatal("Expected detail for 'detail_sn_001'")
	}
	if detail.Selector != "header.main" {
		t.Errorf("Expected selector 'header.main', got %q", detail.Selector)
	}
}

func TestDrawModeComplete_MalformedAnnotationWarning(t *testing.T) {
	ts, _ := createDrawModeTestServer(t)
	defer ts.Close()

	payload := map[string]any{
		"annotations": []any{
			map[string]any{"id": "good", "text": "valid", "rect": map[string]any{"x": 0, "y": 0, "width": 10, "height": 10}},
			"not-a-json-object",
		},
		"element_details": map[string]any{},
		"page_url":        "https://example.com",
		"tab_id":          88,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(ts.URL+"/draw-mode/complete", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	if result["annotation_count"] != float64(1) {
		t.Errorf("Expected annotation_count 1 (only valid one), got %v", result["annotation_count"])
	}
	warnings, ok := result["warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Error("Expected warnings array for malformed annotation")
	}
}
