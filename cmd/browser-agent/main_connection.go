// Purpose: Orchestrates daemon discovery, spawn, health-check, and version-mismatch handling for bridge client connections.
// Why: Handles the complex startup handshake where a bridge client must find or launch a compatible daemon.

package main

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

type healthMetadata struct {
	Version     string `json:"version"`
	Service     string `json:"service"`
	ServiceName string `json:"service-name"`
	Name        string `json:"name"`
}

func decodeHealthMetadata(body []byte) (healthMetadata, bool) {
	var meta healthMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return healthMetadata{}, false
	}
	return meta, true
}

func (m healthMetadata) resolvedServiceName() string {
	if strings.TrimSpace(m.ServiceName) != "" {
		return strings.TrimSpace(m.ServiceName)
	}
	if strings.TrimSpace(m.Service) != "" {
		return strings.TrimSpace(m.Service)
	}
	return strings.TrimSpace(m.Name)
}

func normalizeVersionString(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func versionsMatch(a string, b string) bool {
	return normalizeVersionString(a) == normalizeVersionString(b)
}

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
