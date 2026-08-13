// analyzers_test.go — Style-consistency (#693) and spacing (#695) hazards.
//
// Every test here is a false-positive hazard first and a detection check
// second. An analyzer that flags everything detects all three issues perfectly
// and is useless, so the controls are the part that constrains the design.

package designdrift

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/styleprobe"
)

// makeElement builds an in-flow element view for the analyzers.
func makeElement(index int, selector string, styles map[string]string) elementView {
	return elementView{Selector: selector, Index: index, Styles: styles, InFlow: true}
}

// stackedElement places an element in a vertical stack at a known offset.
func stackedElement(index int, selector string, top, height float64) elementView {
	return elementView{
		Selector: selector,
		Index:    index,
		Styles:   map[string]string{},
		InFlow:   true,
		Box: styleprobe.WireStyleProbeBox{
			Top: top, Bottom: top + height, Height: height,
			Left: 0, Right: 200, Width: 200,
		},
	}
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

// TestAnalyzeSpacing_DetectsTheIssueExample is GitHub #695: 24/24/14/24.
func TestAnalyzeSpacing_DetectsTheIssueExample(t *testing.T) {
	t.Parallel()
	// Cards 32px tall at gaps 24, 24, 14, 24.
	elements := []elementView{
		stackedElement(0, "div.rhythm-card", 0, 32),
		stackedElement(1, "div.rhythm-card", 56, 32),
		stackedElement(2, "div.rhythm-card", 112, 32),
		stackedElement(3, "div.rhythm-card.is-squeezed", 158, 32),
		stackedElement(4, "div.rhythm-card", 214, 32),
	}

	findings, skip := analyzeSpacing(elements, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly the squeezed gap, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.ElementIndex != 3 {
		t.Errorf("the finding blames element %d; the 14px gap sits before element 3", f.ElementIndex)
	}
	if f.Observed != "14px" || f.Expected != "24px" {
		t.Errorf("observed/expected = %s/%s, want 14px/24px", f.Observed, f.Expected)
	}
	if f.Property != "gap-vertical" {
		t.Errorf("property = %q, want gap-vertical", f.Property)
	}
	if !strings.Contains(f.Message, "margin") {
		t.Errorf("with no parent gap the message should point at the element's own margin: %q", f.Message)
	}
}

// TestAnalyzeSpacing_ModeNotMean: a single outlier must not drag the norm.
func TestAnalyzeSpacing_ModeNotMean(t *testing.T) {
	t.Parallel()
	// Gaps 24, 24, 14, 24 have a mean of 21.5. Judging against the mean would
	// flag all four gaps and understate the real one.
	gaps := []siblingGap{{size: 24}, {size: 24}, {size: 14}, {size: 24}}
	rhythm, count := modalGap(gaps)
	if rhythm != 24 || count != 3 {
		t.Fatalf("modalGap = (%v, %d), want (24, 3)", rhythm, count)
	}
}

// TestAnalyzeSpacing_OutOfFlowSiblingsCannotManufactureAGap.
func TestAnalyzeSpacing_OutOfFlowSiblingsCannotManufactureAGap(t *testing.T) {
	t.Parallel()
	hidden := stackedElement(2, "div.rhythm-card.is-hidden", 400, 0)
	hidden.InFlow = false

	elements := []elementView{
		stackedElement(0, "div.rhythm-card", 0, 32),
		stackedElement(1, "div.rhythm-card", 56, 32),
		hidden,
		stackedElement(3, "div.rhythm-card", 112, 32),
		stackedElement(4, "div.rhythm-card", 168, 32),
	}

	findings, skip := analyzeSpacing(elements, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Errorf("a display:none sibling created phantom drift: %+v", findings)
	}
}

// TestAnalyzeSpacing_AttributesFlexGapToTheParent: editing a child margin would
// not change spacing the parent's gap property owns.
func TestAnalyzeSpacing_AttributesFlexGapToTheParent(t *testing.T) {
	t.Parallel()
	withParent := func(el elementView) elementView {
		el.ParentDisplay = "flex"
		el.ParentGap = "24px"
		return el
	}
	elements := []elementView{
		withParent(stackedElement(0, "div.flex-item", 0, 32)),
		withParent(stackedElement(1, "div.flex-item", 56, 32)),
		withParent(stackedElement(2, "div.flex-item", 112, 32)),
		withParent(stackedElement(3, "div.flex-item", 158, 32)),
	}

	findings, skip := analyzeSpacing(elements, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 1 {
		t.Fatalf("expected the one uneven gap, got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "parent") {
		t.Errorf("a flex parent owns this spacing but the message points elsewhere: %q", findings[0].Message)
	}
}

// TestAnalyzeSpacing_OverlapIsADistinctFinding: overlapping elements are a
// layout break, not an uneven gap.
func TestAnalyzeSpacing_OverlapIsADistinctFinding(t *testing.T) {
	t.Parallel()
	elements := []elementView{
		stackedElement(0, "div.card", 0, 32),
		stackedElement(1, "div.card", 56, 32),
		stackedElement(2, "div.card", 80, 32), // starts before the previous ends
		stackedElement(3, "div.card", 136, 32),
	}

	findings, skip := analyzeSpacing(elements, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	overlaps := 0
	for _, f := range findings {
		if strings.HasPrefix(f.Property, "overlap-") {
			overlaps++
			if f.ElementIndex != 2 {
				t.Errorf("overlap blamed element %d, want 2", f.ElementIndex)
			}
		}
	}
	if overlaps != 1 {
		t.Errorf("expected exactly one overlap finding, got %d: %+v", overlaps, findings)
	}
}

// TestAnalyzeSpacing_ToleratesSubPixelRounding: transforms and zoom produce
// 23.98px, which is 24px.
func TestAnalyzeSpacing_ToleratesSubPixelRounding(t *testing.T) {
	t.Parallel()
	elements := []elementView{
		stackedElement(0, "div.card", 0, 32),
		stackedElement(1, "div.card", 55.98, 32),
		stackedElement(2, "div.card", 112.03, 32),
		stackedElement(3, "div.card", 167.99, 32),
	}

	findings, skip := analyzeSpacing(elements, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Errorf("sub-pixel rounding was reported as drift: %+v", findings)
	}
}

// TestAnalyzeSpacing_DetectsHorizontalStacks: a row of cards drifts sideways.
func TestAnalyzeSpacing_DetectsHorizontalStacks(t *testing.T) {
	t.Parallel()
	row := func(index int, left float64) elementView {
		return elementView{
			Selector: "div.card", Index: index, Styles: map[string]string{}, InFlow: true,
			Box: styleprobe.WireStyleProbeBox{Left: left, Right: left + 80, Width: 80, Top: 0, Bottom: 40, Height: 40},
		}
	}
	elements := []elementView{row(0, 0), row(1, 104), row(2, 208), row(3, 302)}

	findings, skip := analyzeSpacing(elements, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one horizontal gap finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Property != "gap-horizontal" {
		t.Errorf("property = %q, want gap-horizontal — the axis was misdetected", findings[0].Property)
	}
}

// TestAnalyzeSpacing_TwoSiblingsHaveNoRhythm.
func TestAnalyzeSpacing_TwoSiblingsHaveNoRhythm(t *testing.T) {
	t.Parallel()
	findings, skip := analyzeSpacing([]elementView{
		stackedElement(0, "div.card", 0, 32),
		stackedElement(1, "div.card", 56, 32),
	}, nil)
	if len(findings) != 0 {
		t.Errorf("one gap is its own norm, but %d finding(s) were produced", len(findings))
	}
	if skip == nil || skip.Reason != reasonInsufficientPeers {
		t.Fatalf("expected an insufficient_peers skip, got %+v", skip)
	}
}

// TestAnalyzeSpacing_DeclaredScaleOverridesRhythm covers the precedence rule and
// the severity flip that follows from it.
func TestAnalyzeSpacing_DeclaredScaleOverridesRhythm(t *testing.T) {
	t.Parallel()
	elements := []elementView{
		stackedElement(0, "div.card", 0, 32),
		stackedElement(1, "div.card", 56, 32),
		stackedElement(2, "div.card", 112, 32),
		stackedElement(3, "div.card", 168, 32),
	}
	// Uniform 24px gaps: inference finds nothing, but 24 is off the scale.
	spec := &designSpec{SpacingScale: []float64{8, 16, 32}}

	inferred, _ := analyzeSpacing(elements, nil)
	if len(inferred) != 0 {
		t.Fatalf("uniform gaps should be clean under inference, got %+v", inferred)
	}

	declared, skip := analyzeSpacing(elements, spec)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(declared) != 3 {
		t.Fatalf("every gap breaks the declared scale; expected 3 findings, got %d: %+v", len(declared), declared)
	}
	for _, f := range declared {
		if f.Severity != severityError {
			t.Errorf("a declared-scale violation is an error, got %q", f.Severity)
		}
	}
}

// TestAnalyzeSpacing_AllOutOfFlowIsItsOwnReason keeps two different situations
// distinguishable: a group whose members are all hidden, and a group with too
// few members. It is also how an extension too old to report in_flow presents,
// so collapsing it into insufficient_peers would send someone hunting for
// elements that are present and visible.
func TestAnalyzeSpacing_AllOutOfFlowIsItsOwnReason(t *testing.T) {
	t.Parallel()
	elements := make([]elementView, 0, 4)
	for i := 0; i < 4; i++ {
		el := stackedElement(i, "div.card", float64(i*56), 32)
		el.InFlow = false
		elements = append(elements, el)
	}

	findings, skip := analyzeSpacing(elements, nil)
	if len(findings) != 0 {
		t.Errorf("out-of-flow elements produced findings: %+v", findings)
	}
	if skip == nil || skip.Reason != reasonNoInFlowElements {
		t.Fatalf("skip = %+v, want reason %q", skip, reasonNoInFlowElements)
	}
}

// TestAnalyzeSpacing_GapToleranceIsLoadBearing pins gapDeviationTolerance from
// both sides. TestAnalyzeSpacing_ToleratesSubPixelRounding does not: its inputs
// all round to exactly 24 in measureGaps, so the tolerance is never consulted
// and the constant could be set to zero without failing anything.
func TestAnalyzeSpacing_GapToleranceIsLoadBearing(t *testing.T) {
	t.Parallel()
	stack := func(gaps ...float64) []elementView {
		els := []elementView{stackedElement(0, "div.card", 0, 32)}
		top := 32.0
		for i, g := range gaps {
			top += g
			els = append(els, stackedElement(i+1, "div.card", top, 32))
			top += 32
		}
		return els
	}

	within, skip := analyzeSpacing(stack(24, 24, 26), nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(within) != 0 {
		t.Errorf("a 2px deviation is inside gapDeviationTolerance but produced %d finding(s): %+v", len(within), within)
	}

	beyond, skip := analyzeSpacing(stack(24, 24, 27), nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(beyond) != 1 {
		t.Fatalf("a 3px deviation exceeds gapDeviationTolerance; expected 1 finding, got %d: %+v", len(beyond), beyond)
	}
	if beyond[0].Observed != "27px" {
		t.Errorf("observed = %s, want 27px", beyond[0].Observed)
	}
}

// TestAnalyzeSpacing_OrdersByLayoutNotDOMOrder covers the documented flex
// `order` hazard: DOM order stops being layout order the moment order, RTL or
// absolute placement is involved, and measuring in DOM order then invents
// overlaps between elements that do not touch.
func TestAnalyzeSpacing_OrdersByLayoutNotDOMOrder(t *testing.T) {
	t.Parallel()
	els := []elementView{
		stackedElement(0, "div.card", 168, 32),
		stackedElement(1, "div.card", 0, 32),
		stackedElement(2, "div.card", 112, 32),
		stackedElement(3, "div.card", 56, 32),
	}
	findings, skip := analyzeSpacing(els, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Errorf("a uniform stack in scrambled DOM order produced %d finding(s): %+v", len(findings), findings)
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
