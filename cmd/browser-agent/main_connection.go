// Purpose: Orchestrates daemon discovery, spawn, health-check, and version-mismatch handling for bridge client connections.
// Why: Handles the complex startup handshake where a bridge client must find or launch a compatible daemon.

package main

import (
	"os"
	"time"
)

// logLifecycle is a convenience method to emit a structured lifecycle log entry.
func (s *Server) logLifecycle(event string, port int, extra map[string]any) {
	entry := LogEntry{
		"type":      "lifecycle",
		"event":     event,
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if port != 0 {
		entry["port"] = port
	}
	for k, v := range extra {
		entry[k] = v
	}
	s.logs.AddEntries([]LogEntry{entry})
}
