// tokens.go — Design-token compliance: flags values that near-miss a declared token (#694).
//
// PURPOSE: catch the case where an agent hardcoded a value one notch off the
// design system — padding:15px against --spacing-md:16px, #2b56e2 against
// --color-primary-main:#2a55e1 — which renders almost identically and so
// survives every functional test and every visual review.
//
// CONTRACT: only near-misses are reported, never "every literal value". A
// page's computed styles contain hundreds of legitimate non-token lengths
// (element widths, derived line heights, borders); flagging them all would bury
// the one value that is actually wrong. An exact token match is the success
// state and produces nothing.

package designdrift

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	// colorNearMissThreshold is an OKLab distance. Below it, two colours are
	// close enough that the difference is almost certainly a typo or a
	// copy-paste rather than a deliberate second colour. Calibrated in
	// tokens_test.go against the #2b56e2 / #2a55e1 pair from the issue.
	colorNearMissThreshold = 0.02

	// lengthNearMissRatio is relative, not absolute, because 2px off a 4px
	// token is a different value while 2px off a 64px token is a slip. 15px
	// against a 16px token is 6.25% and lands inside; 6px against 4px is 50%
	// and does not.
	lengthNearMissRatio = 0.15

	// subPixelTolerance absorbs the fractional pixels that transforms, zoom and
	// fractional layout introduce, so 23.98px is not reported as drift.
	subPixelTolerance = 0.5
)

// spacingProperties() are the longhands a spacing scale governs. Shorthands are
// deliberately absent: `padding: 15px` must expand into four comparisons, which
// is why the probe captures longhands.
func spacingProperties() []string {
	return []string{
		"margin-top", "margin-right", "margin-bottom", "margin-left",
		"padding-top", "padding-right", "padding-bottom", "padding-left",
	}
}

func colorProperties() []string { return []string{"color", "background-color", "border-color"} }

// tokenTable is the page's declared design system, parsed into comparable form.
type tokenTable struct {
	colors  map[string]rgbColor
	lengths map[string]float64
	raw     map[string]string
}

func (t tokenTable) empty() bool { return len(t.colors) == 0 && len(t.lengths) == 0 }

// buildTokenTable parses a raw :root custom-property table into typed values.
// Properties that are neither a colour nor a length (easing curves, font
// stacks, z-index steps) are kept in raw but take part in no comparison.
func buildTokenTable(raw map[string]string) tokenTable {
	table := tokenTable{
		colors:  make(map[string]rgbColor),
		lengths: make(map[string]float64),
		raw:     make(map[string]string, len(raw)),
	}
	for name, value := range raw {
		trimmed := strings.TrimSpace(value)
		table.raw[name] = trimmed
		if c, ok := parseColor(trimmed); ok {
			table.colors[name] = c
			continue
		}
		if l, ok := parseLength(trimmed); ok {
			table.lengths[name] = l
		}
	}
	return table
}

