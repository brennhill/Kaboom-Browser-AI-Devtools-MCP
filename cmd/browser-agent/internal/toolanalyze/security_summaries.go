// security_summaries.go — Summary builders for security_audit and third_party_audit results.
// Why: Keeps summary construction logic separate from handler dispatch.
// Docs: docs/features/feature/security-hardening/index.md

package toolanalyze

import (
	"sort"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/analysis"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
)

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

	// Sort findings by severity (critical first)
	sorted := make([]scan.Finding, len(result.Findings))
	copy(sorted, result.Findings)
	sort.Slice(sorted, func(i, j int) bool {
		return SeverityOrder[sorted[i].Severity] < SeverityOrder[sorted[j].Severity]
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
func BuildThirdPartySummary(result analysis.ThirdPartyResult) map[string]any {
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

	// Sort by risk (critical first)
	sorted := make([]analysis.ThirdPartyEntry, len(result.ThirdParties))
	copy(sorted, result.ThirdParties)
	sort.Slice(sorted, func(i, j int) bool {
		return SeverityOrder[sorted[i].RiskLevel] < SeverityOrder[sorted[j].RiskLevel]
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
