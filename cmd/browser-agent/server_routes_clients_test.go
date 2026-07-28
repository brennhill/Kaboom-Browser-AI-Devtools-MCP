// Purpose: Tests for client management route handlers.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// server_routes_clients_test.go — Tests for handleClientsList and handleClientByID.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// mockClientRegistry implements capture.ClientRegistry for testing.
type mockClientRegistry struct {
	mu      sync.RWMutex
	clients map[string]map[string]any
	order   []string
	nextID  int
}

func newMockClientRegistry() *mockClientRegistry {
	return &mockClientRegistry{
		clients: make(map[string]map[string]any),
	}
}

func (m *mockClientRegistry) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

func (m *mockClientRegistry) List() any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]map[string]any, 0, len(m.clients))
	for _, id := range m.order {
		if c, ok := m.clients[id]; ok {
			result = append(result, c)
		}
	}
	return result
}

func (m *mockClientRegistry) Register(cwd string) any {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	id := fmt.Sprintf("client-%d", m.nextID)
	cs := map[string]any{
		"id":  id,
		"cwd": cwd,
	}
	m.clients[id] = cs
	m.order = append(m.order, id)
	return cs
}

func (m *mockClientRegistry) Get(id string) any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c, ok := m.clients[id]; ok {
		return c
	}
	return nil
}

func (m *mockClientRegistry) Unregister(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.clients[id]; !ok {
		return false
	}
	delete(m.clients, id)
	nextOrder := make([]string, 0, len(m.order))
	for _, existingID := range m.order {
		if existingID != id {
			nextOrder = append(nextOrder, existingID)
		}
	}
	m.order = nextOrder
	return true
}

// newCaptureWithRegistry creates a capture instance with a mock client registry.
func newCaptureWithRegistry(t *testing.T) *capture.Capture {
	t.Helper()
	cap := capture.NewCapture()
	cap.Clients().Set(newMockClientRegistry())
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
	csAny := cap.Clients().Registry().Register("/tmp/project")
	csJSON, _ := json.Marshal(csAny)
	var cs map[string]any
	if err := json.Unmarshal(csJSON, &cs); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	clientID, _ := cs["id"].(string)

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
	registeredMap, ok := registered.(map[string]any)
	if !ok {
		t.Fatalf("expected mock registry to return map, got %T", registered)
	}
	clientID, ok := registeredMap["id"].(string)
	if !ok || clientID == "" {
		t.Fatalf("expected registered client id, got %#v", registeredMap["id"])
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
