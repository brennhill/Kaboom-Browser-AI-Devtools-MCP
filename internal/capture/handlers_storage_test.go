// Purpose: Tests for capture pipeline handlers storage behavior.
// Docs: docs/features/feature/backend-log-streaming/index.md

// handlers_storage_test.go — Unit tests for handleStorageGet and handleStorageRecalculate.
package capture

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleStorageGet_Success(t *testing.T) {
	c := NewCapture()
	w := httptest.NewRecorder()
	NewHTTPHandlers(c).handleStorageGet(w)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
}

func TestHandleStorageRecalculate_Success(t *testing.T) {
	c := NewCapture()
	w := httptest.NewRecorder()
	NewHTTPHandlers(c).handleStorageRecalculate(w)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
	if body["storage"] == nil {
		t.Error("expected storage field in response")
	}
}
