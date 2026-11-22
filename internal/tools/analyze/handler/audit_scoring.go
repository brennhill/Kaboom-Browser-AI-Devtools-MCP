// audit_scoring.go — Maps raw analyzer outputs to normalized 0-100 audit category scores.
// Why: Encapsulates category-specific scoring heuristics and findings extraction logic.
// Docs: docs/features/feature/best-practices-audit/index.md

package toolanalyze

// AuditCategoryResult holds the result for one audit category.
type AuditCategoryResult struct {
	Score    int    `json:"score"`
	Findings []any  `json:"findings"`
	Summary  string `json:"summary"`
	Error    string `json:"error,omitempty"`
}

// ScoreAuditCategory maps raw analyzer output to a 0-100 score.
func ScoreAuditCategory(name string, data map[string]any) AuditCategoryResult {
	result := AuditCategoryResult{Score: 100, Findings: []any{}}

	switch name {
	case "performance":
		result = scorePerformance(data)
	case "accessibility":
		result = scoreAccessibility(data)
	case "security":
		result = scoreSecurity(data)
	case "best_practices":
		result = scoreBestPractices(data)
	}

	return result
}

func scorePerformance(data map[string]any) AuditCategoryResult {
	findings := ExtractFindings(data, "issues", "warnings")
	score := 100 - len(findings)*10
	if score < 0 {
		score = 0
	}
	summary := "Performance analysis"
	if len(findings) == 0 {
		summary = "No performance issues detected"
	}
	return AuditCategoryResult{Score: score, Findings: findings, Summary: summary}
}

func scoreAccessibility(data map[string]any) AuditCategoryResult {
	findings := ExtractFindings(data, "violations", "issues")
	score := 100 - len(findings)*5
	if score < 0 {
		score = 0
	}
	summary := "Accessibility audit"
	if len(findings) == 0 {
		summary = "No accessibility violations detected"
	}
	return AuditCategoryResult{Score: score, Findings: findings, Summary: summary}
}

func scoreSecurity(data map[string]any) AuditCategoryResult {
	findings := ExtractFindings(data, "findings", "issues", "vulnerabilities")
	score := 100
	for _, f := range findings {
		if fm, ok := f.(map[string]any); ok {
			switch fm["severity"] {
			case "critical":
				score -= 25
			case "high":
				score -= 15
			case "medium":
				score -= 10
			case "low":
				score -= 5
			default:
				score -= 5
			}
		} else {
			score -= 5
		}
	}
	if score < 0 {
		score = 0
	}
	summary := "Security audit"
	if len(findings) == 0 {
		summary = "No security issues detected"
	}
	return AuditCategoryResult{Score: score, Findings: findings, Summary: summary}
}

func scoreBestPractices(data map[string]any) AuditCategoryResult {
	findings := ExtractFindings(data, "third_parties", "issues", "findings")
	score := 100 - len(findings)*3
	if score < 0 {
		score = 0
	}
	summary := "Best practices audit"
	if len(findings) == 0 {
		summary = "No best practices issues detected"
	}
	return AuditCategoryResult{Score: score, Findings: findings, Summary: summary}
}

// ExtractFindings extracts array findings from data using multiple candidate keys.
func ExtractFindings(data map[string]any, keys ...string) []any {
	for _, key := range keys {
		if arr, ok := data[key].([]any); ok && len(arr) > 0 {
			return arr
		}
	}
	return []any{}
}
