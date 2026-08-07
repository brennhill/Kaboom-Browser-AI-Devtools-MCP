// security.go — Handles analyze modes for security_audit and third_party_audit.
// Why: Isolates security-focused analysis from general analyze dispatch.
// Docs: docs/features/feature/security-hardening/index.md

package securityaudit

import (
	"encoding/json"
	"sort"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/analysis/thirdparty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
)

// HandleSecurityAudit handles analyze(what="security_audit").
func HandleSecurityAudit(d toolanalyze.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		SeverityMin string   `json:"severity_min"`
		Checks      []string `json:"checks"`
		URLFilter   string   `json:"url"`
		Summary     bool     `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)

	scanner := d.SecurityScanner()
	if scanner == nil {
		return mcp.Fail(req, mcp.ErrNotInitialized, "Security scanner not initialized", "Internal error — do not retry")
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
		return mcp.Fail(req, mcp.ErrInternal, err.Error(), "Internal error — do not retry")
	}

	if params.Summary {
		if scanResult, ok := result.(scan.Result); ok {
			return mcp.Succeed(req, "Security audit summary", BuildSecurityAuditSummary(scanResult))
		}
	}

	return mcp.Succeed(req, "Security audit complete", result)
}

// HandleThirdPartyAudit handles analyze(what="third_party_audit").
func HandleThirdPartyAudit(d toolanalyze.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Summary bool `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)

	networkBodies := d.NetworkBodies()

	var pageURLs []string
	_, _, tabURL := d.GetTrackingStatus()
	if tabURL != "" {
		pageURLs = append(pageURLs, tabURL)
	}

	result, err := thirdparty.HandleAuditThirdParties(args, networkBodies, pageURLs)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, err.Error(), "Fix JSON arguments and try again")
	}

	if params.Summary {
		if tpResult, ok := result.(thirdparty.ThirdPartyResult); ok {
			return mcp.Succeed(req, "Third-party audit summary", BuildThirdPartySummary(tpResult))
		}
	}

	return mcp.Succeed(req, "Third-party audit complete", result)
}

// BuildSecurityAuditSummary creates a compact summary from security scan results.
func BuildSecurityAuditSummary(result scan.Result) map[string]any {
	bySeverity := make(map[string]int)
	for _, f := range result.Findings {
		bySeverity[f.Severity]++
	}

	topN := 5
	if len(result.Findings) < topN {
		topN = len(result.Findings)
	}

	sorted := make([]scan.Finding, len(result.Findings))
	copy(sorted, result.Findings)
	sort.Slice(sorted, func(i, j int) bool {
		return toolanalyze.SeverityOrder[sorted[i].Severity] < toolanalyze.SeverityOrder[sorted[j].Severity]
	})

	topIssues := make([]map[string]any, topN)
	for i := 0; i < topN; i++ {
		topIssues[i] = map[string]any{
			"check":    sorted[i].Check,
			"severity": sorted[i].Severity,
			"title":    sorted[i].Title,
		}
	}

	return map[string]any{
		"total":       len(result.Findings),
		"by_severity": bySeverity,
		"top_issues":  topIssues,
	}
}

// BuildThirdPartySummary creates a compact summary from third-party audit results.
func BuildThirdPartySummary(result thirdparty.ThirdPartyResult) map[string]any {
	byRisk := map[string]int{
		"critical": result.Summary.CriticalRisk,
		"high":     result.Summary.HighRisk,
		"medium":   result.Summary.MediumRisk,
		"low":      result.Summary.LowRisk,
	}

	topN := 5
	if len(result.ThirdParties) < topN {
		topN = len(result.ThirdParties)
	}

	sorted := make([]thirdparty.ThirdPartyEntry, len(result.ThirdParties))
	copy(sorted, result.ThirdParties)
	sort.Slice(sorted, func(i, j int) bool {
		return toolanalyze.SeverityOrder[sorted[i].RiskLevel] < toolanalyze.SeverityOrder[sorted[j].RiskLevel]
	})

	top := make([]map[string]any, topN)
	for i := 0; i < topN; i++ {
		top[i] = map[string]any{
			"origin": sorted[i].Origin,
			"risk":   sorted[i].RiskLevel,
			"reason": sorted[i].RiskReason,
		}
	}

	return map[string]any{
		"total_origins": len(result.ThirdParties),
		"by_risk":       byRisk,
		"top":           top,
	}
}
