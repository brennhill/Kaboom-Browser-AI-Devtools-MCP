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

	axis := detectAxis(flow)
	ordered := orderAlongAxis(flow, axis)
	gaps := measureGaps(ordered, axis)
	if len(gaps) < minimumGapsForRhythm {
		return nil, &skipped{Category: categorySpacing, Reason: reasonInsufficientPeers}
	}

	var findings []finding
	findings = append(findings, overlapFindings(gaps, axis)...)

	positive := positiveGaps(gaps)
	if len(positive) < minimumGapsForRhythm {
		return findings, nil
	}

	rhythm, rhythmCount := modalGap(positive)
	provenance := spec.provenanceForSpacing()
	expected := formatPx(rhythm)
	if provenance == provenanceDeclared {
		expected = formatScale(spec.SpacingScale)
	}

	for _, gap := range positive {
		if provenance == provenanceDeclared {
			if scaleContains(spec.SpacingScale, gap.size) {
				continue
			}
		} else if absFloat(gap.size-rhythm) <= gapDeviationTolerance {
			continue
		}
		findings = append(findings, spacingFinding(gap, axis, expected, provenance, rhythm, rhythmCount, len(positive)))
	}
	return findings, nil
}

// spacingFinding builds one gap finding, attributing the cause to whichever
// rule actually owns the spacing.
func spacingFinding(gap siblingGap, axis stackAxis, expected, provenance string, rhythm float64, rhythmCount, totalGaps int) finding {
	confidence := confidenceLow
	if float64(rhythmCount)/float64(totalGaps) >= strongMajorityRatio {
		confidence = confidenceHigh
	}
	evidence := fmt.Sprintf("%d of %d %s gaps measure %s", rhythmCount, totalGaps, axis, formatPx(rhythm))

	// A flex or grid parent's gap belongs to no child's margin at all. Naming
	// the child would send an agent editing a rule that does not control this
	// spacing.
	owner := "the element's own margin"
	if isGapOwningParent(gap.after) {
		owner = fmt.Sprintf("the parent's %s gap property", gap.after.ParentDisplay)
	}

	return newFinding(categorySpacing, "gap-"+axis.String(), gap.after,
		formatPx(gap.size), expected, provenance, confidence, evidence,
		fmt.Sprintf("the %s gap before this element is %s where the rhythm is %s; the spacing is controlled by %s",
			axis, formatPx(gap.size), expected, owner))
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
		findings = append(findings, newFinding(categorySpacing, "overlap-"+axis.String(), gap.after,
			formatPx(gap.size), "no overlap", provenanceInferred, confidenceHigh,
			fmt.Sprintf("overlaps %s by %s", gap.before.Selector, formatPx(-gap.size)),
			fmt.Sprintf("this element overlaps the previous one by %s along the %s axis", formatPx(-gap.size), axis)))
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

// detectAxis decides whether the group is a column or a row, rather than
// assuming vertical: a row of cards drifts horizontally and would otherwise be
// measured along the wrong axis.
func detectAxis(elements []elementView) stackAxis {
	verticalSpread, horizontalSpread := 0.0, 0.0
	for i := 1; i < len(elements); i++ {
		verticalSpread += absFloat(elements[i].Box.Top - elements[i-1].Box.Top)
		horizontalSpread += absFloat(elements[i].Box.Left - elements[i-1].Box.Left)
	}
	if horizontalSpread > verticalSpread {
		return axisHorizontal
	}
	return axisVertical
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

	best, bestCount := sizes[0], 0
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
