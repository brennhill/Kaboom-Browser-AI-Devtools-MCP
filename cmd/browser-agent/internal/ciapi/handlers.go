// handlers.go — CI snapshot, clear, and test-boundary HTTP handlers.
// Docs: docs/features/feature/ci-infrastructure/index.md

package ciapi

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Logs is the bounded log-store surface used by CI endpoints.
type Logs interface {
	Entries() []types.LogEntry
	EntryCount() int
	ClearEntries()
}

// Capture is the snapshot/test-boundary surface used by CI endpoints.
type Capture interface {
	Extension() *capture.ExtensionRuntime
	Telemetry() *capture.TelemetryStore
}

// Resetter is the coordinated runtime reset surface used by CI endpoints.
type Resetter interface {
	ClearAll() int
}

// Snapshot returns the GET /snapshot handler.
func Snapshot(logs Logs, captured Capture) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			util.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}
		var sinceTime time.Time
		if since := r.URL.Query().Get("since"); since != "" {
			parsed, err := time.Parse(time.RFC3339Nano, since)
			if err != nil {
				util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid since timestamp"})
				return
			}
			sinceTime = parsed
		}

		entries := logs.Entries()
		if !sinceTime.IsZero() {
			entries = FilterLogsSince(entries, sinceTime)
		}
		wsEvents := captured.Telemetry().GetAllWebSocketEvents()
		networkBodies := captured.Telemetry().NetworkBodies().Snapshot().Bodies
		testID := r.URL.Query().Get("test_id")
		if testID == "" {
			activeIDs := captured.Extension().GetActiveTestIDs()
			if len(activeIDs) > 0 {
				testID = activeIDs[0]
			}
		}
		util.JSONResponse(w, http.StatusOK, SnapshotResponse{
			Timestamp:       time.Now().UTC().Format(time.RFC3339Nano),
			TestID:          testID,
			Logs:            entries,
			WebSocket:       wsEvents,
			NetworkBodies:   networkBodies,
			EnhancedActions: captured.Telemetry().Actions().Snapshot().Actions,
			Stats:           ComputeSnapshotStats(entries, wsEvents, networkBodies),
		})
	}
}

// Clear returns the POST/DELETE /clear handler.
func Clear(logs Logs, resetter Resetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			util.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}
		previousCount := logs.EntryCount()
		logs.ClearEntries()
		resetter.ClearAll()
		util.JSONResponse(w, http.StatusOK, map[string]any{"cleared": true, "entries_removed": previousCount})
	}
}

// TestBoundary returns the POST /test-boundary handler.
func TestBoundary(captured Capture) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			util.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1024))
		if err != nil {
			util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Failed to read body"})
			return
		}
		var req TestBoundaryRequest
		if err := json.Unmarshal(body, &req); err != nil {
			util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}
		if req.TestID == "" {
			util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "test_id is required"})
			return
		}
		if req.Action != "start" && req.Action != "end" {
			util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "action must be 'start' or 'end'"})
			return
		}
		if req.Action == "start" {
			captured.Extension().SetTestBoundaryStart(req.TestID)
		} else {
			captured.Extension().SetTestBoundaryEnd(req.TestID)
		}
		util.JSONResponse(w, http.StatusOK, map[string]any{
			"test_id": req.TestID, "action": req.Action, "timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
}
