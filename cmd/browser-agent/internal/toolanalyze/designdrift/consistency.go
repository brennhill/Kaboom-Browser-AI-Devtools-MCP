// consistency.go — Computed-style drift across semantic peers (#693).
//
// PURPOSE: catch the multi-step form whose Step 2 header renders Roboto 11px
// while Step 1 and Step 3 render Inter 12px. Structurally identical elements
// that look different are invisible to unit and E2E tests, which check presence
// and roles, not appearance.
//
// CONTRACT: the difficulty here is false positives, not detection. Legitimate
// variation is indistinguishable from drift in a raw computed-style dump, so
// this analyzer is deliberately narrow: it audits only properties where
// variation is almost never intentional, and it excludes the element classes
// that vary by design. Widening the property list without widening the
// exclusions will produce confident nonsense.

package designdrift

import (
	"fmt"
	"sort"
	"strings"
)

// auditedProperties() are the properties where a minority value is much more
// likely a mistake than a decision.
//
// Excluded on purpose: width and height (content-dependent), background-color
// (striping and emphasis are normal), margin and padding (structural position
// legitimately changes them — that is the spacing analyzer's job, where the
// geometry says which gap is wrong rather than which element is odd).
func auditedProperties() []string {
	return []string{
		"font-family",
		"font-size",
		"font-weight",
		"line-height",
		"letter-spacing",
		"color",
	}
}

// minimumPeersForMajority is the smallest group in which one element can be
// called an outlier. With two elements there is no majority — only two values,
// each as likely to be the correct one — so a verdict would be a coin flip.
const minimumPeersForMajority = 3

// strongMajorityRatio separates a confident call from a weak one. 9 of 10
// sharing a font is a strong signal; 3 of 5 is a group that may simply have two
// legitimate variants.
const strongMajorityRatio = 0.8

// stateVariantMarkers() are class fragments and attributes that mean "this
// element is deliberately different right now". A selected tab, an active nav
// item or a disabled button differing in colour and weight is the design
// working, not drift.
func stateVariantMarkers() []string {
	return []string{
		"active", "selected", "current", "disabled", "checked", "open",
		"expanded", "highlight", "featured", "primary", "danger", "error",
		"warning", "success", "focus", "hover", "invalid", "readonly",
	}
}

// analyzeConsistency flags minority computed values within the peer group.
func analyzeConsistency(elements []elementView, spec *designSpec) ([]finding, *skipped) {
	peers := eligiblePeers(elements)
	if len(peers) < minimumPeersForMajority {
		return nil, &skipped{
			Category: categoryStyleConsistency,
			Reason:   reasonInsufficientPeers,
		}
	}

	var findings []finding
	for _, property := range auditedProperties() {
		findings = append(findings, consistencyFindingsForProperty(peers, property, spec)...)
	}
	return findings, nil
}

// eligiblePeers drops the elements whose difference would be legitimate.
//
// State variants are excluded rather than reported at low confidence: an
// .active card is *supposed* to differ, so a finding about it is noise however
// it is labelled, and leaving it in the group also skews the majority the other
// elements are judged against.
func eligiblePeers(elements []elementView) []elementView {
	peers := make([]elementView, 0, len(elements))
	for _, el := range elements {
		if isStateVariant(el) {
			continue
		}
		peers = append(peers, el)
	}
	return peers
}

