// Purpose: Defines types for audit entries, sessions, filters, configs, and redaction patterns.
// Why: Centralizes audit type definitions so recording, query, and redaction modules share a single source of truth.
package audit

import (
	"regexp"
	"sync"
	"time"
)

// AuditEntry represents a single tool invocation audit record.
type AuditEntry struct {
	ID             string    `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	AuditSessionID string    `json:"audit_session_id"`
	ClientID       string    `json:"client_id"`
	ToolName       string    `json:"tool_name"`
	Parameters     string    `json:"parameters"`
	ResponseSize   int       `json:"response_size"`
	Duration       int64     `json:"duration_ms"`
	Success        bool      `json:"success"`
	ErrorMessage   string    `json:"error_message,omitempty"`
}

// AuditTrail is an append-only, bounded, concurrent-safe audit log.
type AuditTrail struct {
	mu                sync.RWMutex
	entries           []AuditEntry
	maxSize           int
	auditSessions     map[string]*AuditSessionInfo
	config            AuditConfig
	redactions        []RedactionEvent
	redactionPatterns []*redactionPattern
	now               func() time.Time
}

// SessionInfo tracks metadata for an MCP session.
type AuditSessionInfo struct {
	ID        string    `json:"id"`
	ClientID  string    `json:"client_id"`
	StartedAt time.Time `json:"started_at"`
	ToolCalls int       `json:"tool_calls"`
}

// AuditFilter specifies query criteria for audit log entries.
type AuditFilter struct {
	AuditSessionID string     `json:"audit_session_id,omitempty"`
	ToolName       string     `json:"tool_name,omitempty"`
	Since          *time.Time `json:"since,omitempty"`
	Limit          int        `json:"limit,omitempty"`
}

// AuditConfig controls audit trail behavior.
type AuditConfig struct {
	MaxEntries   int  `json:"max_entries"`
	Enabled      bool `json:"enabled"`
	RedactParams bool `json:"redact_params"`
}

// ClientIdentifier holds MCP client identification from the initialize message.
type ClientIdentifier struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// RedactionEvent records that a redaction pattern matched, without storing content.
type RedactionEvent struct {
	Timestamp      time.Time `json:"timestamp"`
	AuditSessionID string    `json:"audit_session_id"`
	ToolName       string    `json:"tool_name"`
	FieldPath      string    `json:"field_path"`
	PatternName    string    `json:"pattern_name"`
}

// redactionPattern is an internal compiled regex for parameter redaction.
type redactionPattern struct {
	name    string
	pattern *regexp.Regexp
}

const (
	defaultAuditMaxEntries = 10000
	defaultAuditQueryLimit = 100
)
