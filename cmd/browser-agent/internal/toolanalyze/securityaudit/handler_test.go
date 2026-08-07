// Purpose: Validate analyze security/third-party summary compact responses.
// Why: Prevents silent regressions in summary mode for security tools.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_security_test.go — Tests for security audit + third-party audit summary builders.
package securityaudit

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/analysis/thirdparty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
)

// ============================================
// Security Audit Summary Tests
// ============================================

func TestBuildSecuritySummary_Basic(t *testing.T) {
	t.Parallel()
	result := scan.Result{
		Findings: []scan.Finding{
			{Check: "credentials", Severity: "critical", Title: "API key in response"},
			{Check: "credentials", Severity: "high", Title: "Token in URL"},
			{Check: "headers", Severity: "medium", Title: "Missing CSP"},
			{Check: "headers", Severity: "medium", Title: "Missing HSTS"},
			{Check: "transport", Severity: "low", Title: "Mixed content"},
		},
		Summary: scan.Summary{
			TotalFindings: 5,
			BySeverity:    map[string]int{"critical": 1, "high": 1, "medium": 2, "low": 1},
			ByCheck:       map[string]int{"credentials": 2, "headers": 2, "transport": 1},
		},
	}

	summary := BuildSecurityAuditSummary(result)

	if summary["total"] != 5 {
		t.Errorf("total = %v, want 5", summary["total"])
	}

	bySeverity, ok := summary["by_severity"].(map[string]int)
	if !ok {
		t.Fatalf("by_severity wrong type: %T", summary["by_severity"])
	}
	if bySeverity["critical"] != 1 {
		t.Errorf("critical = %d, want 1", bySeverity["critical"])
	}

	topIssues, ok := summary["top_issues"].([]map[string]any)
	if !ok {
		t.Fatalf("top_issues wrong type: %T", summary["top_issues"])
	}
	if len(topIssues) == 0 {
		t.Fatal("expected at least one top issue")
	}
	if topIssues[0]["severity"] != "critical" {
		t.Errorf("first top issue severity = %v, want critical", topIssues[0]["severity"])
	}
}

func TestBuildSecuritySummary_Empty(t *testing.T) {
	t.Parallel()
	result := scan.Result{}
	summary := BuildSecurityAuditSummary(result)
	if summary["total"] != 0 {
		t.Errorf("total = %v, want 0", summary["total"])
	}
	topIssues := summary["top_issues"].([]map[string]any)
	if len(topIssues) != 0 {
		t.Errorf("expected 0 top issues, got %d", len(topIssues))
	}
}

func TestBuildSecuritySummary_LimitTo5(t *testing.T) {
	t.Parallel()
	findings := make([]scan.Finding, 8)
	for i := range findings {
		findings[i] = scan.Finding{Check: "headers", Severity: "medium", Title: "issue"}
	}
	result := scan.Result{Findings: findings}
	summary := BuildSecurityAuditSummary(result)
	topIssues := summary["top_issues"].([]map[string]any)
	if len(topIssues) > 5 {
		t.Errorf("top_issues should be capped at 5, got %d", len(topIssues))
	}
}

// ============================================
// Third-party Audit Summary Tests
// ============================================

func TestBuildThirdPartySummary_Basic(t *testing.T) {
	t.Parallel()
	result := thirdparty.ThirdPartyResult{
		ThirdParties: []thirdparty.ThirdPartyEntry{
			{Origin: "https://cdn.evil.com", RiskLevel: "high", RiskReason: "unknown CDN"},
			{Origin: "https://analytics.google.com", RiskLevel: "low", RiskReason: "known tracker"},
			{Origin: "https://malware.xyz", RiskLevel: "critical", RiskReason: "suspicious domain"},
		},
		Summary: thirdparty.ThirdPartySummary{
			TotalThirdParties: 3,
			CriticalRisk:      1,
			HighRisk:          1,
			LowRisk:           1,
		},
	}

	summary := BuildThirdPartySummary(result)

	if summary["total_origins"] != 3 {
		t.Errorf("total_origins = %v, want 3", summary["total_origins"])
	}

	byRisk, ok := summary["by_risk"].(map[string]int)
	if !ok {
		t.Fatalf("by_risk wrong type: %T", summary["by_risk"])
	}
	if byRisk["critical"] != 1 {
		t.Errorf("critical = %d, want 1", byRisk["critical"])
	}

	top, ok := summary["top"].([]map[string]any)
	if !ok {
		t.Fatalf("top wrong type: %T", summary["top"])
	}
	if len(top) != 3 {
		t.Fatalf("expected 3 top entries, got %d", len(top))
	}
	// Critical should be first (sorted by severity)
	if top[0]["risk"] != "critical" {
		t.Errorf("first top entry risk = %v, want critical", top[0]["risk"])
	}
}

func TestBuildThirdPartySummary_Empty(t *testing.T) {
	t.Parallel()
	result := thirdparty.ThirdPartyResult{}
	summary := BuildThirdPartySummary(result)
	if summary["total_origins"] != 0 {
		t.Errorf("total_origins = %v, want 0", summary["total_origins"])
	}
}
