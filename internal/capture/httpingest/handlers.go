// handlers.go — Owns capture HTTP protocol validation and response writing.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/query-service/index.md

package httpingest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/perfstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/telemetrystore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const maxExtensionPostBody = 5 << 20

// Dependencies are the canonical state owners used by HTTP ingestion.
type Dependencies struct {
	Telemetry   *telemetrystore.Store
	Queries     *queries.QueryDispatcher
	Recordings  *recording.RecordingManager
	Performance *perfstore.Store
	Circuit     *circuit.CircuitBreaker
}

// Handlers owns capture's HTTP request boundary.
type Handlers struct{ deps Dependencies }

// New binds the HTTP boundary to explicit canonical owners.
func New(deps Dependencies) *Handlers { return &Handlers{deps: deps} }

// decodeIngestJSON enforces the POST method, bounds the body, and decodes the
// JSON payload, writing the standard error responses. It reports decode success.
func (h *Handlers) decodeIngestJSON(w http.ResponseWriter, r *http.Request, handlerName string, payload any) bool {
	if !util.RequireMethod(w, r, http.MethodPost) {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	if err := json.NewDecoder(r.Body).Decode(payload); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] %s: Invalid JSON - %v\n", handlerName, err)
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return false
	}
	return true
}

//nolint:dupl // decode and error paths are shared via decodeIngestJSON; the residual wrapper differs only in payload field and telemetry sink, which cannot be unified without dynamic JSON tags (a behavior change).
func (h *Handlers) HandleNetworkBodies(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Bodies []types.NetworkBody `json:"bodies"`
	}
	if !h.decodeIngestJSON(w, r, "HandleNetworkBodies", &payload) {
		return
	}
	h.deps.Telemetry.AddNetworkBodies(payload.Bodies)
	util.JSONResponse(w, http.StatusOK, map[string]any{"status": "ok", "count": len(payload.Bodies)})
}

func (h *Handlers) HandleNetworkWaterfall(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, http.MethodPost) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var payload types.NetworkWaterfallPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleNetworkWaterfall: Invalid JSON - %v\n", err)
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	h.deps.Telemetry.NetworkWaterfall().Add(payload.Entries, payload.PageURL)
	util.JSONResponse(w, http.StatusOK, map[string]any{"status": "ok", "count": len(payload.Entries)})
}

func (h *Handlers) HandleQueryResult(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, http.MethodPost) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var body struct {
		ID            string          `json:"id"`
		CorrelationID string          `json:"correlation_id"`
		Status        string          `json:"status"`
		Result        json.RawMessage `json:"result"`
		Error         string          `json:"error"`
		ClientID      string          `json:"client_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleQueryResult: Invalid JSON - %v\n", err)
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.ID != "" {
		if body.CorrelationID != "" {
			h.deps.Queries.SetQueryResultWithClientNoCommandComplete(body.ID, body.Result, body.ClientID)
		} else {
			h.deps.Queries.SetQueryResultWithClient(body.ID, body.Result, body.ClientID)
		}
	}
	if body.CorrelationID != "" {
		h.deps.Queries.ApplyCommandResult(body.CorrelationID, body.Status, body.Result, body.Error)
	}
	util.JSONResponse(w, http.StatusOK, map[string]any{"status": "ok"})
}

//nolint:dupl // decode and error paths are shared via decodeIngestJSON; the residual wrapper differs only in payload field and telemetry sink, which cannot be unified without dynamic JSON tags (a behavior change).
func (h *Handlers) HandleEnhancedActions(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Actions []types.EnhancedAction `json:"actions"`
	}
	if !h.decodeIngestJSON(w, r, "HandleEnhancedActions", &payload) {
		return
	}
	h.deps.Telemetry.AddEnhancedActions(payload.Actions)
	util.JSONResponse(w, http.StatusOK, map[string]any{"status": "ok", "count": len(payload.Actions)})
}

func (h *Handlers) HandleWebSocketEvents(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, http.MethodPost) {
		return
	}
	body, ok := h.readIngestBody(w, r)
	if !ok {
		return
	}
	var payload struct {
		Events []types.WebSocketEvent `json:"events"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if !h.recordAndRecheck(w, len(payload.Events)) {
		return
	}
	h.deps.Telemetry.AddWebSocketEvents(payload.Events)
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) HandleWebSocketStatus(w http.ResponseWriter, _ *http.Request) {
	util.JSONResponse(w, http.StatusOK, h.deps.Telemetry.WebSockets().Status(types.WebSocketStatusFilter{}))
}

func (h *Handlers) HandleRecordingStorage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleStorageGet(w)
	case http.MethodDelete:
		h.handleStorageDelete(w, r)
	case http.MethodPost:
		h.handleStorageRecalculate(w)
	default:
		util.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func (h *Handlers) handleStorageGet(w http.ResponseWriter) {
	info, err := h.deps.Recordings.GetStorageInfo()
	if err != nil {
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	util.JSONResponse(w, http.StatusOK, info)
}

func (h *Handlers) handleStorageDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("recording_id")
	if id == "" {
		fmt.Fprintln(os.Stderr, "[Kaboom] HandleRecordingStorage: Missing recording_id query parameter")
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing recording_id query parameter"})
		return
	}
	if err := h.deps.Recordings.DeleteRecording(id); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleRecordingStorage: Failed to delete recording %s - %v\n", id, err)
		util.JSONResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	util.JSONResponse(w, http.StatusOK, map[string]any{"status": "ok", "deleted": id})
}

func (h *Handlers) handleStorageRecalculate(w http.ResponseWriter) {
	if err := h.deps.Recordings.RecalculateStorageUsed(); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleRecordingStorage: Failed to recalculate storage - %v\n", err)
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	info, err := h.deps.Recordings.GetStorageInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleRecordingStorage: Failed to get storage info - %v\n", err)
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	util.JSONResponse(w, http.StatusOK, map[string]any{"status": "ok", "storage": info})
}

func (h *Handlers) HandlePerformanceSnapshots(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, http.MethodPost) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var payload struct {
		Snapshots []performance.PerformanceSnapshot `json:"snapshots"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	h.deps.Performance.Add(payload.Snapshots)
	util.JSONResponse(w, http.StatusOK, map[string]any{"status": "ok", "count": len(payload.Snapshots)})
}

func (h *Handlers) readIngestBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if h.deps.Circuit.CheckRateLimit() {
		h.deps.Circuit.WriteRateLimitResponse(w)
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return nil, false
	}
	return body, true
}

func (h *Handlers) recordAndRecheck(w http.ResponseWriter, count int) bool {
	h.deps.Circuit.RecordEvents(count)
	if h.deps.Circuit.CheckRateLimit() {
		h.deps.Circuit.WriteRateLimitResponse(w)
		return false
	}
	return true
}

func (h *Handlers) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		util.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(h.deps.Circuit.GetHealthStatus())
}
