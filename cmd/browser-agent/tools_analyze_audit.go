// Purpose: Aggregates performance, accessibility, security, and best-practices analyzers into a single scored audit report.
// Why: Provides a Lighthouse-style combined score without requiring agents to call each analyzer separately.
// Docs: docs/features/feature/best-practices-audit/index.md

package main

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/analyzehandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe"
)

// auditCategory defines a category for the combined audit.
type auditCategory struct {
	Name    string
	Handler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse
	Weight  float64
}

// defaultAuditCategories returns the available audit categories.
func defaultAuditCategories() []auditCategory {
	return []auditCategory{
		{Name: "performance", Handler: func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return observe.CheckPerformance(h, req, args)
		}, Weight: 1.0},
		{Name: "accessibility", Handler: func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return observe.RunA11yAudit(h, req, args)
		}, Weight: 1.0},
		{Name: "security", Handler: func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return analyzehandler.HandleSecurityAudit(h, req, args)
		}, Weight: 1.0},
		{Name: "best_practices", Handler: func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return analyzehandler.HandleThirdPartyAudit(h, req, args)
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

// analyzehandler.AuditCategoryResult is defined in tools_analyze_audit_scoring.go as an alias
// for analyzehandler.AuditCategoryResult.

func (h *ToolHandler) toolAnalyzeAudit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Categories []string `json:"categories"`
		Summary    bool     `json:"summary"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	// Validate categories
	requestedCategories := params.Categories
	if len(requestedCategories) == 0 {
		requestedCategories = []string{"performance", "accessibility", "security", "best_practices"}
	}
	for _, cat := range requestedCategories {
		if !validAuditCategories[cat] {
			return fail(req, ErrInvalidParam,
				"Unknown audit category: "+cat,
				"Use valid categories: performance, accessibility, security, best_practices",
				withParam("categories"))
		}
	}

	// Build category set for filtering
	categorySet := make(map[string]bool, len(requestedCategories))
	for _, c := range requestedCategories {
		categorySet[c] = true
	}

	allCategories := defaultAuditCategories()
	categoryResults := make(map[string]analyzehandler.AuditCategoryResult)
	var totalScore float64
	var totalWeight float64

	// Run categories sequentially (avoids correlation_id collision)
	for _, cat := range allCategories {
		if !categorySet[cat.Name] {
			continue
		}

		catResult := runAuditCategory(h, req, args, cat)
		categoryResults[cat.Name] = catResult
		totalScore += float64(catResult.Score) * cat.Weight
		totalWeight += cat.Weight
	}

	overallScore := 0
	if totalWeight > 0 {
		overallScore = int(totalScore / totalWeight)
	}

	_, _, trackedURL := h.capture.GetTrackingStatus()

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

	return succeed(req, "Combined audit report", responseData)
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
