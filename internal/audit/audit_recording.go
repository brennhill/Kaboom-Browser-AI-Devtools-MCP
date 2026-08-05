// Purpose: Creates and configures the AuditTrail with defaults and records tool invocation entries.
// Why: Separates construction and entry recording from query and redaction concerns.
package audit

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

func generateAuditID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes) // #nosec G104 -- best-effort uniqueness for a non-security audit ID
	return hex.EncodeToString(bytes)
}

func generateAuditSessionID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes) // #nosec G104 -- best-effort uniqueness for a non-security session ID
	return hex.EncodeToString(bytes)
}

// NewAuditTrail creates a new audit trail with the given configuration.
// Zero-value AuditConfig fields are replaced with sensible defaults.
func NewAuditTrail(config AuditConfig) *AuditTrail {
	if config.MaxEntries <= 0 {
		config.MaxEntries = defaultAuditMaxEntries
	}
	// Default: enabled and redact params when config is zero-value
	if !config.Enabled && config.MaxEntries == defaultAuditMaxEntries && !config.RedactParams {
		config.Enabled = true
		config.RedactParams = true
	}

	trail := &AuditTrail{
		entries:       make([]AuditEntry, 0, config.MaxEntries),
		maxSize:       config.MaxEntries,
		auditSessions: make(map[string]*AuditSessionInfo),
		config:        config,
		redactions:    make([]RedactionEvent, 0),
		now:           time.Now,
	}

	if config.RedactParams {
		trail.redactionPatterns = compileRedactionPatterns()
	}

	return trail
}

// Record appends an audit entry to the log. If the trail is disabled, the
// entry is silently dropped. If the buffer is full, the oldest entry is evicted.
func (at *AuditTrail) Record(entry AuditEntry) {
	if !at.config.Enabled {
		return
	}

	at.mu.Lock()
	defer at.mu.Unlock()

	// Assign ID and timestamp
	entry.ID = generateAuditID()
	entry.Timestamp = at.now()

	// Redact parameters if configured
	if at.config.RedactParams && entry.Parameters != "" {
		entry.Parameters = at.redactParameters(entry.Parameters)
	}

	// FIFO eviction when full
	if len(at.entries) >= at.maxSize {
		newEntries := make([]AuditEntry, len(at.entries)-1)
		copy(newEntries, at.entries[1:])
		at.entries = newEntries
	}

	at.entries = append(at.entries, entry)

	// Update session tool call count
	if sess, ok := at.auditSessions[entry.AuditSessionID]; ok {
		sess.ToolCalls++
	}
}

// RecordRedaction logs a redaction event without storing any redacted content.
func (at *AuditTrail) RecordRedaction(event RedactionEvent) {
	if !at.config.Enabled {
		return
	}

	at.mu.Lock()
	defer at.mu.Unlock()

	// Bounded: use same max size for redaction events
	if len(at.redactions) >= at.maxSize {
		newRedactions := make([]RedactionEvent, len(at.redactions)-1)
		copy(newRedactions, at.redactions[1:])
		at.redactions = newRedactions
	}

	at.redactions = append(at.redactions, event)
}
