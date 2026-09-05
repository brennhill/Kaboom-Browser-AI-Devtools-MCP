// spacing.go — Inter-sibling gap drift from box geometry (#695).
//
// PURPOSE: catch the card stack whose gaps run 24, 24, 14, 24 because one
// nested margin override squeezed a single card. Functionally perfect, visibly
// wrong.
//
// CONTRACT: gaps are measured from rendered bounding rects, never from declared
// margins. Adjacent vertical margins COLLAPSE to the larger of the two, so
// margin-bottom + margin-top does not equal the rendered gap and never will.
// Anyone "simplifying" this back to margin arithmetic will reintroduce wrong
// verdicts on every page that uses ordinary block layout.
//
// CONTRACT: a gap is only measured between two elements that are actually
// adjacent — neighbours along one axis whose extents overlap on the other. A
// selector match is a flat list, not a line: it can span two rows of a grid,
// several wrapped chip lines, or two sections of a page. Measuring the flat list
// as one run is what produced 300px "overlaps" between cards in the same grid
// column on different rows, and it is what attributed a section's padding to the
// margin of the first card after it.

package designdrift

import (
	"fmt"
	"math"
	"sort"
)

// minimumGapsForRhythm is the smallest number of gaps that can establish a
// rhythm. Two siblings produce one gap, which is its own norm — there is
// nothing to deviate from, so that reports insufficient peers rather than a
// verdict.
const minimumGapsForRhythm = 2

// gapDeviationTolerance is how far a gap may sit from the rhythm before it is
// drift. Wider than subPixelTolerance because real layouts round.
const gapDeviationTolerance = 2.0

// containerBreakRatio is how many times the local rhythm a gap must reach
// before it reads as a boundary between containers rather than drift inside
// one.
//
// The element view carries no parent identity, so contiguity in the geometry is
// the only signal available for "these belong together". Three is deliberately
// high: a doubled margin is 2x the rhythm and is exactly the defect this
// analyzer exists to report, while a section's padding is typically several
// times its internal rhythm. Lowering this trades a false positive for a false
// negative on the more serious of the two.
const containerBreakRatio = 3.0

// minimumGapsForContainerSplit is the smallest run in which a container
// boundary can be identified. With two gaps, "one of them is much larger" reads
// equally well as "one of them is much smaller", and the smaller one is drift.
const minimumGapsForContainerSplit = 3

// stackAxis is which way the siblings run.
type stackAxis int

const (
	axisVertical stackAxis = iota
	axisHorizontal
)

func (a stackAxis) String() string {
	if a == axisHorizontal {
		return "horizontal"
	}
	return "vertical"
}

// measuredAxes() is the order the axes are measured and reported in. Both are
// always measured: a wrapped layout has a rhythm in each direction, and a page
// can drift in one while holding the other.
func measuredAxes() []stackAxis {
	return []stackAxis{axisHorizontal, axisVertical}
}

// siblingGap is the measured space between two adjacent in-flow elements.
type siblingGap struct {
	before elementView
	after  elementView
	size   float64
}

// analyzeSpacing measures the gaps between adjacent siblings and flags the ones
// that break the page's rhythm.
func analyzeSpacing(elements []elementView, spec *designSpec) ([]finding, *skipped) {
	flow := inFlowElements(elements)
	if len(flow) == 0 && len(elements) > 0 {
		return nil, &skipped{Category: categorySpacing, Reason: reasonNoInFlowElements}
	}
	if len(flow) < minimumGapsForRhythm+1 {
		return nil, &skipped{
			Category: categorySpacing,
			Reason:   reasonInsufficientPeers,
		}
	}

	byAxis, total := gapsByAxis(flow)
	if total < minimumGapsForRhythm {
		return nil, &skipped{Category: categorySpacing, Reason: reasonInsufficientPeers}
	}

	var findings []finding
	for _, axis := range measuredAxes() {
		findings = append(findings, axisFindings(byAxis[axis], axis, spec)...)
	}
	return findings, nil
}

