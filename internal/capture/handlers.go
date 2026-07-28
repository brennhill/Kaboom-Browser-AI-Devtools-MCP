// handlers.go — HTTP ingestion, request plumbing, and recording service delegation.
// Purpose: Owns capture's external request boundary and recording/storage operations.
// Why: recording.Recording HTTP handlers and their delegated service methods evolve together.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/query-service/index.md

package capture

import (
	"encoding/json"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/logdiff"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording/playback"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// HandleNetworkBodies handles POST /network-bodies from the extension.
// Reads go through GET /telemetry?type=network_bodies.
func (c *Capture) HandleNetworkBodies(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var payload struct {
		Bodies []types.NetworkBody `json:"bodies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleNetworkBodies: Invalid JSON - %v\n", err)
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	c.AddNetworkBodies(payload.Bodies)
	util.JSONResponse(w, http.StatusOK, map[string]any{
		"status": "ok",
		"count":  len(payload.Bodies),
	})
}

// HandleNetworkWaterfall handles POST /network-waterfall from the extension.
// Reads go through GET /telemetry?type=network_waterfall.
func (c *Capture) HandleNetworkWaterfall(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var payload types.NetworkWaterfallPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleNetworkWaterfall: Invalid JSON - %v\n", err)
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	c.NetworkWaterfall().Add(payload.Entries, payload.PageURL)
	util.JSONResponse(w, http.StatusOK, map[string]any{
		"status": "ok",
		"count":  len(payload.Entries),
	})
}

// HandleQueryResult processes all query/command results from the extension.
// Unified handler replacing separate dom-result, a11y-result, state-result,
// execute-result, and highlight-result endpoints.
func (c *Capture) HandleQueryResult(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
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

	// Handle query_id for synchronous query results
	if body.ID != "" {
		if body.CorrelationID != "" {
			// Correlated async commands carry explicit lifecycle status below.
			// Do not force "complete" from query-id bookkeeping.
			c.SetQueryResultWithClientNoCommandComplete(body.ID, body.Result, body.ClientID)
		} else {
			c.SetQueryResultWithClient(body.ID, body.Result, body.ClientID)
		}
	}

	// Handle correlation_id for async commands (execute_js, browser actions)
	if body.CorrelationID != "" {
		c.ApplyCommandResult(body.CorrelationID, body.Status, body.Result, body.Error)
	}

	util.JSONResponse(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

// HandleEnhancedActions handles POST /enhanced-actions from the extension.
// Reads go through GET /telemetry?type=actions.
func (c *Capture) HandleEnhancedActions(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var payload struct {
		Actions []types.EnhancedAction `json:"actions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleEnhancedActions: Invalid JSON - %v\n", err)
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	c.AddEnhancedActions(payload.Actions)
	util.JSONResponse(w, http.StatusOK, map[string]any{
		"status": "ok",
		"count":  len(payload.Actions),
	})
}

// HandleRecordingStorage handles recording storage management.
// GET: returns storage info
// DELETE: deletes a recording (requires recording_id query param)
// POST: recalculates storage usage
func (c *Capture) HandleRecordingStorage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		c.handleStorageGet(w)
	case "DELETE":
		c.handleStorageDelete(w, r)
	case "POST":
		c.handleStorageRecalculate(w)
	default:
		util.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func (c *Capture) handleStorageGet(w http.ResponseWriter) {
	info, err := c.GetStorageInfo()
	if err != nil {
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	util.JSONResponse(w, http.StatusOK, info)
}

func (c *Capture) handleStorageDelete(w http.ResponseWriter, r *http.Request) {
	recordingID := r.URL.Query().Get("recording_id")
	if recordingID == "" {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleRecordingStorage: Missing recording_id query parameter\n")
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing recording_id query parameter"})
		return
	}
	if err := c.DeleteRecording(recordingID); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleRecordingStorage: Failed to delete recording %s - %v\n", recordingID, err)
		util.JSONResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	util.JSONResponse(w, http.StatusOK, map[string]any{"status": "ok", "deleted": recordingID})
}

func (c *Capture) handleStorageRecalculate(w http.ResponseWriter) {
	if err := c.RecalculateStorageUsed(); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleRecordingStorage: Failed to recalculate storage - %v\n", err)
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	info, err := c.GetStorageInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] HandleRecordingStorage: Failed to get storage info - %v\n", err)
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	util.JSONResponse(w, http.StatusOK, map[string]any{"status": "ok", "storage": info})
}

// HandlePerformanceSnapshots handles POST /performance-snapshots from the extension.
// Reads go through GET /telemetry?type=performance_snapshots.
func (c *Capture) HandlePerformanceSnapshots(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
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
	c.AddPerformanceSnapshots(payload.Snapshots)
	util.JSONResponse(w, http.StatusOK, map[string]any{
		"status": "ok",
		"count":  len(payload.Snapshots),
	})
}

func ExtractURLPath(rawURL string) string {
	return util.ExtractURLPath(rawURL)
}

func (c *Capture) readIngestBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if c.CheckRateLimit() {
		c.WriteRateLimitResponse(w)
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

func (c *Capture) recordAndRecheck(w http.ResponseWriter, count int) bool {
	c.RecordEvents(count)
	if c.CheckRateLimit() {
		c.WriteRateLimitResponse(w)
		return false
	}
	return true
}

func (c *Capture) RecordEvents(count int) {
	c.circuit.RecordEvents(count)
}

func (c *Capture) CheckRateLimit() bool {
	return c.circuit.CheckRateLimit()
}

func (c *Capture) GetHealthStatus() circuit.HealthResponse {
	return c.circuit.GetHealthStatus()
}

func (c *Capture) WriteRateLimitResponse(w http.ResponseWriter) {
	c.circuit.WriteRateLimitResponse(w)
}

func (c *Capture) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		util.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	health := c.GetHealthStatus()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(health)
}

func isExpiredByTTL(addedAt time.Time, ttl time.Duration) bool {
	if ttl == 0 {
		return false
	}
	return time.Since(addedAt) >= ttl
}

var NewRecordingManager = recording.NewRecordingManager

func (c *Capture) StartRecording(name, pageURL string, sensitiveDataEnabled bool) (string, error) {
	return c.recordingManager.StartRecording(name, pageURL, sensitiveDataEnabled)
}

func (c *Capture) StopRecording(recordingID string) (int, int64, error) {
	return c.recordingManager.StopRecording(recordingID)
}

func (c *Capture) AddRecordingAction(action recording.RecordingAction) error {
	return c.recordingManager.AddRecordingAction(action)
}

func (c *Capture) ListRecordings(limit int) ([]recording.Recording, error) {
	return c.recordingManager.ListRecordings(limit)
}

func (c *Capture) GetRecording(recordingID string) (*recording.Recording, error) {
	return c.recordingManager.GetRecording(recordingID)
}

func (c *Capture) StartPlayback(recordingID string) (*playback.Session, error) {
	return playback.Start(c.recordingManager, recordingID)
}

func (c *Capture) ExecutePlayback(recordingID string) (*playback.Session, error) {
	return playback.Execute(c.recordingManager, recordingID)
}

func (c *Capture) DetectFragileSelectors(sessions []*playback.Session) map[string]bool {
	return playback.DetectFragileSelectors(sessions)
}

func (c *Capture) GetPlaybackStatus(session *playback.Session) map[string]any {
	return playback.Status(session)
}

func (c *Capture) DiffRecordings(originalID, replayID string) (*logdiff.Result, error) {
	return logdiff.Compare(c.recordingManager, originalID, replayID)
}

func (c *Capture) CategorizeActionTypes(recordingItem *recording.Recording) map[string]int {
	return logdiff.CategorizeActionTypes(recordingItem)
}

func (c *Capture) GetStorageInfo() (recording.StorageInfo, error) {
	return c.recordingManager.GetStorageInfo()
}

func (c *Capture) DeleteRecording(recordingID string) error {
	return c.recordingManager.DeleteRecording(recordingID)
}

func (c *Capture) RecalculateStorageUsed() error {
	return c.recordingManager.RecalculateStorageUsed()
}
