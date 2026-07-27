// ci.go — Main-package adapters for the CI snapshot HTTP subsystem.
// Docs: docs/features/feature/ci-infrastructure/index.md

package main

import (
	"net/http"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/ciapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

type SnapshotResponse = ciapi.SnapshotResponse
type SnapshotStats = ciapi.SnapshotStats
type TestBoundaryRequest = ciapi.TestBoundaryRequest

func handleSnapshot(server *Server, captured *capture.Store) http.HandlerFunc {
	return ciapi.Snapshot(server.logs, captured)
}

func handleClear(server *Server, captured *capture.Store) http.HandlerFunc {
	return ciapi.Clear(server.logs, captured)
}

func handleTestBoundary(captured *capture.Store) http.HandlerFunc {
	return ciapi.TestBoundary(captured)
}

func filterLogsSince(logs []LogEntry, since time.Time) []LogEntry {
	return ciapi.FilterLogsSince(logs, since)
}

func computeSnapshotStats(logs []LogEntry, wsEvents []capture.WebSocketEvent, networkBodies []capture.NetworkBody) SnapshotStats {
	return ciapi.ComputeSnapshotStats(logs, wsEvents, networkBodies)
}
