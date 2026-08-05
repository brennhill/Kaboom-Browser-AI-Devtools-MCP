// scan.go — Security scan contracts, dispatch, filtering, and summaries.
// Purpose: Owns the aggregate security scan lifecycle and public result model.
// Why: Keeps orchestration and its state contract together without mutable registries.
// Docs: docs/features/feature/security-hardening/index.md

// Package scan audits captured network and console evidence for security risks.
package scan

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type Finding struct {
	Check       string `json:"check"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Evidence    string `json:"evidence"`
	Remediation string `json:"remediation"`
}

type Input struct {
	NetworkBodies    []types.NetworkBody
	WaterfallEntries []types.NetworkWaterfallEntry
	ConsoleEntries   []types.LogEntry
	PageURLs         []string
	URLFilter        string
	Checks           []string
	SeverityMin      string
}

type Result struct {
	Findings  []Finding `json:"findings"`
	Summary   Summary   `json:"summary"`
	ScannedAt time.Time `json:"scanned_at"`
}

type Summary struct {
	TotalFindings int            `json:"total_findings"`
	BySeverity    map[string]int `json:"by_severity"`
	ByCheck       map[string]int `json:"by_check"`
	URLsScanned   int            `json:"urls_scanned"`
}

type Scanner struct {
	mu sync.RWMutex
}

func NewScanner() *Scanner { return &Scanner{} }

func defaultSecurityChecks() []string {
	return []string{"credentials", "pii", "headers", "cookies", "transport", "auth", "network"}
}

func (s *Scanner) runSecurityChecks(checkSet map[string]bool, bodies []types.NetworkBody, input Input) []Finding {
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
		checks = defaultSecurityChecks()
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

func (s *Scanner) HandleSecurityAudit(params json.RawMessage, bodies []types.NetworkBody, entries []types.LogEntry, pageURLs []string, waterfallEntries []types.NetworkWaterfallEntry) (any, error) {
	var toolParams struct {
		Checks      []string `json:"checks"`
		URLFilter   string   `json:"url"`
		SeverityMin string   `json:"severity_min"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &toolParams); err != nil {
			return nil, fmt.Errorf("invalid security audit parameters: %w", err)
		}
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

func buildSummary(findings []Finding, bodies []types.NetworkBody) Summary {
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

func filterBodiesByURL(bodies []types.NetworkBody, filter string) []types.NetworkBody {
	if filter == "" {
		return bodies
	}
	var filtered []types.NetworkBody
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
