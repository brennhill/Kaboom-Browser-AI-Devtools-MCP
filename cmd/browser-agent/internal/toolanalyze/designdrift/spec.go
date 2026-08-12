// spec.go — The optional caller-declared design system and the baseline precedence rules.
//
// PURPOSE: let a caller state its design system so a page that is uniformly
// wrong is still catchable. Inference alone cannot flag a page where every card
// uses the same wrong font — the majority IS the wrong font.
//
// CONTRACT: precedence is resolved per property, never per call. A spec naming
// only a spacing scale must leave colour and font judgement to inference rather
// than disabling them, so a partial spec makes spacing deviations errors while
// font deviations in the same response stay warnings.

package designdrift

import (
	"fmt"
	"strings"
)

// designSpec is the caller-supplied design system. Every field is optional.
type designSpec struct {
	// SpacingScale is the set of legal spacing values in px.
	SpacingScale []float64 `json:"spacing_scale,omitempty"`
	// FontFamilies is the set of legal font-family stacks.
	FontFamilies []string `json:"font_families,omitempty"`
	// Colors is the set of legal colours, in any CSS notation the parser reads.
	Colors []string `json:"colors,omitempty"`
	// FontSizes is the set of legal font sizes in px.
	FontSizes []float64 `json:"font_sizes,omitempty"`
}

// declares reports whether the spec constrains a given property family, which
// is what makes precedence per-property rather than per-call.
func (s *designSpec) declaresSpacing() bool { return s != nil && len(s.SpacingScale) > 0 }
func (s *designSpec) declaresFonts() bool   { return s != nil && len(s.FontFamilies) > 0 }
func (s *designSpec) declaresColors() bool  { return s != nil && len(s.Colors) > 0 }
func (s *designSpec) declaresSizes() bool   { return s != nil && len(s.FontSizes) > 0 }

// empty reports a spec that constrains nothing, which is treated as no spec.
func (s *designSpec) empty() bool {
	return !s.declaresSpacing() && !s.declaresFonts() && !s.declaresColors() && !s.declaresSizes()
}

// provenanceForSpacing and friends resolve precedence for one property family:
// declared spec beats page token beats inferred majority.
//
// Only the spec produces declared provenance from the caller's side; a page
// :root token is also a declaration, but of the page's own making, and both
// outrank a majority vote.
func (s *designSpec) provenanceForSpacing() string {
	if s.declaresSpacing() {
		return provenanceDeclared
	}
	return provenanceInferred
}

func (s *designSpec) provenanceForFonts() string {
	if s.declaresFonts() {
		return provenanceDeclared
	}
	return provenanceInferred
}

func (s *designSpec) provenanceForColors() string {
	if s.declaresColors() {
		return provenanceDeclared
	}
	return provenanceInferred
}

func (s *designSpec) provenanceForSizes() string {
	if s.declaresSizes() {
		return provenanceDeclared
	}
	return provenanceInferred
}

// specConflict is a disagreement between the caller's spec and the page's own
// :root tokens.
//
// This is reported rather than silently resolved. Two sources both claiming to
// be authoritative is itself the finding: either the spec is stale or the page
// drifted from it, and picking a winner quietly hides whichever one is wrong.
func detectSpecConflicts(spec *designSpec, tokens tokenTable) []finding {
	if spec == nil || spec.empty() || len(tokens.colors)+len(tokens.lengths) == 0 {
		return nil
	}
	var findings []finding

	if spec.declaresColors() {
		findings = append(findings, conflictingColorTokens(spec, tokens)...)
	}
	if spec.declaresSpacing() {
		findings = append(findings, conflictingSpacingTokens(spec, tokens)...)
	}
	return findings
}

// conflictingColorTokens finds page colour tokens the declared palette omits.
func conflictingColorTokens(spec *designSpec, tokens tokenTable) []finding {
	allowed := make([]rgbColor, 0, len(spec.Colors))
	for _, raw := range spec.Colors {
		if c, ok := parseColor(raw); ok {
			allowed = append(allowed, c)
		}
	}
	if len(allowed) == 0 {
		return nil
	}

	var findings []finding
	for _, name := range sortedTokenNames(tokens.colors) {
		declared := tokens.colors[name]
		matched := false
		for _, a := range allowed {
			if colorDistance(declared, a) <= colorNearMissThreshold {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		findings = append(findings, finding{
			Category:     categoryDesignTokens,
			Severity:     severityError,
			Selector:     ":root",
			Property:     name,
			Observed:     declared.css(),
			Expected:     strings.Join(spec.Colors, ", "),
			ExpectedFrom: provenanceDeclared,
			Confidence:   confidenceHigh,
			Evidence:     "declared spec palette",
			Message: fmt.Sprintf("the page declares %s as %s, which the supplied spec's palette does not contain — the spec and the page disagree about the design system",
				name, declared.css()),
		})
	}
	return findings
}

// conflictingSpacingTokens finds page length tokens the declared scale omits.
func conflictingSpacingTokens(spec *designSpec, tokens tokenTable) []finding {
	var findings []finding
	for _, name := range sortedTokenNames(tokens.lengths) {
		declared := tokens.lengths[name]
		if !isSpacingTokenName(name) {
			continue
		}
		if scaleContains(spec.SpacingScale, declared) {
			continue
		}
		findings = append(findings, finding{
			Category:     categoryDesignTokens,
			Severity:     severityError,
			Selector:     ":root",
			Property:     name,
			Observed:     formatPx(declared),
			Expected:     formatScale(spec.SpacingScale),
			ExpectedFrom: provenanceDeclared,
			Confidence:   confidenceHigh,
			Evidence:     "declared spec spacing scale",
			Message: fmt.Sprintf("the page declares %s as %s, which is not on the supplied spec's spacing scale — the spec and the page disagree about the design system",
				name, formatPx(declared)),
		})
	}
	return findings
}

// isSpacingTokenName guesses whether a length token is a spacing token rather
// than a radius, width or font size. Conservative on purpose: mistaking a
// radius for spacing produces a confusing conflict report.
func isSpacingTokenName(name string) bool {
	lower := strings.ToLower(name)
	for _, hint := range []string{"spacing", "space", "gap", "gutter", "inset"} {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

// scaleContains reports whether a value sits on the declared scale, within the
// sub-pixel tolerance that transforms and zoom introduce.
func scaleContains(scale []float64, value float64) bool {
	for _, step := range scale {
		if absFloat(step-value) <= subPixelTolerance {
			return true
		}
	}
	return false
}

func formatScale(scale []float64) string {
	parts := make([]string, 0, len(scale))
	for _, step := range scale {
		parts = append(parts, formatPx(step))
	}
	return strings.Join(parts, ", ")
}