func isStateVariant(el elementView) bool {
	lower := strings.ToLower(el.Selector)
	for _, marker := range stateVariantMarkers() {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func consistencyFindingsForProperty(peers []elementView, property string, spec *designSpec) []finding {
	counts := make(map[string]int)
	present := 0
	for _, el := range peers {
		value, ok := el.Styles[property]
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		counts[normalizeValue(property, value)]++
		present++
	}
	if present < minimumPeersForMajority {
		return nil
	}

	if provenance, expected := consistencyExpectation(property, "", spec); provenance == provenanceDeclared {
		return declaredFindings(peers, property, expected, spec)
	}
	return inferredFindings(peers, property, counts, present)
}

// declaredFindings judges every element against the stated rule.
//
// The majority is deliberately not consulted here. A page where every element
// uses the wrong font has a wrong majority, and that is precisely the case
// inference cannot reach and a declared spec exists to catch — gating this path
// on "differs from its peers" would make the spec useless exactly when it
// matters most.
func declaredFindings(peers []elementView, property, expected string, spec *designSpec) []finding {
	var findings []finding
	for _, el := range peers {
		value, ok := el.Styles[property]
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		normalized := normalizeValue(property, value)
		if matchesDeclared(property, normalized, spec) {
			continue
		}
		findings = append(findings, newFinding(categoryStyleConsistency, property, el, normalized, expected,
			provenanceDeclared, confidenceHigh, "declared design spec",
			fmt.Sprintf("%s is %s, which the declared design spec does not allow (%s)", property, normalized, expected)))
	}
	return findings
}

// inferredFindings flags the minority against the peer majority.
func inferredFindings(peers []elementView, property string, counts map[string]int, present int) []finding {
	if len(counts) < 2 {
		return nil
	}
	majority, majorityCount := dominantValue(counts)
	if majorityCount*2 <= present {
		// No value holds a strict majority, so there is no norm to deviate
		// from — two evenly-split variants are a design choice, not an outlier.
		return nil
	}

	confidence := confidenceLow
	if float64(majorityCount)/float64(present) >= strongMajorityRatio {
		confidence = confidenceHigh
	}

	var findings []finding
	for _, el := range peers {
		value, ok := el.Styles[property]
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		normalized := normalizeValue(property, value)
		if normalized == majority {
			continue
		}
		findings = append(findings, newFinding(categoryStyleConsistency, property, el, normalized, majority,
			provenanceInferred, confidence,
			fmt.Sprintf("%d of %d peers use %s", majorityCount, present, majority),
			fmt.Sprintf("%s is %s while %d of %d matching elements use %s", property, normalized, majorityCount, present, majority)))
	}
	return findings
}

// consistencyExpectation applies precedence for one property: a declared spec
// beats the inferred majority. Precedence is resolved per property, so a spec
// naming only font_families leaves colour judgement to inference in the same
// response.
func consistencyExpectation(property, majority string, spec *designSpec) (provenance, expected string) {
	switch property {
	case "font-family":
		if spec.declaresFonts() {
			return provenanceDeclared, strings.Join(spec.FontFamilies, ", ")
		}
	case "font-size":
		if spec.declaresSizes() {
			return provenanceDeclared, formatScale(spec.FontSizes)
		}
	case "color":
		if spec.declaresColors() {
			return provenanceDeclared, strings.Join(spec.Colors, ", ")
		}
	}
	return provenanceInferred, majority
}

// matchesDeclared reports whether a value satisfies the declared rule for its
// property family.
func matchesDeclared(property, value string, spec *designSpec) bool {
	switch property {
	case "font-family":
		for _, family := range spec.FontFamilies {
			if strings.EqualFold(normalizeFontFamily(family), value) {
				return true
			}
		}
	case "font-size":
		if parsed, ok := parseLength(value); ok {
			return scaleContains(spec.FontSizes, parsed)
		}
	case "color":
		parsed, ok := parseColor(value)
		if !ok {
			return false
		}
		for _, allowed := range spec.Colors {
			if c, parsedAllowed := parseColor(allowed); parsedAllowed && colorDistance(parsed, c) <= colorNearMissThreshold {
				return true
			}
		}
	}
	return false
}

// dominantValue returns the most common value, breaking ties by value so the
// result does not depend on map iteration order.
func dominantValue(counts map[string]int) (string, int) {
	values := make([]string, 0, len(counts))
	for value := range counts {
		values = append(values, value)
	}
	sort.Strings(values)

	best, bestCount := "", 0
	for _, value := range values {
		if counts[value] > bestCount {
			best, bestCount = value, counts[value]
		}
	}
	return best, bestCount
}

// normalizeValue canonicalises a computed value so cosmetic differences do not
// read as drift.
func normalizeValue(property, value string) string {
	trimmed := strings.TrimSpace(value)
	switch property {
	case "font-family":
		return normalizeFontFamily(trimmed)
	case "color":
		if c, ok := parseColor(trimmed); ok {
			return c.css()
		}
	}
	return trimmed
}

// normalizeFontFamily compares the first family in a stack, lowercased and
// unquoted. Two elements resolving to the same face through differently-written
// fallback stacks are not drifting.
func normalizeFontFamily(stack string) string {
	first := stack
	if idx := strings.Index(stack, ","); idx >= 0 {
		first = stack[:idx]
	}
	first = strings.TrimSpace(first)
	first = strings.Trim(first, `"'`)
	return strings.ToLower(first)
}
