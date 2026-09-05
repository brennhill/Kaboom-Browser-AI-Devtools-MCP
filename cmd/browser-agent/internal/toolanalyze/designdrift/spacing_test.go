// spacing_test.go — Inter-sibling gap hazards for the spacing analyzer (#695).
//
// Every test here is a false-positive hazard first and a detection check
// second. An analyzer that flags everything detects all three issues perfectly
// and is useless, so the controls are the part that constrains the design. The
// wrapped-layout, container-boundary and no-mode cases are the three defects of
// kaboom-tmif, and each is paired with the control that stops its fix from
// becoming a blanket refusal to report.

package designdrift

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/styleprobe"
)

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

// boxedElement places an element at an explicit rectangle, which is what the
// wrapped-layout cases need: stackedElement pins every element to the same
// column, and a grid is exactly the case where that assumption is false.
func boxedElement(index int, selector string, left, top, width, height float64) elementView {
	return elementView{
		Selector: selector,
		Index:    index,
		Styles:   map[string]string{},
		InFlow:   true,
		Box: styleprobe.WireStyleProbeBox{
			Left: left, Right: left + width, Width: width,
			Top: top, Bottom: top + height, Height: height,
		},
	}
}

// verticalStack builds a single column of 32px-tall cards separated by the
// given gaps, so a case can state the rhythm it means rather than the
// coordinates that produce it.
func verticalStack(gaps ...float64) []elementView {
	els := []elementView{stackedElement(0, "div.card", 0, 32)}
	top := 32.0
	for i, gap := range gaps {
		top += gap
		els = append(els, stackedElement(i+1, "div.card", top, 32))
		top += 32
	}
	return els
}

