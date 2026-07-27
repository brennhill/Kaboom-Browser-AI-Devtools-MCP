// Purpose: Summarizes audit trail entries into tool call counts, success/failure rates, and session aggregations.
// Docs: docs/features/feature/config-profiles/index.md

package configure

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/audit"

// SummarizeAuditEntries aggregates audit entries into a summary map
// with tool call counts, success/failure rates, and session counts.
func SummarizeAuditEntries(entries []audit.AuditEntry) map[string]any {
	byTool := make(map[string]int)
	uniqueSessions := make(map[string]struct{})
	success := 0
	failed := 0
	for _, entry := range entries {
		byTool[entry.ToolName]++
		uniqueSessions[entry.AuditSessionID] = struct{}{}
		if entry.Success {
			success++
		} else {
			failed++
		}
	}

	return map[string]any{
		"total_calls":         len(entries),
		"success_count":       success,
		"failure_count":       failed,
		"audit_session_count": len(uniqueSessions),
		"calls_by_tool":       byTool,
	}
}
