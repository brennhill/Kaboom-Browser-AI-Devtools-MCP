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

func testJSONResponder(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func TestHandleFileReadHTTP_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/file/read", nil)
	w := httptest.NewRecorder()

	HandleFileReadHTTP(w, req, nil, testJSONResponder)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandleFileDialogInjectHTTP_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/file/dialog/inject", nil)
	w := httptest.NewRecorder()

	HandleFileDialogInjectHTTP(w, req, nil, testJSONResponder)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandleFormSubmitHTTP_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/form/submit", nil)
	w := httptest.NewRecorder()

	HandleFormSubmitHTTP(w, req, nil, testJSONResponder)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandleOSAutomationHTTP_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/os-automation/inject", nil)
	w := httptest.NewRecorder()

	HandleOSAutomationHTTP(w, req, true, nil, testJSONResponder)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandleOSAutomationHTTP_Disabled(t *testing.T) {
	body := strings.NewReader(`{}`)
	req := httptest.NewRequest("POST", "/api/os-automation/inject", body)
	w := httptest.NewRecorder()

	HandleOSAutomationHTTP(w, req, false, nil, testJSONResponder)

	if w.Code != http.StatusForbidden {
		t.Errorf("disabled OS automation should return 403, got %d", w.Code)
	}
}

func TestHandleOSAutomationDismissHTTP_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()

	HandleOSAutomationDismissHTTP(w, req, true, testJSONResponder)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandleOSAutomationDismissHTTP_Disabled(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()

	HandleOSAutomationDismissHTTP(w, req, false, testJSONResponder)

	if w.Code != http.StatusForbidden {
		t.Errorf("disabled should return 403, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// HTTP handlers: invalid JSON
// ---------------------------------------------------------------------------

func TestHandleFileReadHTTP_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`not-json`)
	req := httptest.NewRequest("POST", "/api/file/read", body)
	w := httptest.NewRecorder()

	HandleFileReadHTTP(w, req, nil, testJSONResponder)

	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON should return 400, got %d", w.Code)
	}
}

func TestHandleFormSubmitHTTP_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest("POST", "/api/form/submit", body)
	w := httptest.NewRecorder()

	HandleFormSubmitHTTP(w, req, nil, testJSONResponder)

	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON should return 400, got %d", w.Code)
	}
}