// gapsByAxis measures every adjacent pair in the group and files each gap under
// the axis it runs along.
func gapsByAxis(flow []elementView) (map[stackAxis][]siblingGap, int) {
	byAxis := make(map[stackAxis][]siblingGap, len(measuredAxes()))
	total := 0
	for _, run := range spacingRuns(flow) {
		gaps := measureGaps(run.elements, run.axis)
		byAxis[run.axis] = append(byAxis[run.axis], gaps...)
		total += len(gaps)
	}
	return byAxis, total
}

// axisFindings judges one axis's gaps against that axis's own rhythm.
//
// Per axis, not pooled: a grid's column gap and its row gap are two independent
// design decisions, and pooling them makes whichever is rarer look like drift
// from the other.
func axisFindings(gaps []siblingGap, axis stackAxis, spec *designSpec) []finding {
	if len(gaps) == 0 {
		return nil
	}
	findings := overlapFindings(gaps, axis)

	positive := positiveGaps(gaps)
	if len(positive) < minimumGapsForRhythm {
		return findings
	}

	rhythm, rhythmCount := modalGap(positive)
	provenance := spec.provenanceForSpacing()
	declared := provenance == provenanceDeclared
	if !declared && !isRhythm(rhythmCount, len(positive)) {
		// No gap size holds a strict majority, so there is no norm to deviate
		// from. This is the same refusal inferredFindings makes for computed
		// styles: two evenly-split variants are a design choice, and a set of
		// distinct values is a deliberate scale. Calling the smallest of them
		// "the rhythm" would flag every other gap against a norm the page does
		// not have.
		return findings
	}

	expected := formatPx(rhythm)
	if declared {
		expected = formatScale(spec.SpacingScale)
	}

	for _, gap := range positive {
		if declared {
			if scaleContains(spec.SpacingScale, gap.size) {
				continue
			}
		} else if absFloat(gap.size-rhythm) <= gapDeviationTolerance {
			continue
		}
		findings = append(findings, spacingFinding(gap, axis, expected, provenance, gapRhythm{modal: rhythm, count: rhythmCount, total: len(positive)}))
	}
	return findings
}

// isRhythm reports whether the modal gap holds a strict majority of the
// measured gaps, which is what makes it a norm rather than the most common of
// several equals.
func isRhythm(modeCount, measured int) bool {
	return modeCount*2 > measured
}

// gapRhythm summarizes the modal-gap evidence a spacing verdict rests on: the
// modal gap size, how many gaps share it, and how many positive gaps were measured.
type gapRhythm struct {
	modal float64
	count int
	total int
}

// spacingFinding builds one gap finding, attributing the cause to whichever
// rule actually owns the spacing.
func spacingFinding(gap siblingGap, axis stackAxis, expected, provenance string, rhythm gapRhythm) finding {
	confidence := confidenceLow
	if float64(rhythm.count)/float64(rhythm.total) >= strongMajorityRatio {
		confidence = confidenceHigh
	}

	// On the declared path the verdict comes from the spec, so the evidence
	// names the spec. Quoting the modal gap there would offer a rhythm as the
	// justification for a call the rhythm played no part in — and on a page with
	// no mode at all, would invent one ("1 of 4 gaps measure 12px").
	evidence := "declared spec spacing scale"
	if provenance != provenanceDeclared {
		evidence = fmt.Sprintf("%d of %d %s gaps measure %s", rhythm.count, rhythm.total, axis, formatPx(rhythm.modal))
	}

	// A flex or grid parent's gap belongs to no child's margin at all. Naming
	// the child would send an agent editing a rule that does not control this
	// spacing.
	owner := "the element's own margin"
	if isGapOwningParent(gap.after) {
		owner = fmt.Sprintf("the parent's %s gap property", gap.after.ParentDisplay)
	}

	return newFinding(findingSpec{category: categorySpacing, property: "gap-" + axis.String(), el: gap.after,
		observed: formatPx(gap.size), expected: expected, provenance: provenance,
		confidence: confidence, evidence: evidence,
		message: fmt.Sprintf("the %s gap before this element is %s where the rhythm is %s; the spacing is controlled by %s",
			axis, formatPx(gap.size), expected, owner)})
}

