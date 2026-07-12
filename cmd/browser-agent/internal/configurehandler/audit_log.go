// audit_log.go — Handles configure(what="audit_log") report/analyze/clear operations.
// Isolates audit-trail filtering and session cleanup from the configure router.

package configurehandler

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
)

// HandleAuditLog handles configure(what="audit_log") with operation report|analyze|clear.
func HandleAuditLog(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	if !d.AuditTrailReady() {
		return fail(req, mcp.ErrNotInitialized, "Audit trail not initialized", "Internal error — do not retry")
	}

	var params struct {
		Operation      string `json:"operation"`
		AuditSessionID string `json:"audit_session_id"`
		ToolName       string `json:"tool_name"`
		Limit          int    `json:"limit"`
		Since          string `json:"since"`
	}
	lenientUnmarshal(args, &params)

	operation := strings.ToLower(strings.TrimSpace(params.Operation))
	if operation == "" {
		operation = "report"
	}
	if operation != "analyze" && operation != "report" && operation != "clear" {
		return fail(req, mcp.ErrInvalidParam, "Invalid audit_log operation: "+params.Operation, "Use operation: analyze, report, or clear", mcp.WithParam("operation"))
	}
	if operation == "clear" {
		cleared := d.ClearAuditLog()
		return succeed(req, "Audit log cleared", map[string]any{
			"status":    "ok",
			"operation": "clear",
			"cleared":   cleared,
		})
	}

	filter := audit.Filter{
		AuditSessionID: params.AuditSessionID,
		ToolName:       params.ToolName,
		Limit:          params.Limit,
	}
	if params.Since != "" {
		since, err := time.Parse(time.RFC3339, params.Since)
		if err != nil {
			return fail(req, mcp.ErrInvalidParam, "Invalid 'since' timestamp: "+err.Error(), "Use RFC3339 format, for example 2026-02-17T15:04:05Z", mcp.WithParam("since"))
		}
		filter.Since = &since
	}

	entries := d.QueryAuditLog(filter)
	if operation == "analyze" {
		return succeed(req, "Audit log analysis", map[string]any{
			"status":    "ok",
			"operation": "analyze",
			"summary":   cfg.SummarizeAuditEntries(entries),
		})
	}

	return succeed(req, "Audit log entries", map[string]any{
		"status":    "ok",
		"operation": "report",
		"entries":   entries,
		"count":     len(entries),
	})
}
