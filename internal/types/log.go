// Purpose: Defines canonical server and extension log entry structures used across ingestion and diagnostics.
// Why: Normalizes logging payloads so filtering, redaction, and export logic can rely on stable fields.
// Docs: docs/features/feature/normalized-log-schema/index.md

package types

import "time"

// ============================================
// Server Logging
// ============================================

// LogEntry represents a generic log entry from the server.
// Implemented as a map to allow flexible field addition without schema changes.
// any: Console log entries have dynamic fields (level, message, source, tabId, stack, etc.)
// that vary by log type. A typed struct would require many optional fields.
type LogEntry map[string]any

// ============================================
// Polling Activity Log
// ============================================

// PollingLogEntry tracks a single polling request (GET /pending-queries or POST /settings)
type PollingLogEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	Endpoint     string    `json:"endpoint"` // "pending-queries" or "settings"
	Method       string    `json:"method"`   // "GET" or "POST"
	ExtSessionID string    `json:"ext_session_id,omitempty"`
	PilotEnabled *bool     `json:"pilot_enabled,omitempty"` // Only for POST /settings
	PilotHeader  string    `json:"pilot_header,omitempty"`  // Only for GET with X-Kaboom-Pilot header
	QueryCount   int       `json:"query_count,omitempty"`   // Number of pending queries returned
}

// ============================================
// HTTP Debug Log
// ============================================

// HTTPDebugEntry tracks detailed request/response data for debugging
type HTTPDebugEntry struct {
	Timestamp      time.Time         `json:"timestamp"`
	Endpoint       string            `json:"endpoint"` // URL path
	Method         string            `json:"method"`   // HTTP method
	ExtSessionID   string            `json:"ext_session_id,omitempty"`
	ClientID       string            `json:"client_id,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`         // Request headers (redacted auth)
	RequestBody    string            `json:"request_body,omitempty"`    // First 1KB of request body
	ResponseStatus int               `json:"response_status,omitempty"` // HTTP status code
	ResponseBody   string            `json:"response_body,omitempty"`   // First 1KB of response body
	DurationMs     int64             `json:"duration_ms"`               // Request processing duration
	Error          string            `json:"error,omitempty"`           // Error message if any
}
