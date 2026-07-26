// scan.go — Check dispatch, severity/URL filtering and summary construction.
// Purpose: Orchestrates security checks by dispatching to credential, header, cookie, and transport scanners.
// Why: Separates scan orchestration from individual check implementations.
package scan

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func (s *Scanner) runSecurityChecks(checkSet map[string]bool, bodies []capture.NetworkBody, input Input) []Finding {
	type checkEntry struct {
		name string
		fn   func() []Finding
	}
	checks := []checkEntry{
		{"credentials", func() []Finding { return s.checkCredentials(bodies, input.ConsoleEntries) }},
		{"pii", func() []Finding { return s.checkPII(bodies, input.PageURLs) }},
		{"headers", func() []Finding { return s.checkSecurityHeaders(bodies) }},
		{"cookies", func() []Finding { return s.checkCookies(bodies) }},
		{"transport", func() []Finding { return s.checkTransport(bodies, input.PageURLs) }},
		{"auth", func() []Finding { return s.checkAuthPatterns(bodies) }},
		{"network", func() []Finding { return s.checkNetworkSecurity(input.WaterfallEntries, input.PageURLs) }},
	}

	var findings []Finding
	for _, c := range checks {
		if checkSet[c.name] {
			findings = append(findings, c.fn()...)
		}
	}
	return findings
}

func (s *Scanner) Scan(input Input) Result {
	s.mu.RLock()
	defer s.mu.RUnlock()

	checks := input.Checks
	if len(checks) == 0 {
		checks = defaultChecks
	}
	checkSet := make(map[string]bool)
	for _, c := range checks {
		checkSet[c] = true
	}

	bodies := filterBodiesByURL(input.NetworkBodies, input.URLFilter)
	findings := s.runSecurityChecks(checkSet, bodies, input)
	if input.SeverityMin != "" {
		findings = filterBySeverity(findings, input.SeverityMin)
	}

	return Result{
		Findings:  findings,
		Summary:   buildSummary(findings, bodies),
		ScannedAt: time.Now(),
	}
}

func (s *Scanner) HandleSecurityAudit(params json.RawMessage, bodies []capture.NetworkBody, entries []LogEntry, pageURLs []string, waterfallEntries []capture.NetworkWaterfallEntry) (any, error) {
	var toolParams struct {
		Checks      []string `json:"checks"`
		URLFilter   string   `json:"url"`
		SeverityMin string   `json:"severity_min"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &toolParams)
	}

	input := Input{
		NetworkBodies:    bodies,
		WaterfallEntries: waterfallEntries,
		ConsoleEntries:   entries,
		PageURLs:         pageURLs,
		URLFilter:        toolParams.URLFilter,
		Checks:           toolParams.Checks,
		SeverityMin:      toolParams.SeverityMin,
	}

	result := s.Scan(input)
	return result, nil
}

func buildSummary(findings []Finding, bodies []capture.NetworkBody) Summary {
	bySeverity := make(map[string]int)
	byCheck := make(map[string]int)
	for _, f := range findings {
		bySeverity[f.Severity]++
		byCheck[f.Check]++
	}

	urlSet := make(map[string]bool)
	for _, b := range bodies {
		urlSet[b.URL] = true
	}

	return Summary{
		TotalFindings: len(findings),
		BySeverity:    bySeverity,
		ByCheck:       byCheck,
		URLsScanned:   len(urlSet),
	}
}

func filterBodiesByURL(bodies []capture.NetworkBody, filter string) []capture.NetworkBody {
	if filter == "" {
		return bodies
	}
	var filtered []capture.NetworkBody
	for _, b := range bodies {
		if strings.Contains(b.URL, filter) {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

func filterBySeverity(findings []Finding, minSeverity string) []Finding {
	severityOrder := map[string]int{
		"info":     0,
		"low":      1,
		"medium":   2,
		"warning":  2,
		"high":     3,
		"critical": 4,
	}
	minLevel, ok := severityOrder[minSeverity]
	if !ok {
		return findings
	}

	var filtered []Finding
	for _, f := range findings {
		level := severityOrder[f.Severity]
		if level >= minLevel {
			filtered = append(filtered, f)
		}
	}
	return filtered
}
