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
//
// Three rules make that contract hold. Each one is a defect this file shipped
// with, so each is stated here rather than left implicit at the call site.
//
//  1. FAMILY. A length token governs one property family and no other.
//     --font-size-sm:14px is in nearly every design system, and letting it
//     answer "is 14px a token?" fails twice: it invents near-misses of the type
//     scale against padding, and it excuses the real 14px near-miss of
//     --spacing-md:16px. Families are read off the token NAME because CSS gives
//     a custom property no type, so the classifier is conservative on purpose —
//     a name it cannot place governs NOTHING rather than everything.
//
//  2. PROVENANCE. A near-miss of a PAGE token is an inference, not a broken
//     rule. The page declared --spacing-md:16px; it never declared that this
//     element's padding must use it. That last step is proximity, which is a
//     guess. Since severity follows provenance and error means "a stated rule
//     was broken" — the batch a caller may fix without review — page-token
//     near-misses are inferred/warning. A CALLER-SUPPLIED spec is the only
//     thing here that states a rule, so only a spec violation is
//     declared/error.
//
//  3. AUTHORSHIP. A declared spacing scale judges spacing CHOICES, not layout
//     output. Two exclusions:
//     - A used length that is near no step is not a missed step. The 137.5px
//     that `margin: 0 auto` resolves to and the -1px of the border-collapse
//     idiom are not values reaching for the scale, and no scale of positive
//     magnitudes can ever contain them, so reporting them would make
//     permanent unfixable errors out of correct CSS. The scale is judged with
//     the same relative band as a page token: a spec changes WHO stated the
//     norm, not what counts as drift.
//     - A used value that IS one of the page's own tokens is the page's
//     declaration, not this element's mistake. detectSpecConflicts reports
//     the spec/page disagreement once against :root; repeating it per element
//     multiplies one disagreement by the element count and blames the element
//     for obeying its own design system.
//     Colour gets neither exclusion's near-miss half, because the layout engine
//     never computes a colour out of geometry: an off-palette colour is always
//     somebody's choice, while an off-scale length may be nobody's.

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

func isSpacingProperty(property string) bool { return namesOneOf(spacingProperties(), property) }

// --- Token families ---

// Token families. A length custom property governs the property family its
// name places it in, and no other.
const (
	familySpacing    = "spacing"
	familyRadius     = "radius"
	familyTypography = "typography"
	// familyUnclassified is a token whose name places it nowhere: --breakpoint-md,
	// --sidebar-width, --z-index-modal, --shadow-offset. It governs nothing.
	familyUnclassified = "unclassified"
)

// tokenFamilyRule is one family and the name substrings that identify it.
type tokenFamilyRule struct {
	family string
	hints  []string
}

// tokenFamilyRules() are the classification hints in PRECEDENCE order, which is
// load-bearing because the hints overlap: --letter-spacing contains "spacing"
// but is a typography token, and treating it as a spacing step would let it
// govern padding. Typography is therefore matched before spacing.
//
// A function rather than a package var so the table cannot be mutated at
// runtime; it decides which tokens are allowed to judge which properties.
func tokenFamilyRules() []tokenFamilyRule {
	return []tokenFamilyRule{
		{familyTypography, []string{"font", "text", "letter", "line-height", "leading", "type-scale"}},
		{familyRadius, []string{"radius", "corner", "rounded"}},
		{familySpacing, []string{"spacing", "space", "gap", "gutter", "inset", "padding", "margin"}},
	}
}

// lengthTokenFamily classifies a length custom property from its name.
//
// Name-based because CSS gives a custom property no type: --spacing-md and
// --radius-md are both just "16px" by the time getComputedStyle reports them,
// so the name is the only signal that exists. That makes the classifier a
// heuristic, which is why the unclassified case governs nothing — a wrong guess
// must cost a missed finding, never an invented one.
func lengthTokenFamily(name string) string {
	lower := strings.ToLower(name)
	for _, rule := range tokenFamilyRules() {
		for _, hint := range rule.hints {
			if strings.Contains(lower, hint) {
				return rule.family
			}
		}
	}
	return familyUnclassified
}

