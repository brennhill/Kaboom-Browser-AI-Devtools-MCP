// intent_handlers.go -- HTTP handlers for QA intent creation and terminal injection.
// Why: Bridges the extension's "Find Problems" button to the AI via PTY or intent fallback.
// Docs: docs/features/feature/auto-fix/index.md

package intent

import (
	"encoding/json"
	"net/http"
)

// RelayMap is the injection capability consumed by intent routes.
type RelayMap interface {
	WriteToFirst([]byte) bool
	CloseAll()
}

// RuntimeDeps provides the live terminal relay and intent store.
type RuntimeDeps interface {
	GetPtyRelays() RelayMap
	GetIntentStore() *Store
}

// HTTPDeps owns the HTTP response and request-boundary collaborators.
type HTTPDeps struct {
	JSONResponse   func(http.ResponseWriter, int, any)
	CORSMiddleware func(http.HandlerFunc) http.HandlerFunc
	MaxPostBody    int64
}

// IntentRequest is the JSON body for intent creation.
type IntentRequest struct {
	PageURL string `json:"page_url"`
	Action  string `json:"action"`
}

// RegisterIntentRoutes adds intent-related routes to the terminal mux.
func RegisterRoutes(mux *http.ServeMux, deps HTTPDeps, runtime RuntimeDeps) {
	// Inject text directly into the active PTY session.
	mux.HandleFunc("/terminal/inject", deps.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleTerminalInject(w, r, deps, runtime)
	}))

	// Store an intent for the AI to pick up via MCP tool responses.
	mux.HandleFunc("/intent", deps.CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleCreate(w, r, deps, runtime)
	}))
}

// HandleTerminalInject writes text into the first active PTY session.
func handleTerminalInject(w http.ResponseWriter, r *http.Request, deps HTTPDeps, runtime RuntimeDeps) {
	if r.Method != "POST" {
		deps.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	// Cap the body like every other terminal handler, so an oversized payload is
	// rejected instead of fully buffered into memory (finding G).
	r.Body = http.MaxBytesReader(w, r.Body, deps.MaxPostBody)

	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
		deps.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "missing text field"})
		return
	}

	relays := runtime.GetPtyRelays()
	if relays == nil {
		deps.JSONResponse(w, http.StatusServiceUnavailable, map[string]any{
			"injected": false,
			"reason":   "no_terminal_server",
		})
		return
	}

	ok := relays.WriteToFirst([]byte(body.Text + "\n"))
	if !ok {
		deps.JSONResponse(w, http.StatusServiceUnavailable, map[string]any{
			"injected": false,
			"reason":   "no_active_session",
		})
		return
	}

	deps.JSONResponse(w, http.StatusOK, map[string]any{"injected": true})
}

// HandleIntentCreate creates an intent for the AI to pick up.
func handleCreate(w http.ResponseWriter, r *http.Request, deps HTTPDeps, runtime RuntimeDeps) {
	if r.Method != "POST" {
		deps.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	// Cap the body like every other terminal handler, so an oversized payload is
	// rejected instead of fully buffered into memory (finding G).
	r.Body = http.MaxBytesReader(w, r.Body, deps.MaxPostBody)

	var req IntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deps.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Action == "" {
		req.Action = ActionQAScan
	}

	store := runtime.GetIntentStore()
	if store == nil {
		deps.JSONResponse(w, http.StatusServiceUnavailable, map[string]string{"error": "intent store not initialized"})
		return
	}

	id := store.Add(req.PageURL, req.Action)
	deps.JSONResponse(w, http.StatusOK, map[string]any{
		"correlation_id": id,
		"stored":         true,
	})
}
