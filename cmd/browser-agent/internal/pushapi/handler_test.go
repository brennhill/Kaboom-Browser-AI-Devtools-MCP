// handler_test.go — Tests for push HTTP parsing and delivery.
package pushapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

type testPushServer struct {
	inbox   *push.PushInbox
	router  *push.Router
	runtime *Runtime
	handler *Handler
}

func testJSONResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func newTestPushServer() *testPushServer {
	inbox := push.NewPushInbox(50)
	router := push.NewRouter(inbox, nil, nil, push.ClientCapabilities{})
	runtime := NewRuntime(func([]byte, bridge.StdioFraming) {})
	return &testPushServer{inbox: inbox, router: router, runtime: runtime, handler: NewHandler(router, inbox, runtime, testJSONResponse, 1<<20)}
}

type testPushNotifier struct {
	method string
	params map[string]any
	calls  int
}

func (n *testPushNotifier) SendNotification(method string, params map[string]any) {
	n.calls++
	n.method = method
	n.params = params
}

func TestHandlePushMessage_Success(t *testing.T) {
	s := newTestPushServer()
	body := `{"message":"hello world","page_url":"https://example.com","tab_id":1}`
	req := httptest.NewRequest("POST", "/push/message", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.HandleMessage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "queued" {
		t.Fatalf("expected queued, got %s", resp["status"])
	}
	if resp["delivery_method"] != "inbox" {
		t.Fatalf("expected delivery_method=inbox, got %v", resp["delivery_method"])
	}
	if resp["event_id"] == "" {
		t.Fatal("expected event_id")
	}
}

func TestHandlePushMessage_EmptyMessage(t *testing.T) {
	s := newTestPushServer()
	body := `{"message":"   ","page_url":"https://example.com"}`
	req := httptest.NewRequest("POST", "/push/message", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.HandleMessage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty message, got %d", w.Code)
	}
}

func TestHandlePushMessage_InvalidJSON(t *testing.T) {
	s := newTestPushServer()
	req := httptest.NewRequest("POST", "/push/message", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.HandleMessage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestHandlePushMessage_WrongMethod(t *testing.T) {
	s := newTestPushServer()
	req := httptest.NewRequest("GET", "/push/message", nil)
	w := httptest.NewRecorder()

	s.handler.HandleMessage(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandlePushMessage_WrongContentType(t *testing.T) {
	s := newTestPushServer()
	req := httptest.NewRequest("POST", "/push/message", strings.NewReader("hello"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	s.handler.HandleMessage(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", w.Code)
	}
	// Verify error follows {OPERATION}: {ROOT_CAUSE}. {RECOVERY_ACTION} format
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp["error"], "push_message:") {
		t.Fatalf("error should start with operation prefix, got: %s", resp["error"])
	}
}

func TestHandlePushScreenshot_Success(t *testing.T) {
	s := newTestPushServer()
	body := `{"screenshot_data_url":"data:image/png;base64,iVBOR","page_url":"https://example.com","tab_id":1}`
	req := httptest.NewRequest("POST", "/push/screenshot", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.HandleScreenshot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["delivery_method"] != "inbox" {
		t.Fatalf("expected delivery_method=inbox, got %v", resp["delivery_method"])
	}
}

func TestHandlePushScreenshot_StripsDataPrefix(t *testing.T) {
	s := newTestPushServer()
	body := `{"screenshot_data_url":"data:image/png;base64,AAAA","page_url":"https://example.com","tab_id":1}`
	req := httptest.NewRequest("POST", "/push/screenshot", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.HandleScreenshot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// Verify the event in inbox has stripped prefix
	events := s.inbox.DrainAll()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ScreenshotB64 != "AAAA" {
		t.Fatalf("expected stripped base64 'AAAA', got '%s'", events[0].ScreenshotB64)
	}
}

func TestHandlePushCapabilities(t *testing.T) {
	s := newTestPushServer()

	s.runtime.SetCapabilities(push.ClientCapabilities{
		SupportsSampling: true,
		ClientName:       "test-client",
	})

	req := httptest.NewRequest("GET", "/push/capabilities", nil)
	w := httptest.NewRecorder()

	s.handler.HandleCapabilities(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["push_enabled"] != true {
		t.Fatal("expected push_enabled=true")
	}
	if resp["client_name"] != "test-client" {
		t.Fatalf("expected test-client, got %s", resp["client_name"])
	}
}

func TestHandlePushMessage_NotificationRouting(t *testing.T) {
	inbox := push.NewPushInbox(50)
	notifier := &testPushNotifier{}
	router := push.NewRouter(inbox, nil, notifier, push.ClientCapabilities{
		SupportsSampling:      false,
		SupportsNotifications: true,
		ClientName:            "codex",
	})
	runtime := NewRuntime(func([]byte, bridge.StdioFraming) {})
	s := &testPushServer{inbox: inbox, router: router, runtime: runtime, handler: NewHandler(router, inbox, runtime, testJSONResponse, 1<<20)}

	body := `{"message":"notify me","page_url":"https://example.com","tab_id":1}`
	req := httptest.NewRequest("POST", "/push/message", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handler.HandleMessage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "queued" {
		t.Fatalf("expected queued status on notification path, got %v", resp["status"])
	}
	if resp["delivery_method"] != "notification" {
		t.Fatalf("expected delivery_method=notification, got %v", resp["delivery_method"])
	}
	if notifier.calls != 1 {
		t.Fatalf("expected 1 notification call, got %d", notifier.calls)
	}
	if notifier.method != "notifications/message" {
		t.Fatalf("expected notifications/message method, got %q", notifier.method)
	}
	if inbox.Len() != 1 {
		t.Fatalf("expected inbox fallback enqueue after notification, got %d events", inbox.Len())
	}
}

func TestPushEventID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := EventID("push-test")
		if seen[id] {
			t.Fatalf("duplicate ID: %s", id)
		}
		seen[id] = true
	}
}

func TestPushEventID_HasPrefix(t *testing.T) {
	id := EventID("push-chat")
	if !strings.HasPrefix(id, "push-chat-") {
		t.Fatalf("expected push-chat- prefix, got %s", id)
	}
}
