// Purpose: Implements snapshot/test-boundary and CI webhook endpoints for capture-driven verification workflows.
// Why: Bridges CI signals and runtime snapshots into a single API surface for regression tooling.
// Docs: docs/features/feature/ci-infrastructure/index.md

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// ============================================
// Handlers
// ============================================

// handleSnapshot returns an HTTP handler for GET /snapshot.
// Returns all captured state in a single response.
// #lizard forgives
func handleSnapshot(server *Server, cap *capture.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		since := r.URL.Query().Get("since")
		testID := r.URL.Query().Get("test_id")

		var sinceTime time.Time
		if since != "" {
			var err error
			sinceTime, err = time.Parse(time.RFC3339Nano, since)
			if err != nil {
				jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid since timestamp"})
				return
			}
		}

		// Gather logs
		logs := server.logs.Entries()
		if !sinceTime.IsZero() {
			logs = filterLogsSince(logs, sinceTime)
		}

		// Gather capture data
		wsEvents := cap.GetAllWebSocketEvents()
		networkBodies := cap.GetNetworkBodies()
		enhancedActions := cap.GetAllEnhancedActions()

		// Apply test_id label (use first active test ID if not specified).
		// Note: test_id is currently for labeling only. Full per-entry filtering
		// would require storing test boundary timestamps alongside buffer entries.
		if testID == "" {
			activeIDs := cap.GetActiveTestIDs()
			if len(activeIDs) > 0 {
				testID = activeIDs[0]
			}
		}

		stats := computeSnapshotStats(logs, wsEvents, networkBodies)

		snapshot := SnapshotResponse{
			Timestamp:       time.Now().UTC().Format(time.RFC3339Nano),
			TestID:          testID,
			Logs:            logs,
			WebSocket:       wsEvents,
			NetworkBodies:   networkBodies,
			EnhancedActions: enhancedActions,
			Stats:           stats,
		}

		jsonResponse(w, http.StatusOK, snapshot)
	}
}

// handleClear returns an HTTP handler for POST/DELETE /clear.
// Resets all buffers atomically.
func handleClear(server *Server, cap *capture.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "DELETE" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		previousCount := server.logs.EntryCount()
		server.logs.ClearEntries()
		cap.ClearAll()

		jsonResponse(w, http.StatusOK, map[string]any{
			"cleared":         true,
			"entries_removed": previousCount,
		})
	}
}

// handleTestBoundary returns an HTTP handler for POST /test-boundary.
// Marks test boundaries for correlation.
func handleTestBoundary(cap *capture.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1024))
		if err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Failed to read body"})
			return
		}

		var req TestBoundaryRequest
		if err := json.Unmarshal(body, &req); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}

		if req.TestID == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "test_id is required"})
			return
		}
		if req.Action != "start" && req.Action != "end" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "action must be 'start' or 'end'"})
			return
		}

		now := time.Now().UTC()

		if req.Action == "start" {
			cap.SetTestBoundaryStart(req.TestID)
		} else {
			cap.SetTestBoundaryEnd(req.TestID)
		}

		jsonResponse(w, http.StatusOK, map[string]any{
			"test_id":   req.TestID,
			"action":    req.Action,
			"timestamp": now.Format(time.RFC3339Nano),
		})
	}
}
