// Purpose: Aggregates performance, accessibility, security, and best-practices analyzers into a single scored audit report.
// Why: Provides a Lighthouse-style combined score without requiring agents to call each analyzer separately.
// Docs: docs/features/feature/best-practices-audit/index.md

package combinedaudit

import (
	"encoding/json"
	observepage "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/page"
	observesession "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/session"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze/securityaudit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	observecore "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
)

type Deps struct {
	Analyze toolanalyze.Deps
	Observe observecore.Deps
}

// auditCategory defines a category for the combined audit.
type auditCategory struct {
	Name    string
	Handler func(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse
	Weight  float64
}

// defaultAuditCategories returns the available audit categories.
func defaultAuditCategories() []auditCategory {
	return []auditCategory{
		{Name: "performance", Handler: func(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return observesession.CheckPerformance(d.Observe, req, args)
		}, Weight: 1.0},
		{Name: "accessibility", Handler: func(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return observepage.RunA11yAudit(d.Observe, req, args)
		}, Weight: 1.0},
		{Name: "security", Handler: func(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return securityaudit.HandleSecurityAudit(d.Analyze, req, args)
		}, Weight: 1.0},
		{Name: "best_practices", Handler: func(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return securityaudit.HandleThirdPartyAudit(d.Analyze, req, args)
		}, Weight: 1.0},
	}
}

// validAuditCategories is the set of valid category names.
var validAuditCategories = map[string]bool{
	"performance":    true,
	"accessibility":  true,
	"security":       true,
	"best_practices": true,
}

// toolanalyze.AuditCategoryResult is defined in tools_analyze_audit_scoring.go as an alias
// for toolanalyze.AuditCategoryResult.

func Handle(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Categories []string `json:"categories"`
		Summary    bool     `json:"summary"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &params); stop {
			return resp
		}
	}

	// Validate categories
	requestedCategories, invalid := validateCategories(params.Categories)
	if invalid != "" {
		return mcp.Fail(req, mcp.ErrInvalidParam,
			"Unknown audit category: "+invalid,
			"Use valid categories: performance, accessibility, security, best_practices",
			mcp.WithParam("categories"))
	}

	// Build category set for filtering
	categorySet := make(map[string]bool, len(requestedCategories))
	for _, c := range requestedCategories {
		categorySet[c] = true
	}

	allCategories := defaultAuditCategories()
	categoryResults := make(map[string]toolanalyze.AuditCategoryResult)
	var totalScore float64
	var totalWeight float64

	// Run categories sequentially (avoids correlation_id collision)
	for _, cat := range allCategories {
		if !categorySet[cat.Name] {
			continue
		}

		catResult := runAuditCategory(d, req, args, cat)
		categoryResults[cat.Name] = catResult
		totalScore += float64(catResult.Score) * cat.Weight
		totalWeight += cat.Weight
	}

	overallScore := 0
	if totalWeight > 0 {
		overallScore = int(totalScore / totalWeight)
	}

	_, _, trackedURL := d.Analyze.GetTrackingStatus()

	// When summary=true, strip findings to reduce output size
	catOutput := make(map[string]any, len(categoryResults))
	for name, cr := range categoryResults {
		if params.Summary {
			m := map[string]any{
				"score":          cr.Score,
				"summary":        cr.Summary,
				"findings_count": len(cr.Findings),
			}
			if cr.Error != "" {
				m["error"] = cr.Error
			}
			catOutput[name] = m
		} else {
			catOutput[name] = cr
		}
	}

	responseData := map[string]any{
		"categories":      catOutput,
		"overall_score":   overallScore,
		"url":             trackedURL,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
		"recommendations": auditRecommendations(),
	}

	return mcp.Succeed(req, "Combined audit report", responseData)
}

func validateCategories(categories []string) ([]string, string) {
	if len(categories) == 0 {
		categories = []string{"performance", "accessibility", "security", "best_practices"}
	}
	for _, category := range categories {
		if !validAuditCategories[category] {
			return nil, category
		}
	}
	return categories, ""
}

// auditRecommendations returns best-practice recommendations surfaced with every audit.
func auditRecommendations() []map[string]any {
	return []map[string]any{
		{
			"id":       "llms-txt",
			"category": "best_practices",
			"title":    "Serve /llms.txt for LLM and agent discoverability",
			"description": "The llms.txt specification (llmstxt.org) proposes serving a markdown file at /llms.txt " +
				"that gives LLMs and AI agents concise, structured context about your site. " +
				"Sites that serve /llms.txt and per-page .md variants get better results from AI-powered " +
				"search, coding assistants, and autonomous agents. " +
				"Check whether this site serves /llms.txt — if not, consider adding one.",
			"severity":  "info",
			"reference": "https://llmstxt.org",
		},
	}
}

// runAuditCategory executes one category and normalizes its MCP response into
// the scoring contract used by the combined audit.
func runAuditCategory(d Deps, req mcp.JSONRPCRequest, args json.RawMessage, cat auditCategory) toolanalyze.AuditCategoryResult {
	resp := cat.Handler(d, req, args)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return toolanalyze.AuditCategoryResult{Score: 0, Findings: []any{}, Summary: "Failed to parse result", Error: err.Error()}
	}
	if result.IsError {
		errMsg := "unknown error"
		if len(result.Content) > 0 {
			errMsg = result.Content[0].Text
		}
		return toolanalyze.AuditCategoryResult{Score: 0, Findings: []any{}, Summary: "Category failed", Error: errMsg}
	}
	if len(result.Content) == 0 {
		return toolanalyze.AuditCategoryResult{Score: 0, Findings: []any{}, Summary: "No data available", Error: "no content returned"}
	}

	text := result.Content[0].Text
	jsonStart := -1
	for i, ch := range text {
		if ch == '{' {
			jsonStart = i
			break
		}
	}
	if jsonStart < 0 {
		return toolanalyze.AuditCategoryResult{Score: 0, Findings: []any{}, Summary: "No structured data", Error: "could not parse response"}
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(text[jsonStart:]), &data); err != nil {
		return toolanalyze.AuditCategoryResult{Score: 0, Findings: []any{}, Summary: "Could not parse audit data", Error: "malformed JSON in response"}
	}
	return toolanalyze.ScoreAuditCategory(cat.Name, data)
}