// cardGrid builds a wrapped grid of identically-sized cards in row-major order,
// which is the order a real DOM reports them in.
func cardGrid(lefts, tops []float64, width, height float64) []elementView {
	els := make([]elementView, 0, len(lefts)*len(tops))
	for _, top := range tops {
		for _, left := range lefts {
			els = append(els, boxedElement(len(els), "div.grid-card", left, top, width, height))
		}
	}
	return els
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

	within, skip := analyzeSpacing(verticalStack(24, 24, 26), nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(within) != 0 {
		t.Errorf("a 2px deviation is inside gapDeviationTolerance but produced %d finding(s): %+v", len(within), within)
	}

	beyond, skip := analyzeSpacing(verticalStack(24, 24, 27), nil)
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

// --- Wrapped layouts: the group is not always one line (kaboom-tmif) ---

// TestAnalyzeSpacing_WrappedGridInventsNoOverlaps is the headline false
// positive: a plain 3x2 card grid, perfectly even in both directions, reported
// three overlaps at high confidence. Summing position deltas over the flat
// element set let the column jumps outvote the row jumps, so the whole grid was
// sorted by Left and gaps were measured between cards in the same column on
// different rows — cards that never touch.
func TestAnalyzeSpacing_WrappedGridInventsNoOverlaps(t *testing.T) {
	t.Parallel()
	elements := cardGrid([]float64{0, 324, 648}, []float64{0, 224}, 300, 200)

	findings, skip := analyzeSpacing(elements, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Fatalf("an evenly-spaced 3x2 grid produced %d finding(s): %+v", len(findings), findings)
	}
}

// TestAnalyzeSpacing_WrappedChipRowInventsNoOverlaps is the same hazard with
// ragged widths, which is what a tag or chip row actually looks like: the lines
// do not share column boundaries, so nothing can be recovered by sorting.
func TestAnalyzeSpacing_WrappedChipRowInventsNoOverlaps(t *testing.T) {
	t.Parallel()
	elements := []elementView{
		boxedElement(0, "span.chip", 0, 0, 80, 32),
		boxedElement(1, "span.chip", 104, 0, 120, 32),
		boxedElement(2, "span.chip", 248, 0, 60, 32),
		boxedElement(3, "span.chip", 0, 56, 90, 32),
		boxedElement(4, "span.chip", 114, 56, 70, 32),
	}

	findings, skip := analyzeSpacing(elements, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Fatalf("an evenly-spaced wrapped chip row produced %d finding(s): %+v", len(findings), findings)
	}
}

// TestAnalyzeSpacing_WrappedGridStillCatchesRealDrift is the control on the fix
// above. Suppressing the phantom overlaps by declining to measure wrapped
// layouts at all would pass that test and detect nothing, so the same grid with
// one squeezed column gap must still produce exactly one finding, on the right
// axis and the right element.
func TestAnalyzeSpacing_WrappedGridStillCatchesRealDrift(t *testing.T) {
	t.Parallel()
	elements := cardGrid([]float64{0, 324, 648}, []float64{0, 224}, 300, 200)
	// Pull the last card of the second row 10px left: its column gap is 14px
	// where every other column gap is 24px. Its row gap is untouched.
	elements[5] = boxedElement(5, "div.grid-card", 638, 224, 300, 200)

	findings, skip := analyzeSpacing(elements, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly the squeezed column gap, got %d: %+v", len(findings), findings)
	}
	if findings[0].Property != "gap-horizontal" {
		t.Errorf("property = %q, want gap-horizontal — a column gap in a grid runs sideways", findings[0].Property)
	}
	if findings[0].ElementIndex != 5 {
		t.Errorf("blamed element %d, want 5", findings[0].ElementIndex)
	}
	if findings[0].Observed != "14px" || findings[0].Expected != "24px" {
		t.Errorf("observed/expected = %s/%s, want 14px/24px", findings[0].Observed, findings[0].Expected)
	}
}

// TestAnalyzeSpacing_RowAndColumnRhythmsAreJudgedSeparately: a grid's column gap
// and its row gap are two independent design decisions. Pooling them into one
// rhythm makes whichever is rarer look like drift from the other, so a grid with
// 24px columns and 48px rows would report every row gap as wrong.
func TestAnalyzeSpacing_RowAndColumnRhythmsAreJudgedSeparately(t *testing.T) {
	t.Parallel()
	elements := cardGrid([]float64{0, 324, 648}, []float64{0, 248}, 300, 200)

	findings, skip := analyzeSpacing(elements, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Fatalf("a grid with 24px columns and 48px rows produced %d finding(s): %+v", len(findings), findings)
	}
}

// --- Container boundaries (kaboom-tmif) ---

// TestAnalyzeSpacing_SectionBreakIsNotDrift: two sections of three cards, each
// with a perfect 24px internal rhythm, separated by 120px of section padding.
// The 120px is not a sibling gap at all — it belongs to the section's own box —
// and reporting it blames "the element's own margin" for spacing that margin
// does not control.
func TestAnalyzeSpacing_SectionBreakIsNotDrift(t *testing.T) {
	t.Parallel()
	findings, skip := analyzeSpacing(verticalStack(24, 24, 120, 24, 24), nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Fatalf("the break between two sections was reported as drift: %+v", findings)
	}
}

// TestAnalyzeSpacing_DriftInsideASectionSurvivesTheSectionSplit is the control
// on the split above: dropping the section break must not also drop the real
// outlier that lives inside one of the sections.
func TestAnalyzeSpacing_DriftInsideASectionSurvivesTheSectionSplit(t *testing.T) {
	t.Parallel()
	findings, skip := analyzeSpacing(verticalStack(24, 24, 120, 24, 14), nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 1 {
		t.Fatalf("expected the squeezed gap inside the second section, got %d: %+v", len(findings), findings)
	}
	if findings[0].ElementIndex != 5 {
		t.Errorf("blamed element %d, want 5", findings[0].ElementIndex)
	}
	if findings[0].Observed != "14px" {
		t.Errorf("observed = %s, want 14px", findings[0].Observed)
	}
}

// TestAnalyzeSpacing_ADoubledMarginIsStillDrift pins the container-break bar
// from the other side. A gap of twice the rhythm is the classic doubled margin,
// not a section boundary, and a split that swallowed it would trade one false
// positive for a false negative.
func TestAnalyzeSpacing_ADoubledMarginIsStillDrift(t *testing.T) {
	t.Parallel()
	findings, skip := analyzeSpacing(verticalStack(24, 24, 48, 24), nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 1 {
		t.Fatalf("expected the doubled gap, got %d: %+v", len(findings), findings)
	}
	if findings[0].Observed != "48px" {
		t.Errorf("observed = %s, want 48px", findings[0].Observed)
	}
}

// --- No mode is not a rhythm (kaboom-tmif) ---

// TestAnalyzeSpacing_NoModeInventsNoRhythm: a deliberate escalating scale has no
// repeated gap at all. Seeding the mode search with the smallest value made the
// smallest gap the "rhythm" with a count of one, and every other gap then
// deviated from a rhythm that exists nowhere on the page.
func TestAnalyzeSpacing_NoModeInventsNoRhythm(t *testing.T) {
	t.Parallel()
	findings, skip := analyzeSpacing(verticalStack(12, 20, 32, 48), nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Fatalf("an escalating scale with no repeated gap produced %d finding(s): %+v", len(findings), findings)
	}
}

// TestAnalyzeSpacing_EvenlySplitGapsAreNotDrift mirrors the rule
// inferredFindings already applies to computed styles: two evenly-split variants
// are a design choice, and calling either one wrong is arbitrary. Spacing must
// not judge a split that style consistency refuses to judge.
func TestAnalyzeSpacing_EvenlySplitGapsAreNotDrift(t *testing.T) {
	t.Parallel()
	findings, skip := analyzeSpacing(verticalStack(24, 24, 30, 30), nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Fatalf("an even 2/2 split of gap sizes has no rhythm but produced %d finding(s): %+v", len(findings), findings)
	}
}

// TestAnalyzeSpacing_ABareMajorityIsStillARhythm pins the strict-majority bar
// from the other side, so the guard cannot be widened into "every gap must
// agree" without failing.
func TestAnalyzeSpacing_ABareMajorityIsStillARhythm(t *testing.T) {
	t.Parallel()
	findings, skip := analyzeSpacing(verticalStack(24, 24, 24, 30, 30), nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 2 {
		t.Fatalf("3 of 5 gaps is a strict majority; expected the two 30px gaps flagged, got %d: %+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Observed != "30px" || f.Expected != "24px" {
			t.Errorf("observed/expected = %s/%s, want 30px/24px", f.Observed, f.Expected)
		}
	}
}

// TestModalGap_ReportsItsOwnSupport: the count is how a caller tells a real
// rhythm from a set of distinct values, so a mode of one must say so rather than
// present the smallest value as the norm, and an empty set must not index into
// an empty slice.
func TestModalGap_ReportsItsOwnSupport(t *testing.T) {
	t.Parallel()
	if size, count := modalGap(nil); size != 0 || count != 0 {
		t.Errorf("modalGap(nil) = (%v, %d), want (0, 0)", size, count)
	}
	if size, count := modalGap([]siblingGap{{size: 12}, {size: 20}, {size: 32}}); count != 1 {
		t.Errorf("modalGap over distinct sizes = (%v, %d), want a count of 1 so the caller can reject it", size, count)
	}
}

// TestAnalyzeSpacing_DeclaredScaleDoesNotQuoteAPhantomRhythm: on the declared
// path the verdict comes from the spec, not from a majority, so the evidence
// must name the spec. Quoting "1 of 4 gaps measure 12px" hands a reviewer a
// rhythm the page does not have as the justification for the call.
func TestAnalyzeSpacing_DeclaredScaleDoesNotQuoteAPhantomRhythm(t *testing.T) {
	t.Parallel()
	spec := &designSpec{SpacingScale: []float64{8, 16, 32}}

	findings, skip := analyzeSpacing(verticalStack(12, 20, 32, 48), spec)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	// 12, 20 and 48 are off the scale; 32 is on it. A declared spec is judged
	// per gap, so the absence of a rhythm must not disable it.
	if len(findings) != 3 {
		t.Fatalf("expected the three off-scale gaps, got %d: %+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Severity != severityError {
			t.Errorf("a declared-scale violation is an error, got %q", f.Severity)
		}
		if !strings.Contains(f.Evidence, "declared") {
			t.Errorf("declared-scale evidence should name the spec, got %q", f.Evidence)
		}
		if strings.Contains(f.Evidence, "measure") {
			t.Errorf("declared-scale evidence quotes a rhythm the page does not have: %q", f.Evidence)
		}
	}
}
