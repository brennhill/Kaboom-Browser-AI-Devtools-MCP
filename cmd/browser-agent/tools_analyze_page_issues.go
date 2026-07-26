// tools_analyze_page_issues.go — Aggregates all detectable page issues into a single response.
// Why: Gives any AI a comprehensive starting point without needing to know which individual tools to call.
// Docs: docs/features/feature/auto-fix/index.md

package main

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/scan"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const (
	catConsoleErrors   = "console_errors"
	catNetworkFailures = "network_failures"
	catAccessibility   = "accessibility"
	catSecurity        = "security"

	pageIssuesPerSectionCap = 50
	pageIssuesCheckTimeout  = 5 * time.Second
	pageIssuesSummaryTopN   = 10
)

var allPageIssuesCategories = []string{catConsoleErrors, catNetworkFailures, catAccessibility, catSecurity}

type pageIssuesParams struct {
	Summary    bool     `json:"summary"`
	Categories []string `json:"categories"`
	Limit      int      `json:"limit"`
}

type pageIssuesResult struct {
	TotalIssues     int            `json:"total_issues"`
	BySeverity      map[string]int `json:"by_severity"`
	Sections        map[string]any `json:"sections"`
	ChecksCompleted []string       `json:"checks_completed"`
	ChecksSkipped   []string       `json:"checks_skipped"`
	PageURL         string         `json:"page_url"`
	Timestamp       string         `json:"timestamp"`
}

type checkResult struct {
	name   string
	issues []map[string]any
	err    string
}

func (h *ToolHandler) toolAnalyzePageIssues(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params pageIssuesParams
	if len(args) > 0 {
		lenientUnmarshal(args, &params)
	}
	if params.Limit <= 0 {
		params.Limit = pageIssuesPerSectionCap
	}

	enabled, _, tabURL := h.capture.GetTrackingStatus()
	if !enabled {
		return fail(req, ErrNoData, "No tab is being tracked. Track a tab first.",
			"Open the extension popup and click 'Track This Tab'.",
			withRecoveryToolCall(map[string]any{
				"tool":      "configure",
				"arguments": map[string]any{"what": "health"},
			}))
	}

	categories := defaultCategories(params.Categories)
	result := h.runPageIssuesChecks(categories, params.Limit, tabURL)

	if params.Summary {
		azResult := toolanalyze.PageIssuesResult{
			TotalIssues:     result.TotalIssues,
			BySeverity:      result.BySeverity,
			Sections:        result.Sections,
			ChecksCompleted: result.ChecksCompleted,
			ChecksSkipped:   result.ChecksSkipped,
			PageURL:         result.PageURL,
			Timestamp:       result.Timestamp,
		}
		return succeed(req, "Page issues summary", toolanalyze.BuildPageIssuesSummary(azResult))
	}
	return succeed(req, "Page issues scan complete", result)
}

func defaultCategories(requested []string) map[string]bool {
	src := allPageIssuesCategories
	if len(requested) > 0 {
		src = requested
	}
	m := make(map[string]bool, len(src))
	for _, c := range src {
		m[c] = true
	}
	return m
}

type pageIssuesChecker struct {
	name string
	fn   func(int) ([]map[string]any, error)
}

// Pre-fetch shared data once to avoid redundant buffer copies across parallel checkers.
type sharedPageData struct {
	networkBodies    []capture.NetworkBody
	waterfallEntries []capture.NetworkWaterfallEntry
	logEntries       []LogEntry
	consoleEntries   []scan.LogEntry
	tabURL           string
}

func (h *ToolHandler) prefetchSharedData(tabURL string) sharedPageData {
	bodies := h.capture.GetNetworkBodies()
	waterfall := h.capture.GetNetworkWaterfallEntries()

	// Single lock acquisition for both logEntries and consoleEntries to avoid
	// TOCTOU inconsistency between two separate lock acquisitions on the same data.
	h.server.logs.mu.RLock()
	logEntries := make([]LogEntry, len(h.server.logs.entries))
	copy(logEntries, h.server.logs.entries)
	consoleEntries := make([]scan.LogEntry, len(h.server.logs.entries))
	for i, e := range h.server.logs.entries {
		consoleEntries[i] = scan.LogEntry(e)
	}
	h.server.logs.mu.RUnlock()

	return sharedPageData{
		networkBodies:    bodies,
		waterfallEntries: waterfall,
		logEntries:       logEntries,
		consoleEntries:   consoleEntries,
		tabURL:           tabURL,
	}
}

