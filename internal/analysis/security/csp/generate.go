// generate.go — Derives CSP directives from accumulated origin observations.
// Purpose: Derives Content Security Policy directives from accumulated origin observations.
// Why: Separates CSP generation logic from origin storage and tooling integration.
package csp

import (
	"fmt"
	"time"
)

// Generate derives policy directives from accumulated observations.
//
// Invariants:
// - Read lock protects consistent view of origins/pages during generation.
//
// Failure semantics:
// - Empty/default mode normalizes to "moderate".
// - Low-confidence or explicitly excluded origins are omitted rather than failing generation.
func (g *Generator) Generate(params Params) Response {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if params.Mode == "" {
		params.Mode = "moderate"
	}

	excludeSet := make(map[string]bool)
	for _, origin := range params.ExcludeOrigins {
		excludeSet[origin] = true
	}

	result := g.processOriginEntries(excludeSet)
	sortedDirectives := sortDirectiveSources(result.directives)
	cspHeader := g.buildCSPHeader(sortedDirectives)

	headerName := "Content-Security-Policy"
	if params.Mode == "report_only" {
		headerName = "Content-Security-Policy-Report-Only"
	}

	response := Response{
		CSPHeader:       cspHeader,
		HeaderName:      headerName,
		MetaTag:         formatMetaTag(cspHeader),
		Directives:      sortedDirectives,
		OriginDetails:   result.originDetails,
		FilteredOrigins: result.filteredOrigins,
		Observations: Observations{
			TotalResources:  result.totalResources,
			UniqueOrigins:   result.uniqueOrigins,
			OriginsIncluded: result.originsIncluded,
			OriginsFiltered: result.originsFiltered,
			PagesVisited:    len(g.pages),
		},
		Warnings:            g.buildWarnings(len(g.pages), result.originsFiltered, result.originDetails),
		RecommendedNextStep: "Deploy as Content-Security-Policy-Report-Only first. Browse all pages again and check for violations via Kaboom's console error capture. Once no violations occur, switch to enforcing mode.",
	}

	if len(params.WhitelistOverride) > 0 {
		g.applyWhitelistOverrides(&response, params.WhitelistOverride)
	}

	return response
}

// processOriginEntries classifies each accumulated origin for policy inclusion.
//
// Invariants:
// - default-src always starts with 'self'.
//
// Failure semantics:
// - Entries flagged as dev pollution/explicitly excluded are tracked as filtered and skipped.
func (g *Generator) processOriginEntries(excludeSet map[string]bool) originProcessingResult {
	pageOrigins := g.extractPageOrigins()
	directives := map[string]map[string]bool{
		"default-src": {"'self'": true},
	}
	uniqueOriginSet := make(map[string]bool)
	var r originProcessingResult

	for _, entry := range g.origins {
		r.totalResources += entry.Count
		uniqueOriginSet[entry.Origin] = true

		directive := directiveForResourceType(entry.ResourceType)

		if reason := g.isDevPollution(entry.Origin, pageOrigins); reason != "" {
			r.filteredOrigins = append(r.filteredOrigins, FilteredOrigin{Origin: entry.Origin, Reason: reason})
			r.originsFiltered++
			continue
		}
		if excludeSet[entry.Origin] {
			r.originsFiltered++
			continue
		}

		detail := g.buildOriginDetail(entry, directive)
		r.originDetails = append(r.originDetails, detail)

		if detail.Included {
			addDirectiveSource(directives, directive, entry.Origin)
			r.originsIncluded++
		}
	}

	r.directives = directives
	r.uniqueOrigins = len(uniqueOriginSet)
	return r
}

// buildOriginDetail computes confidence metadata for one origin entry.
//
// Failure semantics:
// - Low-confidence entries remain documented in output but are excluded from directives.
func (g *Generator) buildOriginDetail(entry *OriginEntry, directive string) OriginDetail {
	confidence := g.computeConfidence(entry)
	included := confidence != "low"

	detail := OriginDetail{
		Origin:           entry.Origin,
		Directive:        directive,
		Confidence:       confidence,
		ObservationCount: entry.Count,
		FirstSeen:        entry.FirstSeen.Format(time.RFC3339),
		LastSeen:         entry.LastSeen.Format(time.RFC3339),
		PagesSeenOn:      g.entryPages(entry),
		Included:         included,
	}

	if !included {
		detail.ExclusionReason = "Low confidence: observed only once. May be injected by extension or ad network. Add to exclude_origins or manually include after verification."
	}
	return detail
}

// buildWarnings generates advisory warnings based on the observation state.
func (g *Generator) buildWarnings(pagesVisited, originsFiltered int, details []OriginDetail) []string {
	if len(g.origins) == 0 {
		return []string{"No origins observed yet. Browse your app to capture resource loading patterns before generating a CSP."}
	}

	var warnings []string
	if pagesVisited < 5 {
		warnings = append(warnings, fmt.Sprintf("Only %d pages visited — ensure all app routes are exercised for complete coverage.", pagesVisited))
	}
	if lowCount := countLowConfidenceExclusions(details); lowCount > 0 {
		warnings = append(warnings, fmt.Sprintf("%d origin(s) excluded due to low confidence (seen once) — review origin_details for details.", lowCount))
	}
	return warnings
}
