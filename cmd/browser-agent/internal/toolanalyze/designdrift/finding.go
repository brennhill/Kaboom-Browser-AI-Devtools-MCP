// finding.go — The one finding shape and response envelope all design analyzers emit.
//
// PURPOSE: give style_consistency, design_tokens and spacing a single vocabulary
// so their results aggregate, sort and triage together.
//
// CONTRACT: three analyzers writing three shapes is the divergence the DRY
// checklist exists to prevent, and it would make the categories impossible to
// score against each other. Every analyzer returns []finding and nothing else.

package designdrift

import (
	"fmt"
	"sort"
	"strings"
)

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
	// TotalFindings is every finding the analyzers produced, not just the ones
	// this page carries. A census that shrank with the page size would make
	// "how much drift does this page have?" unanswerable in one call.
	TotalFindings int `json:"total_findings"`
	// ReturnedFindings is how many findings this payload actually contains.
	ReturnedFindings int `json:"returned_findings"`
	// NextOffset is the offset that reaches the findings this page left behind.
	// Absent when the response is complete.
	NextOffset      int            `json:"next_offset,omitempty"`
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

// findingWindow is the slice of each section's findings one response carries.
//
// The mode asks the page for up to maxProbeElements elements, and a page whose
// cards each break one rule produces several findings per element, so an
// unbounded envelope crossed mcp.MaxResponseBytes at around 50 elements: 200
// elements measured 588KB, of which ClampResponseSize kept 46KB — it truncates
// the JSON mid-string, so the other 92% was neither readable nor recoverable,
// and the clamp note told the caller to page with parameters this mode did not
// accept. Bounding the sections here is what makes every finding reachable:
// total_findings still reports the census, and next_offset names the call that
// returns the rest.
type findingWindow struct {
	limit  int
	offset int
}

const (
	// defaultFindingsPerSection mirrors pageissues.pageIssuesPerSectionCap. The
	// envelope was copied from that mode's shape; this is the bound that shape
	// came with.
	defaultFindingsPerSection = 50
	// maxFindingsPerSection is the ceiling a caller may ask for. Three sections
	// of 50 findings measure roughly 63KB, which clears the 100KB clamp with
	// room for the envelope; letting a caller ask for more would put the
	// silent-truncation failure back within reach of a single call.
	maxFindingsPerSection = 50
)

// normalizeWindow turns caller input into a window that cannot exceed the cap.
func normalizeWindow(limit, offset int) findingWindow {
	if limit <= 0 || limit > maxFindingsPerSection {
		limit = defaultFindingsPerSection
	}
	if offset < 0 {
		offset = 0
	}
	return findingWindow{limit: limit, offset: offset}
}

// page returns the findings this window exposes and whether any remain behind it.
func (w findingWindow) page(findings []finding) ([]finding, bool) {
	if w.offset >= len(findings) {
		return []finding{}, false
	}
	end := w.offset + w.limit
	if end >= len(findings) {
		return findings[w.offset:], false
	}
	return findings[w.offset:end], true
}

// auditInputs is everything the envelope is assembled from: what was probed,
// what each category found, and how much of it this response carries.
type auditInputs struct {
	selector   string
	elements   []elementView
	matchCount int
	truncated  bool
	byCategory map[string][]finding
	skips      []skipped
	window     findingWindow
}

// buildAuditResult assembles the envelope from per-category findings.
func buildAuditResult(in auditInputs) auditResult {
	byCategory := reconcileAcrossCategories(in.byCategory)
	window, skips := in.window, in.skips

	sections := make(map[string]any, len(byCategory))
	completed := make([]string, 0, len(byCategory))
	bySeverity := make(map[string]int)
	total, returned, nextOffset := 0, 0, 0

	for _, category := range allCategories() {
		raw, ran := byCategory[category]
		if !ran {
			continue
		}
		findings := collapseShorthandDuplicates(raw)
		sortFindings(findings)
		shown, more := window.page(findings)
		sections[category] = map[string]any{
			"findings": shown, "total": len(findings),
			"returned": len(shown), "offset": window.offset, "has_more": more,
		}
		completed = append(completed, category)
		total += len(findings)
		returned += len(shown)
		if more {
			nextOffset = window.offset + window.limit
		}
		for _, f := range findings {
			bySeverity[f.Severity]++
		}
	}
	if skips == nil {
		skips = []skipped{}
	}

	return auditResult{
		TotalFindings:    total,
		ReturnedFindings: returned,
		NextOffset:       nextOffset,
		BySeverity:       bySeverity,
		Sections:         sections,
		ChecksCompleted:  completed,
		ChecksSkipped:    skips,
		Selector:         in.selector,
		ElementsAudited:  len(in.elements),
		Truncated:        in.truncated,
		MatchCount:       in.matchCount,
	}
}

// --- Reconciliation across categories ---

// gapProducingMargin names the margin longhand on the same element that
// produces a measured gap, which is what makes two findings from two categories
// findings about the same bytes rather than a coincidence of equal numbers.
func gapProducingMargin(property string) (string, bool) {
	switch property {
	case "gap-vertical":
		return "margin-top", true
	case "gap-horizontal":
		return "margin-left", true
	}
	return "", false
}

