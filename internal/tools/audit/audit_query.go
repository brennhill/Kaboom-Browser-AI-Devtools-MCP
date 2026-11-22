// Purpose: Queries audit trail entries with filtering by time, tool, client, and session.
// Why: Separates read-path query logic from write-path recording to keep the audit trail module focused.
package audit

import "encoding/json"

// Query returns audit entries matching the given filter.
// Results are returned in reverse chronological order (newest first).
// If no limit is specified, defaults to 100.
func (at *AuditTrail) Query(filter AuditFilter) []AuditEntry {
	at.mu.RLock()
	defer at.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultAuditQueryLimit
	}

	// Walk entries in reverse order (newest first)
	var results []AuditEntry
	for i := len(at.entries) - 1; i >= 0 && len(results) < limit; i-- {
		e := at.entries[i]

		if filter.AuditSessionID != "" && e.AuditSessionID != filter.AuditSessionID {
			continue
		}
		if filter.ToolName != "" && e.ToolName != filter.ToolName {
			continue
		}
		if filter.Since != nil && e.Timestamp.Before(*filter.Since) {
			continue
		}

		results = append(results, e)
	}

	return results
}

// Clear removes all audit entries, redaction events, and session state,
// returning the number of entries removed. Session counters are reset to
// prevent stale ToolCalls values from accumulating across clears.
func (at *AuditTrail) Clear() int {
	at.mu.Lock()
	defer at.mu.Unlock()

	cleared := len(at.entries)
	at.entries = at.entries[:0]
	at.redactions = at.redactions[:0]
	at.auditSessions = make(map[string]*AuditSessionInfo)
	return cleared
}

// QueryRedactions returns redaction events matching the given filter.
func (at *AuditTrail) QueryRedactions(filter AuditFilter) []RedactionEvent {
	at.mu.RLock()
	defer at.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultAuditQueryLimit
	}

	var results []RedactionEvent
	for i := len(at.redactions) - 1; i >= 0 && len(results) < limit; i-- {
		e := at.redactions[i]

		if filter.AuditSessionID != "" && e.AuditSessionID != filter.AuditSessionID {
			continue
		}
		if filter.ToolName != "" && e.ToolName != filter.ToolName {
			continue
		}

		results = append(results, e)
	}

	return results
}

// HandleGetAuditLog is the MCP tool handler for get_audit_log.
// It parses filter parameters and returns matching audit entries.
func (at *AuditTrail) HandleGetAuditLog(params json.RawMessage) (any, error) {
	var filter AuditFilter
	if len(params) > 0 && string(params) != "{}" {
		if err := json.Unmarshal(params, &filter); err != nil {
			return nil, err
		}
	}

	entries := at.Query(filter)

	type auditLogResponse struct {
		Entries []AuditEntry `json:"entries"`
		Count   int          `json:"count"`
	}

	return auditLogResponse{
		Entries: entries,
		Count:   len(entries),
	}, nil
}
