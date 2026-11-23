// handler.go — HTTP ingestion and delivery for browser-to-client push events.

package pushapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

// JSONResponse writes one JSON HTTP response.
type JSONResponse func(http.ResponseWriter, int, any)

// Handler owns push HTTP parsing and delivery.
type Handler struct {
	router       *push.Router
	inbox        *push.PushInbox
	runtime      *Runtime
	jsonResponse JSONResponse
	maxBody      int64
}

// NewHandler creates the push HTTP handler.
func NewHandler(router *push.Router, inbox *push.PushInbox, runtime *Runtime, jsonResponse JSONResponse, maxBody int64) *Handler {
	return &Handler{router: router, inbox: inbox, runtime: runtime, jsonResponse: jsonResponse, maxBody: maxBody}
}

func EventID(prefix string) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return prefix + "-" + fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(random[:])
}

// HandleScreenshot receives and routes a screenshot push.
func (handler *Handler) HandleScreenshot(w http.ResponseWriter, request *http.Request) {
	if !handler.requireJSONPost(w, request, "push_screenshot") {
		return
	}
	var body struct {
		ScreenshotDataURL string `json:"screenshot_data_url"`
		Note              string `json:"note"`
		PageURL           string `json:"page_url"`
		TabID             int    `json:"tab_id"`
	}
	request.Body = http.MaxBytesReader(w, request.Body, handler.maxBody)
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		handler.jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "push_screenshot: invalid JSON body. Send a valid JSON object."})
		return
	}
	screenshot := body.ScreenshotDataURL
	if comma := strings.IndexByte(screenshot, ','); comma >= 0 {
		screenshot = screenshot[comma+1:]
	}
	handler.deliver(w, push.PushEvent{
		ID: EventID("push-ss"), Type: "screenshot", Timestamp: time.Now(),
		PageURL: body.PageURL, TabID: body.TabID, ScreenshotB64: screenshot, Note: body.Note,
	}, "push_screenshot")
}

// HandleMessage receives and routes a chat push.
func (handler *Handler) HandleMessage(w http.ResponseWriter, request *http.Request) {
	if !handler.requireJSONPost(w, request, "push_message") {
		return
	}
	var body struct {
		Message string `json:"message"`
		PageURL string `json:"page_url"`
		TabID   int    `json:"tab_id"`
	}
	request.Body = http.MaxBytesReader(w, request.Body, handler.maxBody)
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		handler.jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "push_message: invalid JSON body. Send a valid JSON object with a 'message' field."})
		return
	}
	if strings.TrimSpace(body.Message) == "" {
		handler.jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "push_message: message field is empty. Provide a non-empty message."})
		return
	}
	handler.deliver(w, push.PushEvent{
		ID: EventID("push-chat"), Type: "chat", Timestamp: time.Now(),
		PageURL: body.PageURL, TabID: body.TabID, Message: body.Message,
	}, "push_message")
}

func (handler *Handler) requireJSONPost(w http.ResponseWriter, request *http.Request, operation string) bool {
	if request.Method != http.MethodPost {
		handler.jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": operation + ": method not allowed. Use POST method."})
		return false
	}
	contentType := request.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		handler.jsonResponse(w, http.StatusUnsupportedMediaType, map[string]string{"error": operation + ": unsupported content type. Set Content-Type to application/json."})
		return false
	}
	return true
}

func (handler *Handler) deliver(w http.ResponseWriter, event push.PushEvent, operation string) {
	status := "queued"
	method := string(push.DeliveredViaInbox)
	if handler.router != nil {
		result, err := handler.router.DeliverPush(event)
		if err != nil {
			handler.jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": operation + ": delivery failed. " + err.Error()})
			return
		}
		method = string(result.Method)
		if result.Method == push.DeliveredViaSampling {
			status = "delivered"
		}
	} else if handler.inbox != nil {
		handler.inbox.Enqueue(event)
	}
	handler.jsonResponse(w, http.StatusOK, map[string]any{"status": status, "event_id": event.ID, "delivery_method": method})
}

// HandleCapabilities reports negotiated delivery capabilities and inbox depth.
func (handler *Handler) HandleCapabilities(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		handler.jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "push_capabilities: method not allowed. Use GET method."})
		return
	}
	caps := handler.runtime.Capabilities()
	inboxCount := 0
	if handler.inbox != nil {
		inboxCount = handler.inbox.Len()
	}
	handler.jsonResponse(w, http.StatusOK, map[string]any{
		"push_enabled": caps.SupportsSampling || caps.SupportsNotifications, "supports_sampling": caps.SupportsSampling,
		"supports_notifications": caps.SupportsNotifications, "client_name": caps.ClientName, "inbox_count": inboxCount,
	})
}

// HandleDrain returns and clears queued push events.
func (handler *Handler) HandleDrain(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		handler.jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if handler.inbox == nil {
		handler.jsonResponse(w, http.StatusOK, map[string]any{"events": []any{}, "count": 0})
		return
	}
	events := handler.inbox.DrainAll()
	if len(events) == 0 {
		handler.jsonResponse(w, http.StatusOK, map[string]any{"events": []any{}, "count": 0})
		return
	}
	handler.jsonResponse(w, http.StatusOK, map[string]any{"events": events, "count": len(events)})
}

// DeliverAnnotations routes completed draw-mode annotations.
func DeliverAnnotations(router *push.Router, pageURL string, tabID int, session string, annotations []annotation.Annotation) {
	if router == nil {
		return
	}
	encoded, err := json.Marshal(annotations)
	if err != nil {
		return
	}
	_, _ = router.DeliverPush(push.PushEvent{
		ID: EventID("push-ann"), Type: "annotations", Timestamp: time.Now(),
		PageURL: pageURL, TabID: tabID, Annotations: encoded, AnnotSession: session,
	})
}
