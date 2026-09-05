// analyzers_test.go — Style-consistency (#693) hazards.
//
// Every test here is a false-positive hazard first and a detection check
// second. An analyzer that flags everything detects all three issues perfectly
// and is useless, so the controls are the part that constrains the design.
// The spacing analyzer's hazards live in spacing_test.go, which grew past the
// 800-line file limit while sharing this file.

package designdrift

import (
	"strings"
	"testing"
)

// makeElement builds an in-flow element view for the analyzers.
func makeElement(index int, selector string, styles map[string]string) elementView {
	return elementView{Selector: selector, Index: index, Styles: styles, InFlow: true}
}

func header(index, size int, family string) elementView {
	return makeElement(index, "p.step-card__header", map[string]string{
		"font-family": family,
		"font-size":   itoaPx(size),
		"font-weight": "600",
		"line-height": "20px",
		"color":       "rgb(17, 24, 39)",
	})
}

func itoaPx(v int) string { return formatPx(float64(v)) }

// TestAnalyzeConsistency_DetectsTheIssueExample is GitHub #693 verbatim.
func TestAnalyzeConsistency_DetectsTheIssueExample(t *testing.T) {
	t.Parallel()
	findings, skip := analyzeConsistency([]elementView{
		header(0, 12, "Inter, sans-serif"),
		header(1, 11, "Roboto, sans-serif"),
		header(2, 12, "Inter, sans-serif"),
	}, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}

	if !equalStringSets(propertiesOf(findings), []string{"font-family", "font-size"}) {
		t.Fatalf("flagged %v, want exactly font-family and font-size", propertiesOf(findings))
	}
	for _, f := range findings {
		if f.ElementIndex != 1 {
			t.Errorf("finding blames element %d; the odd header is element 1: %+v", f.ElementIndex, f)
		}
		if f.Severity != severityWarning {
			t.Errorf("an inferred majority is a warning, got %q", f.Severity)
		}
		if f.ExpectedFrom != provenanceInferred {
			t.Errorf("expected provenance inferred, got %q", f.ExpectedFrom)
		}
		if !strings.Contains(f.Evidence, "2 of 3") {
			t.Errorf("evidence should quote the majority it relied on, got %q", f.Evidence)
		}
	}
}

// TestAnalyzeConsistency_ConfidenceTracksMajorityStrength: 9 of 10 is not 2 of 3.
func TestAnalyzeConsistency_ConfidenceTracksMajorityStrength(t *testing.T) {
	t.Parallel()

	weak := []elementView{header(0, 12, "Inter, sans-serif"), header(1, 11, "Roboto, sans-serif"), header(2, 12, "Inter, sans-serif")}
	weakFindings, _ := analyzeConsistency(weak, nil)
	for _, f := range weakFindings {
		if f.Confidence != confidenceLow {
			t.Errorf("a 2-of-3 majority should be low confidence, got %q", f.Confidence)
		}
	}

	strong := make([]elementView, 0, 10)
	for i := 0; i < 9; i++ {
		strong = append(strong, header(i, 12, "Inter, sans-serif"))
	}
	strong = append(strong, header(9, 11, "Roboto, sans-serif"))
	strongFindings, _ := analyzeConsistency(strong, nil)
	if len(strongFindings) == 0 {
		t.Fatal("a 9-of-10 majority found no outlier")
	}
	for _, f := range strongFindings {
		if f.Confidence != confidenceHigh {
			t.Errorf("a 9-of-10 majority should be high confidence, got %q", f.Confidence)
		}
	}
}

// TestAnalyzeConsistency_TwoElementsHaveNoMajority: with two values, each is as
// likely to be right, so a verdict would be a coin flip presented as a fact.
func TestAnalyzeConsistency_TwoElementsHaveNoMajority(t *testing.T) {
	t.Parallel()
	findings, skip := analyzeConsistency([]elementView{
		header(0, 12, "Inter, sans-serif"),
		header(1, 11, "Roboto, sans-serif"),
	}, nil)
	if len(findings) != 0 {
		t.Errorf("a two-element group produced %d finding(s): %+v", len(findings), findings)
	}
	if skip == nil || skip.Reason != reasonInsufficientPeers {
		t.Fatalf("expected an insufficient_peers skip, got %+v", skip)
	}
}

// TestAnalyzeConsistency_StateVariantsAreNotDrift covers the .active hazard.
func TestAnalyzeConsistency_StateVariantsAreNotDrift(t *testing.T) {
	t.Parallel()
	base := func(index int, selector string) elementView {
		return makeElement(index, selector, map[string]string{
			"font-family": "Inter, sans-serif", "font-size": "13px",
			"font-weight": "400", "color": "rgb(17, 24, 39)",
		})
	}
	active := makeElement(1, "div.state-item.state-item--active", map[string]string{
		"font-family": "Inter, sans-serif", "font-size": "13px",
		"font-weight": "700", "color": "rgb(42, 85, 225)",
	})

	findings, skip := analyzeConsistency([]elementView{
		base(0, "div.state-item"), active, base(2, "div.state-item"), base(3, "div.state-item"),
	}, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Errorf("the deliberately-styled active item was reported as drift: %+v", findings)
	}
}

// TestAnalyzeConsistency_EvenSplitIsNotAnOutlier: two equally-sized variants are
// a design choice, and calling either one wrong is arbitrary.
func TestAnalyzeConsistency_EvenSplitIsNotAnOutlier(t *testing.T) {
	t.Parallel()
	findings, skip := analyzeConsistency([]elementView{
		header(0, 12, "Inter, sans-serif"),
		header(1, 12, "Inter, sans-serif"),
		header(2, 11, "Roboto, sans-serif"),
		header(3, 11, "Roboto, sans-serif"),
	}, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Errorf("an even 2/2 split has no majority but produced %d finding(s): %+v", len(findings), findings)
	}
}