// reconcileAcrossCategories drops a token near-miss that a measured gap about
// the same value already answers.
//
// The two analyzers could describe the same 14px and disagree about the fix. On
// the shipped fixture the DEFAULT call returned both `[design_tokens]
// margin-top idx=3 observed=14px expected=--spacing-md (16px) confidence=high`
// and `[spacing] gap-vertical idx=3 observed=14px expected=24px
// confidence=low`. 24px is the right answer — nearestLengthToken picked 16px
// only because 14 sits inside the 15% band of 16 and outside the band of 24 —
// so an agent triaging by confidence applied the target that makes the page
// worse.
//
// Proximity loses. A rhythm is measured from what the page renders across the
// element's own peers; a near-miss is the analyzer guessing that the author
// reached for a token and mistyped. Both analyzers read the same probe, so a
// declared spec makes both verdicts declared at once and the precedence never
// inverts. The rejected expectation is folded into the survivor's evidence
// rather than dropped, so a reviewer can still see what was considered.
func reconcileAcrossCategories(byCategory map[string][]finding) map[string][]finding {
	spacing, hasSpacing := byCategory[categorySpacing]
	tokens, hasTokens := byCategory[categoryDesignTokens]
	if !hasSpacing || !hasTokens || len(spacing) == 0 || len(tokens) == 0 {
		return byCategory
	}

	measured := make([]finding, len(spacing))
	copy(measured, spacing)
	superseded := make(map[int]bool, len(tokens))

	for i := range measured {
		margin, addressable := gapProducingMargin(measured[i].Property)
		if !addressable {
			continue
		}
		for j, guess := range tokens {
			if superseded[j] || guess.Property != margin ||
				guess.ElementIndex != measured[i].ElementIndex || guess.Observed != measured[i].Observed {
				continue
			}
			superseded[j] = true
			measured[i].Evidence = fmt.Sprintf("%s; supersedes a %s near-miss on %s that expected %s",
				measured[i].Evidence, categoryDesignTokens, guess.Property, guess.Expected)
		}
	}

	kept := make([]finding, 0, len(tokens))
	for j, guess := range tokens {
		if !superseded[j] {
			kept = append(kept, guess)
		}
	}

	out := make(map[string][]finding, len(byCategory))
	for category, findings := range byCategory {
		out[category] = findings
	}
	out[categorySpacing] = measured
	out[categoryDesignTokens] = kept
	return out
}

// --- Shorthand collapse ---

// boxShorthand is a CSS box shorthand and the longhands it writes, in CSS order.
type boxShorthand struct {
	name      string
	longhands []string
}

// boxShorthands() are the shorthands whose longhands the token analyzer judges
// one at a time. A function rather than a package var so the table cannot be
// mutated at runtime.
func boxShorthands() []boxShorthand {
	return []boxShorthand{
		{"padding", []string{"padding-top", "padding-right", "padding-bottom", "padding-left"}},
		{"margin", []string{"margin-top", "margin-right", "margin-bottom", "margin-left"}},
	}
}

// collapseShorthandDuplicates folds the four byte-identical findings a single
// `padding: 15px` produces into the one edit that fixes them.
//
// The probe reports longhands because `padding: 15px 16px` really is two
// values, so each side has to be judged on its own. But when all four sides
// carry the same verdict the author wrote one declaration, and reporting it four
// times multiplies every such element by four: forty identical cards with one
// broken rule produced 200 findings for one edit, which is most of what pushed
// the response past the response clamp.
//
// Only the complete, uniform group collapses. A partial group — `padding: 15px
// 16px` drifting on two sides — keeps its longhands, because "padding is 15px"
// would be false about the other two.
func collapseShorthandDuplicates(findings []finding) []finding {
	if len(findings) == 0 {
		return []finding{}
	}
	collapseTo := make(map[int]string, len(findings))
	drop := make(map[int]bool, len(findings))
	for _, shorthand := range boxShorthands() {
		for _, group := range uniformShorthandGroups(findings, shorthand) {
			collapseTo[group[0]] = shorthand.name
			for _, index := range group[1:] {
				drop[index] = true
			}
		}
	}

	out := make([]finding, 0, len(findings))
	for i, f := range findings {
		if drop[i] {
			continue
		}
		if name, collapsed := collapseTo[i]; collapsed {
			f.Message = strings.Replace(f.Message, f.Property, name, 1)
			f.Property = name
		}
		out = append(out, f)
	}
	return out
}

// uniformShorthandGroups returns the index sets that cover every side of one
// shorthand on one element with one identical verdict.
func uniformShorthandGroups(findings []finding, shorthand boxShorthand) [][]int {
	type verdictKey struct {
		element  int
		category string
		verdict  string
	}
	members := make(map[verdictKey][]int)
	order := make([]verdictKey, 0, len(findings))
	for i, f := range findings {
		if !namesOneOf(shorthand.longhands, f.Property) {
			continue
		}
		key := verdictKey{f.ElementIndex, f.Category, strings.Join(
			[]string{f.Observed, f.Expected, f.Severity, f.ExpectedFrom, f.Confidence, f.Evidence}, "\x00")}
		if _, seen := members[key]; !seen {
			order = append(order, key)
		}
		members[key] = append(members[key], i)
	}

	var groups [][]int
	for _, key := range order {
		// One element emits each longhand at most once per category, so a group
		// the size of the shorthand covers every side of it.
		if len(members[key]) == len(shorthand.longhands) {
			groups = append(groups, members[key])
		}
	}
	return groups
}

func namesOneOf(candidates []string, property string) bool {
	for _, candidate := range candidates {
		if candidate == property {
			return true
		}
	}
	return false
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