// familyGoverns reports whether a token family constrains a CSS property.
func familyGoverns(family, property string) bool {
	switch family {
	case familySpacing:
		// Spacing steps govern the margin and padding longhands, and nothing
		// else in the probed set.
		return isSpacingProperty(property)
	case familyRadius, familyTypography:
		// Radii govern border-radius; type tokens govern font-size and
		// line-height. Neither property is probed by this analyzer, so neither
		// family has anything to say about the properties that are — and in
		// particular neither governs margin or padding.
		return false
	default:
		// Unclassified. Letting an unplaceable token govern everything is the
		// defect this model replaces.
		return false
	}
}

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
	if !ok || value <= 0 {
		// Zero is the CSS default for margin and padding and carries no design
		// intent; treating it as a token miss would flag most of the page. A
		// negative used length is an offset idiom — the -1px border-collapse
		// pull, a hanging indent — and a scale of positive magnitudes can never
		// contain one, so every negative margin would be a permanent error.
		return finding{}, false
	}

	if spec.declaresSpacing() {
		return specScaleFinding(el, property, value, tokens, spec)
	}
	return pageTokenLengthFinding(el, property, value, tokens)
}

// specScaleFinding judges a used length against the caller's declared scale.
func specScaleFinding(el elementView, property string, value float64, tokens tokenTable, spec *designSpec) (finding, bool) {
	if scaleContains(spec.SpacingScale, value) {
		return finding{}, false
	}
	if !nearAnyScaleStep(spec.SpacingScale, value) {
		// Rule 3: near no step, so it is not a missed step. This is the
		// resolved-`auto` margin, the percentage padding, the layout-derived
		// length — values the scale never described. Reporting them restores the
		// "flag every literal value" behaviour the contract forbids.
		return finding{}, false
	}
	if rendersAPageLengthToken(tokens, property, value) {
		// Rule 3: the element renders the page's own token. detectSpecConflicts
		// reports the spec/page disagreement once, against :root.
		return finding{}, false
	}
	return newFinding(findingSpec{category: categoryDesignTokens, property: property, el: el,
		observed: formatPx(value), expected: formatScale(spec.SpacingScale),
		provenance: provenanceDeclared, confidence: confidenceHigh, evidence: "declared spec spacing scale",
		message: fmt.Sprintf("%s is %s, which is not on the declared spacing scale", property, formatPx(value))}), true
}

// pageTokenLengthFinding judges a used length against the page's own tokens.
func pageTokenLengthFinding(el elementView, property string, value float64, tokens tokenTable) (finding, bool) {
	name, tokenValue, found := nearestLengthToken(tokens, property, value)
	if !found {
		return finding{}, false
	}
	if absFloat(tokenValue-value) <= subPixelTolerance {
		// Exact match: the success state this feature steers toward.
		return finding{}, false
	}
	// Rule 2: the page declared the token, not that this property must use it.
	return newFinding(findingSpec{category: categoryDesignTokens, property: property, el: el,
		observed: formatPx(value), expected: fmt.Sprintf("%s (%s)", name, formatPx(tokenValue)),
		provenance: provenanceInferred, confidence: confidenceHigh,
		evidence: fmt.Sprintf("page token %s = %s", name, formatPx(tokenValue)),
		message:  fmt.Sprintf("%s is %s, a near-miss of the page token %s (%s)", property, formatPx(value), name, formatPx(tokenValue))}), true
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
		return specPaletteFinding(el, property, value, tokens, spec)
	}
	return pageTokenColorFinding(el, property, value, tokens)
}

// specPaletteFinding judges a colour against the caller's declared palette.
//
// There is no near-miss exclusion here, unlike specScaleFinding: the layout
// engine never computes a colour out of geometry, so every colour that reaches
// this function was authored by somebody. A palette that omits it is therefore
// a real disagreement, however far away the nearest palette entry is.
func specPaletteFinding(el elementView, property string, value rgbColor, tokens tokenTable, spec *designSpec) (finding, bool) {
	for _, allowed := range spec.Colors {
		if c, parsed := parseColor(allowed); parsed && colorDistance(value, c) <= colorNearMissThreshold {
			return finding{}, false
		}
	}
	if rendersAPageColorToken(tokens, value) {
		// Rule 3: the element renders the page's own token. detectSpecConflicts
		// reports the spec/page disagreement once, against :root.
		return finding{}, false
	}
	return newFinding(findingSpec{category: categoryDesignTokens, property: property, el: el,
		observed: value.css(), expected: strings.Join(spec.Colors, ", "),
		provenance: provenanceDeclared, confidence: confidenceHigh, evidence: "declared spec palette",
		message: fmt.Sprintf("%s is %s, which is not in the declared palette", property, value.css())}), true
}

