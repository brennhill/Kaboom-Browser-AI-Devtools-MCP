// finding.go — The one finding shape and response envelope all design analyzers emit.
//
// PURPOSE: give style_consistency, design_tokens and spacing a single vocabulary
// so their results aggregate, sort and triage together.
//
// CONTRACT: three analyzers writing three shapes is the divergence the DRY
// checklist exists to prevent, and it would make the categories impossible to
// score against each other. Every analyzer returns []finding and nothing else.

package designdrift

import "sort"

// Categories, one per originating GitHub issue.
const (
	categoryStyleConsistency = "style_consistency" // #693
	categoryDesignTokens     = "design_tokens"     // #694
	categorySpacing          = "spacing"           // #695
)

// allCategories() is the valid set, in the order findings are reported.
//
// A function rather than a package var so the set cannot be mutated at runtime:
// a slice global is writable by anything in the package, and this one decides
// which analyzers run and how the response is ordered.
func allCategories() []string {
	return []string{categoryStyleConsistency, categoryDesignTokens, categorySpacing}
}

// Provenance records where the expected value came from. It is the reason an
// agent can act on a finding without re-deriving how confident to be.
const (
	// provenanceDeclared means a rule that was actually stated is broken: the
	// caller's spec, or the page's own :root disagreeing with that spec.
	// Breaking it is unambiguous.
	//
	// A page token on its own does NOT make an element's value declared. The
	// page declared --spacing-md:16px; it never declared that this element's
	// padding must use it, and that last step is the analyzer's inference.
	provenanceDeclared = "declared"
	// provenanceInferred means the analyzer supplied the expectation: a majority
	// vote over the element's own peers, or proximity to a page token. Either
	// can be intentional, so both need a human look.
	provenanceInferred = "inferred"
)

// Severity is derived from provenance, never chosen independently.
//
// This is the whole triage axis: "fix all errors" is a safe batch operation
// because every error contradicts something explicitly declared, while "fix all
// warnings" is a review pass because a majority vote needs human judgement.
// Collapsing the two would force the agent to re-derive the distinction from
// the provenance field, so severityFor is the only way to set it.
const (
	severityError   = "error"
	severityWarning = "warning"
)

// severityFor maps provenance to severity. Keeping this a function rather than
// a caller-set field is what stops a finding claiming an inferred expectation
// with error severity, which would make by_severity a lie.
func severityFor(provenance string) string {
	if provenance == provenanceDeclared {
		return severityError
	}
	return severityWarning
}

// Confidence bands describe how strong the evidence behind a finding is.
//
// A 9-of-10 majority is not a 3-of-5 majority, and two elements have no
// majority at all. Reporting that difference is what keeps the analyzer honest
// about a small peer group instead of calling a coin flip.
const (
	confidenceHigh = "high"
	confidenceLow  = "low"
)

// finding is one piece of design drift. Every analyzer emits exactly this.
type finding struct {
	Category string `json:"category"`
	Severity string `json:"severity"`
	// Selector identifies the element the finding is about.
	Selector string `json:"selector"`
	// ElementIndex is the element's position among the selector's matches, so a
	// caller can distinguish peers that share a generated selector.
	ElementIndex int `json:"element_index"`
	// Property is the CSS property or geometric measure at issue.
	Property string `json:"property"`
	// Observed is what the page actually renders.
	Observed string `json:"observed"`
	// Expected is the norm the observation is judged against.
	Expected string `json:"expected"`
	// ExpectedFrom records whether Expected was declared or inferred. An agent
	// acting on Expected needs to know which, and severity follows from it.
	ExpectedFrom string `json:"expected_from"`
	Confidence   string `json:"confidence"`
	// Evidence names the peer set, token or rhythm that justified the call, so
	// a human can check the reasoning without re-running the analysis.
	Evidence string `json:"evidence"`
	// Message is a one-line human summary.
	Message string `json:"message"`
}

// findingSpec carries the verdict fields every analyzer reports. newFinding is
// the only constructor, so severity stays derived from provenance.
type findingSpec struct {
	category   string
	property   string
	el         elementView
	observed   string
	expected   string
	provenance string
	confidence string
	evidence   string
	message    string
}

