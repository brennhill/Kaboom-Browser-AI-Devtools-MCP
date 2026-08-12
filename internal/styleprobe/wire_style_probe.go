// wire_style_probe.go — Defines the computed-style probe wire contract shared by the page and the design analyzers.
//
// PURPOSE: carry everything a design-drift analysis needs out of the page in one
// query — computed longhands, box geometry, and the CSS custom properties in
// scope — so the analysis itself can be pure Go over this payload.
//
// CONTRACT: the page reports raw observed values and makes no judgements. It
// never decides what is a token, what is drift, or what the norm is; those are
// arithmetic and belong on the Go side where they can be table-tested. Anything
// added here must stay a measurement, not a verdict.

package styleprobe

// WireStyleProbeResult is the payload the page returns for one probe query.
type WireStyleProbeResult struct {
	Elements []WireStyleProbeElement `json:"elements"`

	// Count is how many elements are present in Elements.
	Count int `json:"count"`

	// MatchCount is how many elements the selector actually matched, which is
	// larger than Count when the cap truncated the set. Truncated reports the
	// same fact as a flag. Both exist because a verdict computed over a
	// silently truncated set is wrong while looking authoritative: a 60-card
	// grid capped at 50 would have its spacing rhythm and its style majority
	// decided by two thirds of the evidence, with nothing saying so.
	MatchCount int  `json:"match_count"`
	Truncated  bool `json:"truncated"`

	// RootTokens is the :root custom-property table, resolved, keyed by the
	// full property name including the leading dashes.
	RootTokens map[string]string `json:"root_tokens,omitempty"`
}

// WireStyleProbeElement is one matched element's observed state.
type WireStyleProbeElement struct {
	Selector       string            `json:"selector"`
	Tag            string            `json:"tag"`
	ComputedStyles map[string]string `json:"computed_styles"`
	BoxModel       WireStyleProbeBox `json:"box_model"`
	ContrastRatio  float64           `json:"contrast_ratio,omitempty"`

	// CustomProperties are the --* values in scope for this element, which may
	// shadow the :root table. Token matching needs the value the element
	// actually sees, not only the document-level declaration.
	CustomProperties map[string]string `json:"custom_properties,omitempty"`

	// Index is the element's position among the selector's matches, before any
	// out-of-flow filtering. Structural position is a legitimate reason for a
	// style to differ, so an analyzer needs to know which element was first.
	Index int `json:"index"`

	// ParentDisplay and ParentGap describe the layout context that owns the
	// spacing between siblings. A flex or grid parent's gap belongs to no
	// child's margin, and attributing it to one sends an agent editing the
	// wrong rule.
	ParentDisplay string `json:"parent_display,omitempty"`
	ParentGap     string `json:"parent_gap,omitempty"`

	// InFlow is false for elements excluded from normal flow — absolute,
	// fixed, display:none, or zero-sized. A hidden node otherwise manufactures
	// a phantom gap in the rhythm.
	InFlow bool `json:"in_flow"`
}

// WireStyleProbeBox is the element's border-box geometry in viewport pixels.
type WireStyleProbeBox struct {
	// Gaps are measured from these rects rather than from declared margins on
	// purpose: adjacent vertical margins collapse to the larger of the two, so
	// margin arithmetic does not equal the rendered gap. Do not "fix" a gap
	// calculation back to reading margin-bottom and margin-top.
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
}