// TestAnalyzeConsistency_DeclaredSpecCatchesAUniformlyWrongPage is the case
// inference cannot reach: when every element is wrong, the majority is wrong.
func TestAnalyzeConsistency_DeclaredSpecCatchesAUniformlyWrongPage(t *testing.T) {
	t.Parallel()
	elements := []elementView{
		header(0, 12, "Comic Sans MS, cursive"),
		header(1, 12, "Comic Sans MS, cursive"),
		header(2, 12, "Inter, sans-serif"),
	}
	spec := &designSpec{FontFamilies: []string{"Inter"}}

	findings, skip := analyzeConsistency(elements, spec)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 2 {
		t.Fatalf("expected both Comic Sans elements flagged against the declared font, got %d: %+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Severity != severityError {
			t.Errorf("breaking a declared rule is an error, got %q", f.Severity)
		}
		if f.ExpectedFrom != provenanceDeclared {
			t.Errorf("expected declared provenance, got %q", f.ExpectedFrom)
		}
		if f.ElementIndex == 2 {
			t.Error("element 2 matches the declared font and must not be flagged for disagreeing with its drifted peers")
		}
	}
}

// TestAnalyzeConsistency_AuditsColour keeps `color` in the audited set. It is
// the one audited property with no dedicated case, so dropping it from
// auditedProperties() changed nothing that any test observed.
func TestAnalyzeConsistency_AuditsColour(t *testing.T) {
	t.Parallel()
	peer := func(i int, colour string) elementView {
		return makeElement(i, "p.label", map[string]string{
			"font-family": "Inter, sans-serif", "font-size": "13px",
			"font-weight": "400", "line-height": "20px", "color": colour,
		})
	}
	findings, skip := analyzeConsistency([]elementView{
		peer(0, "rgb(17, 24, 39)"), peer(1, "rgb(17, 24, 39)"),
		peer(2, "rgb(200, 30, 30)"), peer(3, "rgb(17, 24, 39)"),
	}, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 1 || findings[0].Property != "color" {
		t.Fatalf("expected one colour finding, got %+v", findings)
	}
	if findings[0].ElementIndex != 2 {
		t.Errorf("blamed element %d, want 2", findings[0].ElementIndex)
	}
}

// TestTieBreaksAreDeterministicOnTheLowestValue pins both majority tie-breaks.
// With counts equal, `>` keeps the first candidate in sorted order; `>=` would
// keep the last. Either is defensible, but the choice must be fixed or the
// reported "expected" value flips between runs of identical input.
func TestTieBreaksAreDeterministicOnTheLowestValue(t *testing.T) {
	t.Parallel()
	value, count := dominantValue(map[string]int{"beta": 2, "alpha": 2})
	if value != "alpha" || count != 2 {
		t.Errorf("dominantValue tie = (%q, %d), want (alpha, 2) — the first value in sorted order", value, count)
	}

	gap, gapCount := modalGap([]siblingGap{{size: 24}, {size: 16}, {size: 24}, {size: 16}})
	if gap != 16 || gapCount != 2 {
		t.Errorf("modalGap tie = (%v, %d), want (16, 2) — the smallest gap in sorted order", gap, gapCount)
	}
}

// TestClassMarksState_MatchesStateNamesNotStateWords pins all three shapes the
// exclusion recognises and, more importantly, the ordinary component names it
// must leave alone. Substring matching silently deleted every one of the
// negatives below from every audit.
func TestClassMarksState_MatchesStateNamesNotStateWords(t *testing.T) {
	t.Parallel()
	states := []string{
		"active", "selected", "disabled", // bare state word
		"tab--active", "card--selected", "pricing-card--primary", // BEM modifier
		"is-active", "is-open", "has-error", // stateful prefix
	}
	for _, class := range states {
		if !classMarksState(class) {
			t.Errorf("%q names a state and should be excluded from the peer group", class)
		}
	}

	components := []string{
		"error-message", "success-banner", "open-hours", "interactive-tile",
		"featured-post", "focus-area", "current-balance", "danger-zone",
		"checked-baggage", "readonly-viewer", "highlight-reel", "expanded-content",
		"is-drifted", "is-squeezed", // the fixture's own defect markers
	}
	for _, class := range components {
		if classMarksState(class) {
			t.Errorf("%q is an ordinary component name; excluding it deletes real drift from the audit", class)
		}
	}
}

// TestEligiblePeers_KeepsComponentsWhoseNamesContainStateWords is the same rule
// at the analyzer boundary: drift inside .error-message peers must be found.
func TestEligiblePeers_KeepsComponentsWhoseNamesContainStateWords(t *testing.T) {
	t.Parallel()
	peer := func(i int, family, size string) elementView {
		return makeElement(i, "div.error-message", map[string]string{
			"font-family": family, "font-size": size,
			"font-weight": "400", "line-height": "20px", "color": "rgb(200, 30, 30)",
		})
	}
	findings, skip := analyzeConsistency([]elementView{
		peer(0, "Inter, sans-serif", "14px"), peer(1, "Inter, sans-serif", "14px"),
		peer(2, "Roboto, sans-serif", "11px"),
		peer(3, "Inter, sans-serif", "14px"),
	}, nil)
	if skip != nil {
		t.Fatalf("unexpected skip %+v — the whole group was excluded by its block name", skip)
	}
	if !equalStringSets(propertiesOf(findings), []string{"font-family", "font-size"}) {
		t.Fatalf("flagged %v, want the Roboto/11px outlier's two properties", propertiesOf(findings))
	}
	for _, f := range findings {
		if f.ElementIndex != 2 {
			t.Errorf("blamed element %d, want 2", f.ElementIndex)
		}
	}
}
