// security.go — Handles analyze modes for security_audit and third_party_audit.
// Why: Isolates security-focused analysis from general analyze dispatch.
// Docs: docs/features/feature/security-hardening/index.md

package toolanalyze

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/analysis/thirdparty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
)

// HandleSecurityAudit handles analyze(what="security_audit").
func HandleSecurityAudit(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		SeverityMin string   `json:"severity_min"`
		Checks      []string `json:"checks"`
		URLFilter   string   `json:"url"`
		Summary     bool     `json:"summary"`
	}
	lenientUnmarshal(args, &params)

	scanner := d.SecurityScanner()
	if scanner == nil {
		return fail(req, mcp.ErrNotInitialized, "Security scanner not initialized", "Internal error — do not retry")
	}

	networkBodies := d.NetworkBodies()
	waterfallEntries := d.NetworkWaterfallEntries()
	consoleEntries := d.ConsoleSecurityEntries()

	var pageURLs []string
	_, _, tabURL := d.GetTrackingStatus()
	if tabURL != "" {
		pageURLs = append(pageURLs, tabURL)
	}

	result, err := scanner.HandleSecurityAudit(args, networkBodies, consoleEntries, pageURLs, waterfallEntries)
	if err != nil {
		return fail(req, mcp.ErrInternal, err.Error(), "Internal error — do not retry")
	}

	if params.Summary {
		if scanResult, ok := result.(scan.Result); ok {
			return succeed(req, "Security audit summary", BuildSecurityAuditSummary(scanResult))
		}
	}

	return succeed(req, "Security audit complete", result)
}

// HandleThirdPartyAudit handles analyze(what="third_party_audit").
func HandleThirdPartyAudit(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Summary bool `json:"summary"`
	}
	lenientUnmarshal(args, &params)

	networkBodies := d.NetworkBodies()

	var pageURLs []string
	_, _, tabURL := d.GetTrackingStatus()
	if tabURL != "" {
		pageURLs = append(pageURLs, tabURL)
	}

	result, err := thirdparty.HandleAuditThirdParties(args, networkBodies, pageURLs)
	if err != nil {
		return fail(req, mcp.ErrInvalidJSON, err.Error(), "Fix JSON arguments and try again")
	}

	if params.Summary {
		if tpResult, ok := result.(thirdparty.ThirdPartyResult); ok {
			return succeed(req, "Third-party audit summary", BuildThirdPartySummary(tpResult))
		}
	}

	return succeed(req, "Third-party audit complete", result)
}
