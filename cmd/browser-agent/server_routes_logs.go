// Purpose: Implements /logs ingest and clear endpoints.
// Why: Keeps log ingestion behavior independent from broader route registration concerns.

package main

import (
	"encoding/json"
	"net/http"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"
)

// handleLogs serves the /logs endpoint for ingesting and clearing log entries.
// Reads go through GET /telemetry?type=logs.
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		s.handleLogsPost(w, r)
	case "DELETE":
		s.logs.ClearEntries()
		jsonResponse(w, http.StatusOK, map[string]bool{"cleared": true})
	default:
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// handleLogsPost processes POST /logs requests to ingest new log entries.
func (s *Server) handleLogsPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body struct {
		Entries []LogEntry `json:"entries"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.Entries == nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing entries array"})
		return
	}

	valid, rejected := logstore.ValidateEntries(body.Entries)
	received := s.logs.AddEntries(valid)
	jsonResponse(w, http.StatusOK, map[string]int{
		"received": received,
		"rejected": rejected,
		"entries":  s.logs.EntryCount(),
	})
}