func (h *ToolHandler) runPageIssuesChecks(categories map[string]bool, limit int, tabURL string) pageIssuesResult {
	shared := h.prefetchSharedData(tabURL)

	checkers := make([]pageIssuesChecker, 0, len(categories))
	if categories[catConsoleErrors] {
		checkers = append(checkers, pageIssuesChecker{catConsoleErrors, func(lim int) ([]map[string]any, error) {
			return collectConsoleErrors(shared.logEntries, lim), nil
		}})
	}
	if categories[catNetworkFailures] {
		checkers = append(checkers, pageIssuesChecker{catNetworkFailures, func(lim int) ([]map[string]any, error) {
			return collectNetworkFailures(shared.networkBodies, lim), nil
		}})
	}
	if categories[catAccessibility] {
		checkers = append(checkers, pageIssuesChecker{catAccessibility, func(lim int) ([]map[string]any, error) {
			return h.collectA11yIssues(lim)
		}})
	}
	if categories[catSecurity] {
		checkers = append(checkers, pageIssuesChecker{catSecurity, func(lim int) ([]map[string]any, error) {
			return h.collectSecurityIssues(shared, lim)
		}})
	}

	results := make(chan checkResult, len(checkers))
	var wg sync.WaitGroup

	for _, c := range checkers {
		wg.Add(1)
		util.SafeGo(func() {
			defer wg.Done()
			// Buffered so the inner goroutine can send without blocking if timeout fires first.
			done := make(chan checkResult, 1)
			util.SafeGo(func() {
				issues, err := c.fn(limit)
				cr := checkResult{name: c.name, issues: issues}
				if err != nil {
					cr.err = err.Error()
				}
				done <- cr
			})
			select {
			case r := <-done:
				results <- r
			case <-time.After(pageIssuesCheckTimeout):
				results <- checkResult{name: c.name, err: "timeout"}
			}
		})
	}

	util.SafeGo(func() {
		wg.Wait()
		close(results)
	})

	completed := make([]string, 0, len(checkers))
	skipped := make([]string, 0)
	sections := make(map[string]any)
	totalIssues := 0
	bySeverity := make(map[string]int)

	for r := range results {
		if r.err != "" {
			if r.err == "timeout" {
				skipped = append(skipped, r.name)
			} else {
				completed = append(completed, r.name)
				sections[r.name] = map[string]any{
					"issues": []map[string]any{},
					"total":  0,
					"error":  r.err,
				}
			}
			continue
		}
		completed = append(completed, r.name)
		issues := r.issues
		if issues == nil {
			issues = []map[string]any{}
		}
		sections[r.name] = map[string]any{
			"issues": issues,
			"total":  len(issues),
		}
		totalIssues += len(issues)
		for _, issue := range issues {
			if sev, _ := issue["severity"].(string); sev != "" {
				bySeverity[sev]++
			}
		}
	}

	return pageIssuesResult{
		TotalIssues:     totalIssues,
		BySeverity:      bySeverity,
		Sections:        sections,
		ChecksCompleted: completed,
		ChecksSkipped:   skipped,
		PageURL:         tabURL,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}
}

// collectConsoleErrors gathers error/warning log entries from pre-fetched data.
func collectConsoleErrors(entries []LogEntry, limit int) []map[string]any {
	issues := make([]map[string]any, 0)
	for _, entry := range entries {
		level, _ := entry["level"].(string)
		if level != "error" && level != "warn" {
			continue
		}
		msg, _ := entry["message"].(string)
		if msg == "" {
			continue
		}
		severity := "medium"
		if level == "error" {
			severity = "high"
		}
		issues = append(issues, map[string]any{
			"severity":    severity,
			"message":     msg,
			"level":       level,
			"source":      entry["source"],
			"url":         entry["url"],
			"stack_trace": entry["stackTrace"],
		})
		if len(issues) >= limit {
			break
		}
	}
	return issues
}

// collectNetworkFailures gathers HTTP 4xx/5xx from pre-fetched data.
func collectNetworkFailures(bodies []capture.NetworkBody, limit int) []map[string]any {
	issues := make([]map[string]any, 0)
	for _, b := range bodies {
		if b.Status < 400 {
			continue
		}
		severity := "medium"
		if b.Status >= 500 {
			severity = "high"
		}
		issues = append(issues, map[string]any{
			"severity":     severity,
			"url":          b.URL,
			"method":       b.Method,
			"status":       b.Status,
			"content_type": b.ContentType,
			"duration_ms":  b.Duration,
		})
		if len(issues) >= limit {
			break
		}
	}
	return issues
}

// collectA11yIssues runs an accessibility audit via the extension (async, not pre-fetchable).
func (h *ToolHandler) collectA11yIssues(limit int) ([]map[string]any, error) {
	result, err := h.ExecuteA11yQuery("", nil, nil, false)
	if err != nil {
		return nil, err
	}

	var auditResult map[string]any
	if err := json.Unmarshal(result, &auditResult); err != nil {
		return nil, err
	}

	violations, _ := auditResult["violations"].([]any)
	issues := make([]map[string]any, 0)
	for _, v := range violations {
		vMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		impact, _ := vMap["impact"].(string)
		id, _ := vMap["id"].(string)
		description, _ := vMap["description"].(string)
		nodes, _ := vMap["nodes"].([]any)
		issues = append(issues, map[string]any{
			"severity":    mapA11yImpact(impact),
			"rule":        id,
			"description": description,
			"impact":      impact,
			"node_count":  len(nodes),
		})
		if len(issues) >= limit {
			break
		}
	}
	return issues, nil
}

// collectSecurityIssues runs the security scanner against pre-fetched data.
func (h *ToolHandler) collectSecurityIssues(shared sharedPageData, limit int) ([]map[string]any, error) {
	if h.securityScannerImpl == nil {
		return nil, nil
	}

	var pageURLs []string
	if shared.tabURL != "" {
		pageURLs = append(pageURLs, shared.tabURL)
	}

	result, err := h.securityScannerImpl.HandleSecurityAudit(
		json.RawMessage("{}"), shared.networkBodies, shared.consoleEntries, pageURLs, shared.waterfallEntries,
	)
	if err != nil {
		return nil, err
	}

	scanResult, ok := result.(scan.Result)
	if !ok {
		return nil, nil
	}

	issues := make([]map[string]any, 0)
	for _, f := range scanResult.Findings {
		issues = append(issues, map[string]any{
			"severity": f.Severity,
			"check":    f.Check,
			"title":    f.Title,
			"evidence": f.Evidence,
		})
		if len(issues) >= limit {
			break
		}
	}
	return issues, nil
}

// mapA11yImpact translates axe-core impact levels to the unified severity scale
// used across all page_issues sections (critical/high/medium/low/info).
func mapA11yImpact(impact string) string {
	switch impact {
	case "critical":
		return "critical"
	case "serious":
		return "high"
	case "moderate":
		return "medium"
	case "minor":
		return "low"
	default:
		return "info"
	}
}
