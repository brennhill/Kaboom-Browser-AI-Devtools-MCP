// handler_test.go — Tests client registry HTTP list, registration, lookup, and deletion.

package clientapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/clientreg"
)

func testMux(captured *capture.Capture) *http.ServeMux {
	mux := http.NewServeMux()
	Register(mux, captured, 10<<20)
	return mux
}

func request(t *testing.T, mux http.Handler, method, path, body string) (int, map[string]any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
	httpRequest.Header.Set("X-Kaboom-Client", "kaboom-extension/test")
	mux.ServeHTTP(recorder, httpRequest)
	result := make(map[string]any)
	if recorder.Body.Len() > 0 {
		if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode %s %s: %v", method, path, err)
		}
	}
	return recorder.Code, result
}

func captureWithRegistry() *capture.Capture {
	captured := capture.NewCapture()
	captured.Clients().Set(clientreg.NewClientRegistry())
	return captured
}

func TestListAndRegisterClients(t *testing.T) {
	missingStatus, _ := request(t, testMux(capture.NewCapture()), http.MethodGet, "/clients", "")
	if missingStatus != http.StatusServiceUnavailable {
		t.Fatalf("missing registry status = %d", missingStatus)
	}

	captured := captureWithRegistry()
	mux := testMux(captured)
	status, listed := request(t, mux, http.MethodGet, "/clients", "")
	if status != http.StatusOK || listed["count"] != float64(0) || listed["clients"] == nil {
		t.Fatalf("empty list status/body = %d/%#v", status, listed)
	}
	status, registered := request(t, mux, http.MethodPost, "/clients", `{"cwd":"/tmp/project"}`)
	if status != http.StatusOK || registered["result"] == nil {
		t.Fatalf("register status/body = %d/%#v", status, registered)
	}
	_, listed = request(t, mux, http.MethodGet, "/clients", "")
	if listed["count"] != float64(1) {
		t.Fatalf("registered count = %#v", listed["count"])
	}
	status, invalid := request(t, mux, http.MethodPost, "/clients", `{invalid`)
	if status != http.StatusBadRequest || invalid["error"] != "Invalid JSON" {
		t.Fatalf("invalid JSON status/body = %d/%#v", status, invalid)
	}
	for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
		status, _ = request(t, mux, method, "/clients", "")
		if status != http.StatusMethodNotAllowed {
			t.Fatalf("%s list status = %d", method, status)
		}
	}
}

func TestGetAndDeleteClientByID(t *testing.T) {
	captured := captureWithRegistry()
	clientID := captured.Clients().Registry().Register("/tmp/project").ID
	mux := testMux(captured)

	status, client := request(t, mux, http.MethodGet, "/clients/"+clientID, "")
	if status != http.StatusOK || client["id"] != clientID {
		t.Fatalf("get status/body = %d/%#v", status, client)
	}
	for path, expectedStatus := range map[string]int{"/clients/": http.StatusBadRequest, "/clients/missing": http.StatusNotFound} {
		status, _ = request(t, mux, http.MethodGet, path, "")
		if status != expectedStatus {
			t.Fatalf("GET %s status = %d", path, status)
		}
	}
	status, deleted := request(t, mux, http.MethodDelete, "/clients/"+clientID, "")
	if status != http.StatusOK || deleted["unregistered"] != true {
		t.Fatalf("delete status/body = %d/%#v", status, deleted)
	}
	status, _ = request(t, mux, http.MethodDelete, "/clients/"+clientID, "")
	if status != http.StatusNotFound {
		t.Fatalf("repeat delete status = %d", status)
	}
	for _, method := range []string{http.MethodPut, http.MethodPatch, http.MethodPost} {
		status, _ = request(t, mux, method, "/clients/missing", "")
		if status != http.StatusMethodNotAllowed {
			t.Fatalf("%s client status = %d", method, status)
		}
	}
}
