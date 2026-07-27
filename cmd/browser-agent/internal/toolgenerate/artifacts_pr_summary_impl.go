// artifacts_pr_summary_impl.go — Implements generate(pr_summary) artifact assembly.
// Why: Keeps PR markdown summary logic separate from other generate artifact formats.

package toolgenerate

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// HandlePRSummary generates a PR markdown summary from captured session data.
func HandlePRSummary(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	cap := d.GetCapture()
	actions := cap.GetAllEnhancedActions()
	completedCmds := cap.GetCompletedCommands()
	failedCmds := cap.GetFailedCommands()
	logs := cap.GetExtensionLogs()
	networkBodies := cap.GetNetworkBodies()
	_, _, tabURL := cap.GetTrackingStatus()

	// Count actions by type.
	actionCounts := map[string]int{}
	for _, a := range actions {
		actionCounts[a.Type]++
	}

	// Count errors in logs.
	errorCount := 0
	for _, l := range logs {
		if l.Level == "error" {
			errorCount++
		}
	}

	// Count failed network requests.
	networkErrors := 0
	for _, nb := range networkBodies {
		if nb.Status >= 400 {
			networkErrors++
		}
	}

	totalActivity := len(actions) + len(completedCmds) + len(failedCmds) + len(networkBodies)

	// Build markdown summary.
	var sb strings.Builder
	sb.WriteString("## Session Summary\n\n")

	if totalActivity == 0 {
		sb.WriteString("No activity captured during this session.\n\n")
		sb.WriteString("Navigate to a page or interact with the browser to generate activity.\n")
		return mcp.Succeed(req, "PR summary generated", map[string]any{
			"summary": sb.String(),
			"reason":  "no_activity_captured",
			"hint":    "Navigate to a page or interact with the browser first, then call generate(pr_summary) again.",
			"stats": map[string]any{
				"actions": 0, "commands_completed": 0, "commands_failed": 0,
				"console_errors": 0, "network_errors": 0, "network_captured": 0,
			},
		})
	}

	if tabURL != "" {
		sb.WriteString(fmt.Sprintf("- **Page:** %s\n", tabURL))
	}
	sb.WriteString(fmt.Sprintf("- **Actions:** %d total", len(actions)))
	if len(actionCounts) > 0 {
		parts := make([]string, 0, len(actionCounts))
		for t, c := range actionCounts {
			parts = append(parts, fmt.Sprintf("%s: %d", t, c))
		}
		sort.Strings(parts)
		sb.WriteString(fmt.Sprintf(" (%s)", strings.Join(parts, ", ")))
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("- **Commands:** %d completed, %d failed\n", len(completedCmds), len(failedCmds)))
	if errorCount > 0 {
		sb.WriteString(fmt.Sprintf("- **Console Errors:** %d\n", errorCount))
	}
	if networkErrors > 0 {
		sb.WriteString(fmt.Sprintf("- **Network Errors:** %d (HTTP 4xx/5xx)\n", networkErrors))
	}
	sb.WriteString(fmt.Sprintf("- **Network Requests Captured:** %d\n", len(networkBodies)))

	summary := sb.String()
	return mcp.Succeed(req, "PR summary generated", map[string]any{
		"summary": summary,
		"stats": map[string]any{
			"actions":            len(actions),
			"commands_completed": len(completedCmds),
			"commands_failed":    len(failedCmds),
			"console_errors":     errorCount,
			"network_errors":     networkErrors,
			"network_captured":   len(networkBodies),
		},
	})
}
