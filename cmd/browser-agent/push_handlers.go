// push_handlers.go — Push capability state, MCP delivery, and HTTP handlers.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

var pushState struct {
	mu       sync.RWMutex
	caps     push.ClientCapabilities
	framing  bridge.StdioFraming
	onChange func(push.ClientCapabilities)
}

func setPushClientCapabilities(caps push.ClientCapabilities) {
	cb := func() func(push.ClientCapabilities) {
		pushState.mu.Lock()
		defer pushState.mu.Unlock()
		pushState.caps = caps
		return pushState.onChange
	}()
	if cb != nil {
		cb(caps)
	}
}

func getPushClientCapabilities() push.ClientCapabilities {
	pushState.mu.RLock()
	defer pushState.mu.RUnlock()
	return pushState.caps
}

func onPushCapabilitiesChange(fn func(push.ClientCapabilities)) {
	pushState.mu.Lock()
	defer pushState.mu.Unlock()
	pushState.onChange = fn
}

func storeBridgeFraming(f bridge.StdioFraming) {
	pushState.mu.Lock()
	defer pushState.mu.Unlock()
	pushState.framing = f
}

func getBridgeFraming() bridge.StdioFraming {
	pushState.mu.RLock()
	defer pushState.mu.RUnlock()
	return pushState.framing
}

func extractClientCapabilities(rawParams json.RawMessage) push.ClientCapabilities {
	if len(rawParams) == 0 {
		return push.ClientCapabilities{}
	}
	var params struct {
		Capabilities struct {
			Sampling json.RawMessage `json:"sampling"`
		} `json:"capabilities"`
		ClientInfo struct {
			Name string `json:"name"`
		} `json:"clientInfo"` // SPEC:MCP initialize params
	}
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return push.ClientCapabilities{}
	}
	caps := push.ClientCapabilities{ClientName: params.ClientInfo.Name}
	if len(params.Capabilities.Sampling) > 0 && string(params.Capabilities.Sampling) != "null" {
		caps.SupportsSampling = true
	}
	if caps.ClientName != "" {
		caps.SupportsNotifications = true
	}
	return caps
}

type stdioSamplingSender struct{}

func (s *stdioSamplingSender) SendSampling(req push.SamplingRequest) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	writeMCPPayload(payload, getBridgeFraming())
	return nil
}

type stdioNotifier struct{}

func (n *stdioNotifier) SendNotification(method string, params map[string]any) {
	notif := map[string]any{
		"jsonrpc": JSONRPCVersion,
		"method":  method,
		"params":  params,
	}
	payload, err := json.Marshal(notif)
	if err != nil {
		return
	}
	writeMCPPayload(payload, getBridgeFraming())
}

// pushEventID generates a unique event ID with the given prefix.
func pushEventID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to time-based ID on crypto failure (extremely rare)
		ts := time.Now().UnixNano()
		return prefix + "-" + fmt.Sprintf("%x", ts)
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}

