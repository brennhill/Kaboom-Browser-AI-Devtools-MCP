// Purpose: Manages per-client audit sessions including client identification and session lifecycle.
// Why: Separates session tracking from entry recording to keep client normalization self-contained.
package audit

import "strings"

// IdentifyClient normalizes the client name from the MCP initialize message.
// Known clients are lowercased; unknown names are preserved as-is.
func (at *AuditTrail) IdentifyClient(client ClientIdentifier) string {
	if client.Name == "" {
		return "unknown"
	}

	lower := strings.ToLower(client.Name)

	// Known clients — normalize to lowercase
	switch lower {
	case "claude-code", "cursor", "windsurf", "cline":
		return lower
	}

	// Unknown client — preserve raw name
	return client.Name
}

// CreateSession generates a new unique session and registers it.
// The session ID is a 16-byte random value, hex-encoded (32 chars).
func (at *AuditTrail) CreateAuditSession(client ClientIdentifier) *AuditSessionInfo {
	at.mu.Lock()
	defer at.mu.Unlock()

	sessionID := generateAuditSessionID()
	clientID := at.identifyClientUnlocked(client)

	info := &AuditSessionInfo{
		ID:        sessionID,
		ClientID:  clientID,
		StartedAt: at.now(),
		ToolCalls: 0,
	}

	at.auditSessions[sessionID] = info
	return info
}

// GetSession returns session info for the given ID, or nil if not found.
func (at *AuditTrail) GetAuditSession(id string) *AuditSessionInfo {
	at.mu.RLock()
	defer at.mu.RUnlock()

	return at.auditSessions[id]
}

// identifyClientUnlocked is the internal version without locking.
func (at *AuditTrail) identifyClientUnlocked(client ClientIdentifier) string {
	if client.Name == "" {
		return "unknown"
	}

	lower := strings.ToLower(client.Name)
	switch lower {
	case "claude-code", "cursor", "windsurf", "cline":
		return lower
	}

	return client.Name
}