// overlapFindings reports negative gaps separately. An overlap is a different
// and more serious defect than an uneven one: the elements are on top of each
// other, which is a layout break rather than a polish issue.
func overlapFindings(gaps []siblingGap, axis stackAxis) []finding {
	var findings []finding
	for _, gap := range gaps {
		if gap.size >= -subPixelTolerance {
			continue
		}
		findings = append(findings, newFinding(findingSpec{category: categorySpacing, property: "overlap-" + axis.String(), el: gap.after,
			observed: formatPx(gap.size), expected: "no overlap", provenance: provenanceInferred, confidence: confidenceHigh,
			evidence: fmt.Sprintf("overlaps %s by %s", gap.before.Selector, formatPx(-gap.size)),
			message:  fmt.Sprintf("this element overlaps the previous one by %s along the %s axis", formatPx(-gap.size), axis)}))
	}
	return findings
}

// inFlowElements drops the siblings that take no part in the rhythm.
//
// An absolutely positioned or display:none node sits anywhere, or nowhere, in
// the geometry; leaving one in manufactures a phantom gap and shifts the modal
// rhythm away from the real one.
func inFlowElements(elements []elementView) []elementView {
	flow := make([]elementView, 0, len(elements))
	for _, el := range elements {
		if el.InFlow {
			flow = append(flow, el)
		}
	}
	return flow
}

// spacingRun is an ordered set of elements that really are adjacent: successive
// along the run's axis, overlapping on the other one, and inside the same
// contiguous block of layout.
type spacingRun struct {
	axis     stackAxis
	elements []elementView
}

// spacingRuns splits the flat element set into the runs whose gaps are
// measurable.
//
// A selector match is a list, not a line. Deciding one axis for the whole list
// by summing position deltas — as this used to — hands a wrapped layout to
// whichever axis has the larger jumps, and the elements then sorted along that
// axis are neighbours in the sort but strangers on the page. Rows and columns
// are derived instead, so each run is a real line of the layout and both
// directions of a grid get measured.
func spacingRuns(flow []elementView) []spacingRun {
	var runs []spacingRun
	for _, axis := range measuredAxes() {
		for _, line := range lineBands(flow, axis) {
			if len(line) < 2 {
				continue
			}
			ordered := orderAlongAxis(line, axis)
			for _, segment := range splitAtContainerBreaks(ordered, axis) {
				if len(segment) < 2 {
					continue
				}
				runs = append(runs, spacingRun{axis: axis, elements: segment})
			}
		}
	}
	return runs
}

// lineBands groups the elements that share a line running along axis: a row for
// the horizontal axis, a column for the vertical one.
func lineBands(elements []elementView, axis stackAxis) [][]elementView {
	sorted := make([]elementView, len(elements))
	copy(sorted, elements)
	sort.SliceStable(sorted, func(i, j int) bool {
		iStart, _ := crossSpan(sorted[i], axis)
		jStart, _ := crossSpan(sorted[j], axis)
		return iStart < jStart
	})

	var bands [][]elementView
	for _, el := range sorted {
		if last := len(bands) - 1; last >= 0 && sharesLine(bands[last][0], el, axis) {
			bands[last] = append(bands[last], el)
			continue
		}
		bands = append(bands, []elementView{el})
	}
	return bands
}

// sharesLine reports whether two elements sit side by side along axis rather
// than one after the other.
//
// Mutual centre containment, not bare overlap: two stacked cards that overlap
// by a few pixels are a layout break, and folding them into one row would
// measure their gap along the wrong axis and report the break as a clean row
// instead. Requiring each centre to fall inside the other's span keeps a
// genuine row together while leaving a partial overlap as two lines.
func sharesLine(anchor, el elementView, axis stackAxis) bool {
	anchorStart, anchorEnd := crossSpan(anchor, axis)
	elStart, elEnd := crossSpan(el, axis)
	anchorCenter := (anchorStart + anchorEnd) / 2
	elCenter := (elStart + elEnd) / 2
	return elCenter >= anchorStart && elCenter <= anchorEnd &&
		anchorCenter >= elStart && anchorCenter <= elEnd
}

// crossSpan is the element's extent on the axis the run does NOT travel along:
// the vertical extent of a row, the horizontal extent of a column.
func crossSpan(el elementView, axis stackAxis) (start, end float64) {
	if axis == axisHorizontal {
		return el.Box.Top, el.Box.Bottom
	}
	return el.Box.Left, el.Box.Right
}