// handlePushScreenshot receives a screenshot from the extension and routes it.
func (s *Server) handlePushScreenshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "push_screenshot: method not allowed. Use POST method."})
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		jsonResponse(w, http.StatusUnsupportedMediaType, map[string]string{"error": "push_screenshot: unsupported content type. Set Content-Type to application/json."})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body struct {
		ScreenshotDataURL string `json:"screenshot_data_url"`
		Note              string `json:"note"`
		PageURL           string `json:"page_url"`
		TabID             int    `json:"tab_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "push_screenshot: invalid JSON body. Send a valid JSON object."})
		return
	}

	// Strip data URL prefix to get raw base64
	b64 := body.ScreenshotDataURL
	if idx := strings.Index(b64, ","); idx >= 0 {
		b64 = b64[idx+1:]
	}

	ev := push.PushEvent{
		ID:            pushEventID("push-ss"),
		Type:          "screenshot",
		Timestamp:     time.Now(),
		PageURL:       body.PageURL,
		TabID:         body.TabID,
		ScreenshotB64: b64,
		Note:          body.Note,
	}

	status := "queued"
	deliveryMethod := string(push.DeliveredViaInbox)
	if s.pushRouter != nil {
		result, err := s.pushRouter.DeliverPush(ev)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "push_screenshot: delivery failed. " + err.Error()})
			return
		}
		deliveryMethod = string(result.Method)
		if result.Method == push.DeliveredViaSampling {
			status = "delivered"
		}
	} else if s.pushInbox != nil {
		s.pushInbox.Enqueue(ev)
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"status":          status,
		"event_id":        ev.ID,
		"delivery_method": deliveryMethod,
	})
}

// handlePushMessage receives a chat message from the extension and routes it.
func (s *Server) handlePushMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "push_message: method not allowed. Use POST method."})
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		jsonResponse(w, http.StatusUnsupportedMediaType, map[string]string{"error": "push_message: unsupported content type. Set Content-Type to application/json."})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body struct {
		Message string `json:"message"`
		PageURL string `json:"page_url"`
		TabID   int    `json:"tab_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "push_message: invalid JSON body. Send a valid JSON object with a 'message' field."})
		return
	}

	if strings.TrimSpace(body.Message) == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "push_message: message field is empty. Provide a non-empty message."})
		return
	}

	ev := push.PushEvent{
		ID:        pushEventID("push-chat"),
		Type:      "chat",
		Timestamp: time.Now(),
		PageURL:   body.PageURL,
		TabID:     body.TabID,
		Message:   body.Message,
	}

	status := "queued"
	deliveryMethod := string(push.DeliveredViaInbox)
	if s.pushRouter != nil {
		result, err := s.pushRouter.DeliverPush(ev)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "push_message: delivery failed. " + err.Error()})
			return
		}
		deliveryMethod = string(result.Method)
		if result.Method == push.DeliveredViaSampling {
			status = "delivered"
		}
	} else if s.pushInbox != nil {
		s.pushInbox.Enqueue(ev)
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"status":          status,
		"event_id":        ev.ID,
		"delivery_method": deliveryMethod,
	})
}

// handlePushCapabilities returns per-session push capability flags for the extension.
func (s *Server) handlePushCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "push_capabilities: method not allowed. Use GET method."})
		return
	}

	caps := getPushClientCapabilities()
	inboxCount := 0
	if s.pushInbox != nil {
		inboxCount = s.pushInbox.Len()
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"push_enabled":           caps.SupportsSampling || caps.SupportsNotifications,
		"supports_sampling":      caps.SupportsSampling,
		"supports_notifications": caps.SupportsNotifications,
		"client_name":            caps.ClientName,
		"inbox_count":            inboxCount,
	})
}

// handlePushDrain drains the push inbox and returns events as JSON.
// Called by the bridge process to relay push events to Claude via stdio.
func (s *Server) handlePushDrain(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.pushInbox == nil {
		jsonResponse(w, http.StatusOK, map[string]any{"events": []any{}, "count": 0})
		return
	}
	events := s.pushInbox.DrainAll()
	if len(events) == 0 {
		jsonResponse(w, http.StatusOK, map[string]any{"events": []any{}, "count": 0})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"events": events, "count": len(events)})
}

// pushDrawModeCompletion builds an annotation PushEvent and routes it.
func (s *Server) pushDrawModeCompletion(body *drawModeRequest, screenshotPath string, annotations []Annotation) {
	if s.pushRouter == nil {
		return
	}

	annotJSON, err := json.Marshal(annotations)
	if err != nil {
		return
	}

	ev := push.PushEvent{
		ID:           pushEventID("push-ann"),
		Type:         "annotations",
		Timestamp:    time.Now(),
		PageURL:      body.PageURL,
		TabID:        body.TabID,
		Annotations:  annotJSON,
		AnnotSession: body.AnnotSessionName,
	}

	_, _ = s.pushRouter.DeliverPush(ev)
}