func sortedTokenNames[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// analyzeTokens checks each element's used values against the token table and
// the declared spec.
func analyzeTokens(elements []elementView, tokens tokenTable, spec *designSpec) ([]finding, *skipped) {
	if tokens.empty() && (spec == nil || spec.empty()) {
		return nil, &skipped{
			Category: categoryDesignTokens,
			Reason:   reasonNoTokens,
			// Without a declared system there is nothing to be non-compliant
			// with. Reporting every literal value as "hardcoded" here would be
			// noise, and reporting nothing without saying why would look like
			// a clean bill of health.
		}
	}

	var findings []finding
	findings = append(findings, detectSpecConflicts(spec, tokens)...)

	for _, el := range elements {
		findings = append(findings, tokenFindingsForElement(el, tokens, spec)...)
	}
	return findings, nil
}

func tokenFindingsForElement(el elementView, tokens tokenTable, spec *designSpec) []finding {
	var findings []finding

	for _, property := range spacingProperties() {
		if f, ok := lengthFinding(el, property, tokens, spec); ok {
			findings = append(findings, f)
		}
	}
	for _, property := range colorProperties() {
		if f, ok := colorFinding(el, property, tokens, spec); ok {
			findings = append(findings, f)
		}
	}
	return findings
}

// lengthFinding compares one length longhand against the spacing scale or the
// page's length tokens.
func lengthFinding(el elementView, property string, tokens tokenTable, spec *designSpec) (finding, bool) {
	raw, present := el.Styles[property]
	if !present {
		return finding{}, false
	}
	value, ok := parseLength(raw)
	if !ok || value == 0 {
		// Zero is the CSS default for margin and padding and carries no design
		// intent; treating it as a token miss would flag most of the page.
		return finding{}, false
	}

	if spec.declaresSpacing() {
		if scaleContains(spec.SpacingScale, value) {
			return finding{}, false
		}
		return newFinding(categoryDesignTokens, property, el, formatPx(value), formatScale(spec.SpacingScale),
			provenanceDeclared, confidenceHigh, "declared spec spacing scale",
			fmt.Sprintf("%s is %s, which is not on the declared spacing scale", property, formatPx(value))), true
	}

	name, tokenValue, found := nearestLengthToken(tokens, value)
	if !found {
		return finding{}, false
	}
	if absFloat(tokenValue-value) <= subPixelTolerance {
		// Exact match: the success state this feature steers toward.
		return finding{}, false
	}
	return newFinding(categoryDesignTokens, property, el, formatPx(value), fmt.Sprintf("%s (%s)", name, formatPx(tokenValue)),
		provenanceDeclared, confidenceHigh, fmt.Sprintf("page token %s = %s", name, formatPx(tokenValue)),
		fmt.Sprintf("%s is %s, a near-miss of the declared token %s (%s)", property, formatPx(value), name, formatPx(tokenValue))), true
}

// colorFinding compares one colour property against the palette or the page's
// colour tokens.
func colorFinding(el elementView, property string, tokens tokenTable, spec *designSpec) (finding, bool) {
	raw, present := el.Styles[property]
	if !present {
		return finding{}, false
	}
	value, ok := parseColor(raw)
	if !ok || value.transparent() {
		// A fully transparent colour is an absence, not a design decision.
		return finding{}, false
	}

	if spec.declaresColors() {
		for _, allowed := range spec.Colors {
			if c, parsed := parseColor(allowed); parsed && colorDistance(value, c) <= colorNearMissThreshold {
				return finding{}, false
			}
		}
		return newFinding(categoryDesignTokens, property, el, value.css(), strings.Join(spec.Colors, ", "),
			provenanceDeclared, confidenceHigh, "declared spec palette",
			fmt.Sprintf("%s is %s, which is not in the declared palette", property, value.css())), true
	}

	name, tokenValue, distance, found := nearestColorToken(tokens, value)
	if !found || distance > colorNearMissThreshold {
		return finding{}, false
	}
	if distance == 0 {
		return finding{}, false
	}
	return newFinding(categoryDesignTokens, property, el, value.css(), fmt.Sprintf("%s (%s)", name, tokenValue.css()),
		provenanceDeclared, confidenceHigh, fmt.Sprintf("page token %s = %s", name, tokenValue.css()),
		fmt.Sprintf("%s is %s, a near-miss of the declared token %s (%s)", property, value.css(), name, tokenValue.css())), true
}

// nearestLengthToken returns the closest length token within the relative
// near-miss band. A value far from every token is not reported: it is more
// likely a legitimate non-token length than a mistyped one.
func nearestLengthToken(tokens tokenTable, value float64) (string, float64, bool) {
	bestName, bestValue, bestDelta := "", 0.0, math.MaxFloat64
	for _, name := range sortedTokenNames(tokens.lengths) {
		tokenValue := tokens.lengths[name]
		if tokenValue <= 0 {
			continue
		}
		delta := absFloat(tokenValue - value)
		if delta/tokenValue > lengthNearMissRatio {
			continue
		}
		if delta < bestDelta {
			bestName, bestValue, bestDelta = name, tokenValue, delta
		}
	}
	return bestName, bestValue, bestName != ""
}

// nearestColorToken returns the perceptually closest colour token.
func nearestColorToken(tokens tokenTable, value rgbColor) (string, rgbColor, float64, bool) {
	bestName, bestValue, bestDistance := "", rgbColor{}, math.MaxFloat64
	for _, name := range sortedTokenNames(tokens.colors) {
		tokenValue := tokens.colors[name]
		distance := colorDistance(value, tokenValue)
		if distance < bestDistance {
			bestName, bestValue, bestDistance = name, tokenValue, distance
		}
	}
	return bestName, bestValue, bestDistance, bestName != ""
}

// --- Value parsing ---

var (
	lengthPattern = regexp.MustCompile(`^(-?[0-9]*\.?[0-9]+)px$`)
	rgbPattern    = regexp.MustCompile(`^rgba?\(\s*([0-9.]+)[,\s]+([0-9.]+)[,\s]+([0-9.]+)\s*(?:[,/]\s*([0-9.%]+)\s*)?\)$`)
	hexPattern    = regexp.MustCompile(`^#([0-9a-fA-F]{3,8})$`)
)

// parseLength reads a computed length. getComputedStyle resolves rem, em, % and
// calc() to px before we ever see them, so px is the only unit that arrives.
func parseLength(value string) (float64, bool) {
	m := lengthPattern.FindStringSubmatch(strings.TrimSpace(value))
	if m == nil {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

// rgbColor is an sRGB colour with alpha.
type rgbColor struct {
	R, G, B float64
	A       float64
}

func (c rgbColor) transparent() bool { return c.A == 0 }

// css renders the colour the way the page reported it, so a finding quotes
// something the reader can search for.
func (c rgbColor) css() string {
	if c.A < 1 {
		return fmt.Sprintf("rgba(%d, %d, %d, %g)", int(math.Round(c.R)), int(math.Round(c.G)), int(math.Round(c.B)), c.A)
	}
	return fmt.Sprintf("#%02x%02x%02x", int(math.Round(c.R)), int(math.Round(c.G)), int(math.Round(c.B)))
}

// parseColor reads rgb(), rgba() and hex notations.
func parseColor(value string) (rgbColor, bool) {
	trimmed := strings.TrimSpace(value)
	if m := rgbPattern.FindStringSubmatch(trimmed); m != nil {
		r, rErr := strconv.ParseFloat(m[1], 64)
		g, gErr := strconv.ParseFloat(m[2], 64)
		b, bErr := strconv.ParseFloat(m[3], 64)
		if rErr != nil || gErr != nil || bErr != nil {
			return rgbColor{}, false
		}
		alpha := 1.0
		if m[4] != "" {
			alpha = parseAlpha(m[4])
		}
		return rgbColor{R: r, G: g, B: b, A: alpha}, true
	}
	if m := hexPattern.FindStringSubmatch(trimmed); m != nil {
		return parseHex(m[1])
	}
	return rgbColor{}, false
}

func parseAlpha(raw string) float64 {
	if strings.HasSuffix(raw, "%") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(raw, "%"), 64); err == nil {
			return v / 100
		}
		return 1
	}
	if v, err := strconv.ParseFloat(raw, 64); err == nil {
		return v
	}
	return 1
}

func parseHex(digits string) (rgbColor, bool) {
	expand := func(s string) string { return string([]byte{s[0], s[0]}) }
	switch len(digits) {
	case 3, 4:
		full := expand(digits[0:1]) + expand(digits[1:2]) + expand(digits[2:3])
		if len(digits) == 4 {
			full += expand(digits[3:4])
		}
		return parseHex(full)
	case 6, 8:
		r, rErr := strconv.ParseUint(digits[0:2], 16, 8)
		g, gErr := strconv.ParseUint(digits[2:4], 16, 8)
		b, bErr := strconv.ParseUint(digits[4:6], 16, 8)
		if rErr != nil || gErr != nil || bErr != nil {
			return rgbColor{}, false
		}
		alpha := 1.0
		if len(digits) == 8 {
			if a, err := strconv.ParseUint(digits[6:8], 16, 8); err == nil {
				alpha = float64(a) / 255
			}
		}
		return rgbColor{R: float64(r), G: float64(g), B: float64(b), A: alpha}, true
	}
	return rgbColor{}, false
}

// --- Perceptual colour distance ---

// colorDistance returns the OKLab distance between two colours.
//
// Naive RGB distance treats the channels as equally weighted, which they are
// not: the eye is far more sensitive to green than to blue, so an RGB metric
// calls two obviously different colours close and two identical-looking ones
// far apart. OKLab is perceptually uniform, which is what makes a single
// threshold meaningful across the whole gamut.
func colorDistance(a, b rgbColor) float64 {
	al, aa, ab := oklab(a)
	bl, ba, bb := oklab(b)
	return math.Sqrt((al-bl)*(al-bl) + (aa-ba)*(aa-ba) + (ab-bb)*(ab-bb))
}

// oklab converts sRGB to OKLab (Björn Ottosson's transform).
func oklab(c rgbColor) (l, a, b float64) {
	lr := srgbToLinear(c.R / 255)
	lg := srgbToLinear(c.G / 255)
	lb := srgbToLinear(c.B / 255)

	longC := 0.4122214708*lr + 0.5363325363*lg + 0.0514459929*lb
	medium := 0.2119034982*lr + 0.6806995451*lg + 0.1073969566*lb
	short := 0.0883024619*lr + 0.2817188376*lg + 0.6299787005*lb

	lRoot := math.Cbrt(longC)
	mRoot := math.Cbrt(medium)
	sRoot := math.Cbrt(short)

	return 0.2104542553*lRoot + 0.7936177850*mRoot - 0.0040720468*sRoot,
		1.9779984951*lRoot - 2.4285922050*mRoot + 0.4505937099*sRoot,
		0.0259040371*lRoot + 0.7827717662*mRoot - 0.8086757660*sRoot
}

func srgbToLinear(channel float64) float64 {
	if channel <= 0.04045 {
		return channel / 12.92
	}
	return math.Pow((channel+0.055)/1.055, 2.4)
}

// --- Small helpers ---

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// formatPx prints a length without trailing zeros, so 16 reads as "16px".
func formatPx(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64) + "px"
}
