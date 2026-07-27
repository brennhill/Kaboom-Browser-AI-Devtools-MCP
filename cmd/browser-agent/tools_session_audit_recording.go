// Purpose: Records tool-call audit entries and manages per-client audit sessions.
// Why: Separates audit trail persistence and filtering rules from snapshot reader logic.
// Docs: docs/features/feature/enterprise-audit/index.md

package main

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func (h *ToolHandler) recordAuditToolCall(
	req mcp.JSONRPCRequest,
	toolName string,
	args json.RawMessage,
	resp mcp.JSONRPCResponse,
	started time.Time,
) {
	if h == nil || h.auditTrail == nil {
		return
	}
	if shouldSkipAuditRecording(toolName, args) {
		return
	}

	sessionID := h.auditSessionForClient(req.ClientID)
	if sessionID == "" {
		return
	}

	success := resp.Error == nil && !isToolResultError(resp.Result)
	entry := audit.Entry{
		AuditSessionID: sessionID,
		ClientID:       normalizeAuditClientID(req.ClientID),
		ToolName:       toolName,
		Parameters:     string(args),
		ResponseSize:   len(resp.Result),
		Duration:       time.Since(started).Milliseconds(),
		Success:        success,
	}
	if !success {
		entry.ErrorMessage = auditErrorMessage(resp)
	}

	h.auditTrail.Record(entry)
}

func (h *ToolHandler) auditSessionForClient(clientID string) string {
	if h == nil || h.auditTrail == nil {
		return ""
	}

	client := normalizeAuditClientID(clientID)

	h.auditMu.Lock()
	defer h.auditMu.Unlock()

	if sid, ok := h.auditSessionMap[client]; ok && sid != "" {
		if h.auditTrail.GetAuditSession(sid) != nil {
			return sid
		}
		delete(h.auditSessionMap, client)
	}

	info := h.auditTrail.CreateAuditSession(audit.ClientIdentifier{Name: client})
	if info == nil || info.ID == "" {
		return ""
	}
	h.auditSessionMap[client] = info.ID
	return info.ID
}

func normalizeAuditClientID(clientID string) string {
	trimmed := strings.TrimSpace(clientID)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}

func auditErrorMessage(resp mcp.JSONRPCResponse) string {
	if resp.Error != nil && resp.Error.Message != "" {
		return resp.Error.Message
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return ""
	}
	if len(result.Content) == 0 {
		return ""
	}
	return result.Content[0].Text
}

func shouldSkipAuditRecording(toolName string, args json.RawMessage) bool {
	if toolName != "configure" || len(args) == 0 {
		return false
	}
	var params struct {
		What      string `json:"what"`
		Action    string `json:"action"`
		Operation string `json:"operation"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return false
	}
	dispatch := params.What
	if dispatch == "" {
		dispatch = params.Action
	}
	return strings.EqualFold(strings.TrimSpace(dispatch), "audit_log") &&
		strings.EqualFold(strings.TrimSpace(params.Operation), "clear")
}
