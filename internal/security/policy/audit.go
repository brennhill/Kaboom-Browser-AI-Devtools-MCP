// audit.go — In-memory audit trail of security policy decisions and blocked mutations.
// Purpose: Maintains an in-memory audit log of security mode changes and blocked mutation attempts.
// Why: Separates security event auditing from mode management and policy configuration.
// Docs: docs/features/feature/security-hardening/index.md
package policy

import (
	"sync"
	"time"
)

var (
	// auditLog is intentionally in-memory only.
	// Rationale: this log records ephemeral session decisions and blocked mutation attempts,
	// and should not persist across restarts without explicit user opt-in.
	auditLog []AuditEvent
	auditMu  sync.Mutex
)

func LogEvent(event AuditEvent) {
	auditMu.Lock()
	defer auditMu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	auditLog = append(auditLog, event)
}

func AuditEvents() []AuditEvent {
	auditMu.Lock()
	defer auditMu.Unlock()

	events := make([]AuditEvent, len(auditLog))
	copy(events, auditLog)
	return events
}

func ClearAuditEvents() {
	auditMu.Lock()
	defer auditMu.Unlock()
	auditLog = nil
}
