// Purpose: Tests for client management route handlers.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// server_routes_clients_test.go — Tests for handleClientsList and handleClientByID.
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/clientreg"
)

// newCaptureWithRegistry creates a capture instance with the canonical registry.
func newCaptureWithRegistry(t *testing.T) *capture.Capture {
	t.Helper()
	cap := capture.NewCapture()
	cap.Clients().Set(clientreg.NewClientRegistry())
	return cap
}

func TestHandleClientsList_RegistryUnavailable(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()

	req := httptest.NewRequest("GET", "/clients", nil)
	w := httptest.NewRecorder()
	handleClientsList(w, req, cap)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", resp.StatusCode)
	}
}

// ============================================
// handleClientsList
// ============================================

func TestHandleClientsList_GET_ReturnsClientsAndCount(t *testing.T) {
	t.Parallel()
	cap := newCaptureWithRegistry(t)

	req := httptest.NewRequest("GET", "/clients", nil)
	w := httptest.NewRecorder()
	handleClientsList(w, req, cap)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := data["clients"]; !ok {
		t.Error("expected 'clients' field in response")
	}
	if _, ok := data["count"]; !ok {
		t.Error("expected 'count' field in response")
	}
	if data["count"] != float64(0) {
		t.Errorf("expected count 0 for fresh capture, got %v", data["count"])
	}
}

func TestHandleClientsList_POST_RegistersClient(t *testing.T) {
	t.Parallel()
	cap := newCaptureWithRegistry(t)

	body := strings.NewReader(`{"cwd":"/tmp/test-project"}`)
	req := httptest.NewRequest("POST", "/clients", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handleClientsList(w, req, cap)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := data["result"]; !ok {
		t.Error("expected 'result' field in response")
	}
}

func TestHandleClientsList_POST_InvalidJSON(t *testing.T) {
	t.Parallel()
	cap := newCaptureWithRegistry(t)

	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest("POST", "/clients", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handleClientsList(w, req, cap)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if data["error"] != "Invalid JSON" {
		t.Errorf("expected error 'Invalid JSON', got %v", data["error"])
	}
}

func TestHandleClientsList_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	cap := newCaptureWithRegistry(t)

	for _, method := range []string{"PUT", "DELETE", "PATCH"} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(method, "/clients", nil)
			w := httptest.NewRecorder()
			handleClientsList(w, req, cap)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("expected status 405 for %s, got %d", method, resp.StatusCode)
			}
		})
	}
}

func TestHandleClientsList_GET_CountMatchesRegistered(t *testing.T) {
	t.Parallel()
	cap := newCaptureWithRegistry(t)

	// Register a client first
	cap.Clients().Registry().Register("/tmp/project-a")

	req := httptest.NewRequest("GET", "/clients", nil)
	w := httptest.NewRecorder()
	handleClientsList(w, req, cap)

	resp := w.Result()
	defer resp.Body.Close()

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if data["count"] != float64(1) {
		t.Errorf("expected count 1 after registering, got %v", data["count"])
	}
}

func TestHandleClientsList_POST_ThenGET_ClientVisible(t *testing.T) {
	t.Parallel()
	cap := newCaptureWithRegistry(t)

	// POST to register
	postBody := strings.NewReader(`{"cwd":"/tmp/project-b"}`)
	postReq := httptest.NewRequest("POST", "/clients", postBody)
	postReq.Header.Set("Content-Type", "application/json")
	postW := httptest.NewRecorder()
	handleClientsList(postW, postReq, cap)

	if postW.Code != http.StatusOK {
		t.Fatalf("POST register failed: %d", postW.Code)
	}

	// GET to verify
	getReq := httptest.NewRequest("GET", "/clients", nil)
	getW := httptest.NewRecorder()
	handleClientsList(getW, getReq, cap)

	var data map[string]any
	if err := json.NewDecoder(getW.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode GET response: %v", err)
	}

	if data["count"] != float64(1) {
		t.Errorf("expected count 1 after POST+GET, got %v", data["count"])
	}
}

// ============================================
// handleClientByID
// ============================================

func TestHandleClientByID_GET_ExistingClient(t *testing.T) {
	t.Parallel()
	cap := newCaptureWithRegistry(t)

	// Register a client and get its ID
	clientID := cap.Clients().Registry().Register("/tmp/project").ID

	if clientID == "" {
		t.Fatal("failed to get client ID from registration")
	}

	req := httptest.NewRequest("GET", "/clients/"+clientID, nil)
	req.URL.Path = "/clients/" + clientID
	w := httptest.NewRecorder()
	handleClientByID(w, req, cap)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestHandleClientByID_GET_NonexistentClient(t *testing.T) {
	t.Parallel()
	cap := newCaptureWithRegistry(t)

	req := httptest.NewRequest("GET", "/clients/nonexistent-id", nil)
	req.URL.Path = "/clients/nonexistent-id"
	w := httptest.NewRecorder()
	handleClientByID(w, req, cap)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if data["error"] != "Client not found" {
		t.Errorf("expected error 'Client not found', got %v", data["error"])
	}
}

func TestHandleClientByID_EmptyID(t *testing.T) {
	t.Parallel()
	cap := newCaptureWithRegistry(t)

	req := httptest.NewRequest("GET", "/clients/", nil)
	req.URL.Path = "/clients/"
	w := httptest.NewRecorder()
	handleClientByID(w, req, cap)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if data["error"] != "Missing client ID" {
		t.Errorf("expected error 'Missing client ID', got %v", data["error"])
	}
}

func TestHandleClientByID_DELETE_ReturnsUnregistered(t *testing.T) {
	t.Parallel()
	cap := newCaptureWithRegistry(t)
	registered := cap.Clients().Registry().Register("/tmp/project")
	clientID := registered.ID
	if clientID == "" {
		t.Fatal("expected registered client id")
	}

	req := httptest.NewRequest("DELETE", "/clients/"+clientID, nil)
	req.URL.Path = "/clients/" + clientID
	w := httptest.NewRecorder()
	handleClientByID(w, req, cap)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if data["unregistered"] != true {
		t.Errorf("expected unregistered true, got %v", data["unregistered"])
	}
}

func TestHandleClientByID_DELETE_NonexistentClient(t *testing.T) {
	t.Parallel()
	cap := newCaptureWithRegistry(t)

	req := httptest.NewRequest("DELETE", "/clients/missing-id", nil)
	req.URL.Path = "/clients/missing-id"
	w := httptest.NewRecorder()
	handleClientByID(w, req, cap)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if data["error"] != "Client not found" {
		t.Errorf("expected error 'Client not found', got %v", data["error"])
	}
}

func TestHandleClientByID_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	cap := newCaptureWithRegistry(t)

	for _, method := range []string{"PUT", "PATCH", "POST"} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(method, "/clients/some-id", nil)
			req.URL.Path = "/clients/some-id"
			w := httptest.NewRecorder()
			handleClientByID(w, req, cap)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("expected status 405 for %s, got %d", method, resp.StatusCode)
			}
		})
	}
}
