// contracts_test.go — HTTP method, gating, and invalid-payload contracts.

package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// HTTP handlers: method enforcement
// ---------------------------------------------------------------------------

func testHandlers(enabled bool) *Handlers {
	return NewHandlers(nil, enabled, testJSONResponder)
}

func testJSONResponder(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func TestHandlersHandleFileRead_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/file/read", nil)
	w := httptest.NewRecorder()

	testHandlers(true).HandleFileRead(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandlersHandleFileDialogInject_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/file/dialog/inject", nil)
	w := httptest.NewRecorder()

	testHandlers(true).HandleFileDialogInject(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandlersHandleFormSubmit_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/form/submit", nil)
	w := httptest.NewRecorder()

	testHandlers(true).HandleFormSubmit(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandlersHandleOSAutomation_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/os-automation/inject", nil)
	w := httptest.NewRecorder()

	testHandlers(true).HandleOSAutomation(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandlersHandleOSAutomation_Disabled(t *testing.T) {
	body := strings.NewReader(`{}`)
	req := httptest.NewRequest("POST", "/api/os-automation/inject", body)
	w := httptest.NewRecorder()

	testHandlers(false).HandleOSAutomation(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("disabled OS automation should return 403, got %d", w.Code)
	}
}

func TestHandlersHandleOSAutomationDismiss_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()

	testHandlers(true).HandleOSAutomationDismiss(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandlersHandleOSAutomationDismiss_Disabled(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()

	testHandlers(false).HandleOSAutomationDismiss(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("disabled should return 403, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// HTTP handlers: invalid JSON
// ---------------------------------------------------------------------------

func TestHandlersHandleFileRead_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`not-json`)
	req := httptest.NewRequest("POST", "/api/file/read", body)
	w := httptest.NewRecorder()

	testHandlers(true).HandleFileRead(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON should return 400, got %d", w.Code)
	}
}

func TestHandlersHandleFormSubmit_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest("POST", "/api/form/submit", body)
	w := httptest.NewRecorder()

	testHandlers(true).HandleFormSubmit(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON should return 400, got %d", w.Code)
	}
}