// splitAtContainerBreaks divides one line wherever the spacing jumps to a
// multiple of the line's own rhythm.
//
// Without a parent identity in the element view there is nothing that says
// "these three cards are one section and those three are another", so the break
// itself has to serve as the boundary. The alternative is what this replaces:
// 120px of section padding measured as a sibling gap and blamed on the margin
// of the first card after it, which is a rule that does not control it.
func splitAtContainerBreaks(ordered []elementView, axis stackAxis) [][]elementView {
	whole := [][]elementView{ordered}

	gaps := measureGaps(ordered, axis)
	positive := positiveGaps(gaps)
	if len(positive) < minimumGapsForContainerSplit {
		return whole
	}
	rhythm, rhythmCount := modalGap(positive)
	if rhythm <= 0 || rhythmCount < minimumGapsForRhythm {
		// No repeated gap means no rhythm to be a multiple of, and splitting on
		// a mode of one would carve the line up at its own largest gap.
		return whole
	}
	threshold := rhythm * containerBreakRatio

	segments := [][]elementView{{ordered[0]}}
	for i, gap := range gaps {
		if gap.size >= threshold {
			segments = append(segments, []elementView{ordered[i+1]})
			continue
		}
		last := len(segments) - 1
		segments[last] = append(segments[last], ordered[i+1])
	}
	return segments
}

// orderAlongAxis sorts by rendered position. DOM order is not layout order once
// flex ordering, direction or absolute placement are involved.
func orderAlongAxis(elements []elementView, axis stackAxis) []elementView {
	ordered := make([]elementView, len(elements))
	copy(ordered, elements)
	sort.SliceStable(ordered, func(i, j int) bool {
		if axis == axisHorizontal {
			return ordered[i].Box.Left < ordered[j].Box.Left
		}
		return ordered[i].Box.Top < ordered[j].Box.Top
	})
	return ordered
}

// measureGaps computes the rendered space between each adjacent pair.
func measureGaps(ordered []elementView, axis stackAxis) []siblingGap {
	gaps := make([]siblingGap, 0, len(ordered)-1)
	for i := 1; i < len(ordered); i++ {
		before, after := ordered[i-1], ordered[i]
		var size float64
		if axis == axisHorizontal {
			size = after.Box.Left - before.Box.Right
		} else {
			size = after.Box.Top - before.Box.Bottom
		}
		gaps = append(gaps, siblingGap{before: before, after: after, size: roundToTolerance(size)})
	}
	return gaps
}

func positiveGaps(gaps []siblingGap) []siblingGap {
	positive := make([]siblingGap, 0, len(gaps))
	for _, gap := range gaps {
		if gap.size >= -subPixelTolerance {
			positive = append(positive, gap)
		}
	}
	return positive
}

// modalGap returns the most common gap and how many gaps share it.
//
// The mode, not the mean: a single 14px among 24s drags a mean to 21.5px, which
// then makes every correct gap look slightly wrong and the actual outlier look
// less wrong than it is.
//
// The count is half the answer. A set of distinct gaps has a "most common"
// value with a count of one, which is not a rhythm, so every caller must
// consult the count before treating the value as a norm — see isRhythm.
func modalGap(gaps []siblingGap) (float64, int) {
	buckets := make(map[float64]int)
	for _, gap := range gaps {
		buckets[gap.size]++
	}

	sizes := make([]float64, 0, len(buckets))
	for size := range buckets {
		sizes = append(sizes, size)
	}
	sort.Float64s(sizes)

	best, bestCount := 0.0, 0
	for _, size := range sizes {
		if buckets[size] > bestCount {
			best, bestCount = size, buckets[size]
		}
	}
	return best, bestCount
}

// isGapOwningParent reports whether the parent's own gap property, rather than
// the child's margin, produces the spacing.
func isGapOwningParent(el elementView) bool {
	if el.ParentDisplay == "" {
		return false
	}
	switch el.ParentDisplay {
	case "flex", "inline-flex", "grid", "inline-grid":
		gap, ok := parseLength(el.ParentGap)
		return ok && gap > 0
	}
	return false
}

// roundToTolerance snaps sub-pixel noise so 23.98px and 24px are one bucket.
func roundToTolerance(value float64) float64 {
	return math.Round(value)
}