// newFinding builds a finding with severity derived from provenance.
func newFinding(s findingSpec) finding {
	return finding{
		Category:     s.category,
		Severity:     severityFor(s.provenance),
		Selector:     s.el.Selector,
		ElementIndex: s.el.Index,
		Property:     s.property,
		Observed:     s.observed,
		Expected:     s.expected,
		ExpectedFrom: s.provenance,
		Confidence:   s.confidence,
		Evidence:     s.evidence,
		Message:      s.message,
	}
}

// skipped records a category that could not run, and why.
//
// A category that produces no findings because it could not run is not a clean
// page, and reporting the two the same way is how a tool starts claiming
// success it did not earn.
type skipped struct {
	Category string `json:"category"`
	Reason   string `json:"reason"`
}

// Reasons a category legitimately produces no verdict.
const (
	reasonNoTokens          = "no_tokens_declared"
	reasonInsufficientPeers = "insufficient_peers"
	reasonNoElements        = "no_elements_matched"
	// reasonNoInFlowElements distinguishes "every match is out of flow" from
	// "too few matches". They are different situations with different fixes,
	// and they are also how an extension too old to report in_flow presents
	// itself — reporting both as insufficient_peers would send someone looking
	// for missing elements that are actually there.
	reasonNoInFlowElements = "no_in_flow_elements"
	// reasonAllPeersExcluded means every match was a deliberate state variant.
	// Reporting that as insufficient_peers sends the reader looking for missing
	// elements when the elements are present and were judged ineligible.
	reasonAllPeersExcluded = "all_peers_excluded"
)

// auditResult is the response envelope.
//
// It mirrors pageissues.pageIssuesResult (total/by_severity/sections/
// checks_completed/checks_skipped) because that is the closest existing
// analogue and reusing its shape keeps the two aggregatable by any caller that
// already understands one of them.
type auditResult struct {
	TotalFindings   int            `json:"total_findings"`
	BySeverity      map[string]int `json:"by_severity"`
	Sections        map[string]any `json:"sections"`
	ChecksCompleted []string       `json:"checks_completed"`
	ChecksSkipped   []skipped      `json:"checks_skipped"`
	Selector        string         `json:"selector"`
	ElementsAudited int            `json:"elements_audited"`
	Truncated       bool           `json:"truncated"`
	MatchCount      int            `json:"match_count"`
	PageURL         string         `json:"page_url,omitempty"`
}

// buildAuditResult assembles the envelope from per-category findings.
func buildAuditResult(selector string, elements []elementView, matchCount int, truncated bool,
	byCategory map[string][]finding, skips []skipped) auditResult {

	sections := make(map[string]any, len(byCategory))
	completed := make([]string, 0, len(byCategory))
	bySeverity := make(map[string]int)
	total := 0

	for _, category := range allCategories() {
		findings, ran := byCategory[category]
		if !ran {
			continue
		}
		if findings == nil {
			findings = []finding{}
		}
		sortFindings(findings)
		sections[category] = map[string]any{"findings": findings, "total": len(findings)}
		completed = append(completed, category)
		total += len(findings)
		for _, f := range findings {
			bySeverity[f.Severity]++
		}
	}
	if skips == nil {
		skips = []skipped{}
	}

	return auditResult{
		TotalFindings:   total,
		BySeverity:      bySeverity,
		Sections:        sections,
		ChecksCompleted: completed,
		ChecksSkipped:   skips,
		Selector:        selector,
		ElementsAudited: len(elements),
		Truncated:       truncated,
		MatchCount:      matchCount,
	}
}

// sortFindings gives the response a stable order: errors before warnings, then
// by element, then by property. Without this the map iteration underneath the
// analyzers would reorder findings between identical runs and make the UAT's
// expected-findings comparison flaky for no reason.
func sortFindings(findings []finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Severity != b.Severity {
			return a.Severity == severityError
		}
		if a.ElementIndex != b.ElementIndex {
			return a.ElementIndex < b.ElementIndex
		}
		return a.Property < b.Property
	})
}