// pageTokenColorFinding judges a colour against the page's own colour tokens.
func pageTokenColorFinding(el elementView, property string, value rgbColor, tokens tokenTable) (finding, bool) {
	name, tokenValue, distance, found := nearestColorToken(tokens, value)
	if !found || distance > colorNearMissThreshold {
		return finding{}, false
	}
	if distance == 0 {
		return finding{}, false
	}
	// Rule 2: the page declared the token, not that this property must use it.
	return newFinding(findingSpec{category: categoryDesignTokens, property: property, el: el,
		observed: value.css(), expected: fmt.Sprintf("%s (%s)", name, tokenValue.css()),
		provenance: provenanceInferred, confidence: confidenceHigh,
		evidence: fmt.Sprintf("page token %s = %s", name, tokenValue.css()),
		message:  fmt.Sprintf("%s is %s, a near-miss of the page token %s (%s)", property, value.css(), name, tokenValue.css())}), true
}

// rendersAPageLengthToken reports whether a used length IS one of the page's own
// tokens for this property family, rather than a value near one.
func rendersAPageLengthToken(tokens tokenTable, property string, value float64) bool {
	for name, tokenValue := range tokens.lengths {
		if !familyGoverns(lengthTokenFamily(name), property) {
			continue
		}
		if absFloat(tokenValue-value) <= subPixelTolerance {
			return true
		}
	}
	return false
}

// rendersAPageColorToken reports whether a used colour IS one of the page's own
// colour tokens. Exact, not near: a near-miss of a page token is the element's
// own mistake and stays reportable, while an exact match is the page's
// declaration and belongs in the :root conflict report.
func rendersAPageColorToken(tokens tokenTable, value rgbColor) bool {
	for _, tokenValue := range tokens.colors {
		if colorDistance(value, tokenValue) == 0 {
			return true
		}
	}
	return false
}

// nearestLengthToken returns the closest length token that GOVERNS the property,
// within the relative near-miss band. A value far from every governing token is
// not reported: it is more likely a legitimate non-token length than a mistyped
// one.
func nearestLengthToken(tokens tokenTable, property string, value float64) (string, float64, bool) {
	bestName, bestValue, bestDelta := "", 0.0, math.MaxFloat64
	for _, name := range sortedTokenNames(tokens.lengths) {
		tokenValue := tokens.lengths[name]
		if tokenValue <= 0 {
			continue
		}
		// Rule 1: a radius or type-scale token has no say over a padding, and
		// must not get one by happening to hold the same number.
		if !familyGoverns(lengthTokenFamily(name), property) {
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

// alphaDistanceWeight scales opacity onto the OKLab axes so it can share their
// single threshold.
//
// 1.0 makes a full 0→1 opacity swing exactly as large as the black→white
// lightness swing, which is the honest comparison: both take a colour from one
// extreme of what the eye can resolve to the other. The consequence that
// matters is that a 10% tint lands 0.1 away — five times outside the near-miss
// band — so a deliberate tint is never read as a typo of the opaque colour it
// derives from, and never shadows it in nearestColorToken.
const alphaDistanceWeight = 1.0

// colorDistance returns the OKLab distance between two colours, opacity
// included.
//
// Naive RGB distance treats the channels as equally weighted, which they are
// not: the eye is far more sensitive to green than to blue, so an RGB metric
// calls two obviously different colours close and two identical-looking ones
// far apart. OKLab is perceptually uniform, which is what makes a single
// threshold meaningful across the whole gamut.
//
// OKLab itself has no opacity axis, so alpha is carried as a fourth,
// independently weighted term rather than folded into oklab(). Dropping it —
// as this function originally did — makes rgba(42,85,225,0.1) an EXACT match of
// #2a55e1: every tint in the palette then reads as its own base colour, and the
// tint token shadows the opaque one it was derived from.
func colorDistance(a, b rgbColor) float64 {
	al, aa, ab := oklab(a)
	bl, ba, bb := oklab(b)
	dAlpha := alphaDistanceWeight * (a.A - b.A)
	return math.Sqrt((al-bl)*(al-bl) + (aa-ba)*(aa-ba) + (ab-bb)*(ab-bb) + dAlpha*dAlpha)
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
