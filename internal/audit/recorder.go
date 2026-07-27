// recorder.go — Records MCP tool calls into per-client audit sessions.

package audit

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// Recorder owns tool-call audit policy and per-client session state.
type Recorder struct {
	trail    *AuditTrail
	mu       sync.Mutex
	sessions map[string]string
}

// NewRecorder creates a recorder for an audit trail.
func NewRecorder(trail *AuditTrail) *Recorder {
	return &Recorder{trail: trail, sessions: make(map[string]string)}
}

// Record adds one completed tool call unless the call is the audit clear operation.
func (recorder *Recorder) Record(req mcp.JSONRPCRequest, toolName string, args json.RawMessage, response mcp.JSONRPCResponse, started time.Time) {
	if recorder == nil || recorder.trail == nil || shouldSkipToolCall(toolName, args) {
		return
	}
	sessionID := recorder.sessionForClient(req.ClientID)
	if sessionID == "" {
		return
	}
	success := response.Error == nil && !toolResultIsError(response.Result)
	entry := AuditEntry{
		AuditSessionID: sessionID,
		ClientID:       normalizeClientID(req.ClientID),
		ToolName:       toolName,
		Parameters:     string(args),
		ResponseSize:   len(response.Result),
		Duration:       time.Since(started).Milliseconds(),
		Success:        success,
	}
	if !success {
		entry.ErrorMessage = responseErrorMessage(response)
	}
	recorder.trail.Record(entry)
}

// ResetSessions discards cached session IDs after the trail is cleared.
func (recorder *Recorder) ResetSessions() {
	if recorder == nil {
		return
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.sessions = make(map[string]string)
}

// SessionID returns the currently cached audit session for a client.
func (recorder *Recorder) SessionID(clientID string) string {
	if recorder == nil {
		return ""
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return recorder.sessions[normalizeClientID(clientID)]
}

func (recorder *Recorder) sessionForClient(clientID string) string {
	client := normalizeClientID(clientID)
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if sessionID := recorder.sessions[client]; sessionID != "" {
		if recorder.trail.GetAuditSession(sessionID) != nil {
			return sessionID
		}
		delete(recorder.sessions, client)
	}
	info := recorder.trail.CreateAuditSession(ClientIdentifier{Name: client})
	if info == nil || info.ID == "" {
		return ""
	}
	recorder.sessions[client] = info.ID
	return info.ID
}

func normalizeClientID(clientID string) string {
	if trimmed := strings.TrimSpace(clientID); trimmed != "" {
		return trimmed
	}
	return "unknown"
}

func responseErrorMessage(response mcp.JSONRPCResponse) string {
	if response.Error != nil && response.Error.Message != "" {
		return response.Error.Message
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil || len(result.Content) == 0 {
		return ""
	}
	return result.Content[0].Text
}

func toolResultIsError(raw json.RawMessage) bool {
	var result mcp.MCPToolResult
	return json.Unmarshal(raw, &result) == nil && result.IsError
}

func shouldSkipToolCall(toolName string, args json.RawMessage) bool {
	if toolName != "configure" || len(args) == 0 {
		return false
	}
	var params struct {
		What      string `json:"what"`
		Action    string `json:"action"`
		Operation string `json:"operation"`
	}
	if json.Unmarshal(args, &params) != nil {
		return false
	}
	dispatch := params.What
	if dispatch == "" {
		dispatch = params.Action
	}
	return strings.EqualFold(strings.TrimSpace(dispatch), "audit_log") &&
		strings.EqualFold(strings.TrimSpace(params.Operation), "clear")
}
