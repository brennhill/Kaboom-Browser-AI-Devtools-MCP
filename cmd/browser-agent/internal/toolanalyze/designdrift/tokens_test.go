// tokens_test.go — Design-token compliance and the perceptual colour metric (#694).

package designdrift

import (
	"strings"
	"testing"
)

// TestColorDistance_ThresholdSeparatesTyposFromDecisions calibrates the one
// number the whole colour check depends on.
//
// The threshold is only meaningful if a near-identical pair lands under it and
// a deliberately different pair lands well over it. Naive RGB distance fails
// this: it weights the channels equally when the eye does not, so it calls
// obviously different colours close and identical-looking ones far apart.
func TestColorDistance_ThresholdSeparatesTyposFromDecisions(t *testing.T) {
	t.Parallel()
	mustParse := func(raw string) rgbColor {
		t.Helper()
		c, ok := parseColor(raw)
		if !ok {
			t.Fatalf("parseColor(%q) failed", raw)
		}
		return c
	}

	// The pair from GitHub #694: one unit per channel apart, visually identical.
	nearMiss := colorDistance(mustParse("#2b56e2"), mustParse("#2a55e1"))
	if nearMiss > colorNearMissThreshold {
		t.Errorf("#2b56e2 vs #2a55e1 distance %.5f exceeds threshold %.5f — the issue's own example would not be caught",
			nearMiss, colorNearMissThreshold)
	}
	if nearMiss == 0 {
		t.Error("distance between two different colours is 0; the metric is not discriminating")
	}

	// Colours a designer chose separately must stay far apart.
	for _, tc := range []struct{ a, b string }{
		{"#2a55e1", "#111827"}, // brand blue vs body text
		{"#2a55e1", "#e1552a"}, // complementary
		{"#ffffff", "#000000"},
		{"#2a55e1", "#2ae155"}, // same channels, permuted — the case RGB distance gets wrong
	} {
		if d := colorDistance(mustParse(tc.a), mustParse(tc.b)); d <= colorNearMissThreshold {
			t.Errorf("%s vs %s distance %.5f is within the near-miss band; distinct colours would be reported as typos",
				tc.a, tc.b, d)
		}
	}

	// Identical colours in different notations are the same colour.
	if d := colorDistance(mustParse("#2a55e1"), mustParse("rgb(42, 85, 225)")); d != 0 {
		t.Errorf("hex and rgb notations of one colour differ by %.5f", d)
	}
}

func TestParseColor_NotationsAndAlpha(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw           string
		r, g, b       float64
		alpha         float64
		shouldSucceed bool
	}{
		{raw: "#2a55e1", r: 42, g: 85, b: 225, alpha: 1, shouldSucceed: true},
		{raw: "#fff", r: 255, g: 255, b: 255, alpha: 1, shouldSucceed: true},
		{raw: "rgb(42, 85, 225)", r: 42, g: 85, b: 225, alpha: 1, shouldSucceed: true},
		{raw: "rgba(42, 85, 225, 0.5)", r: 42, g: 85, b: 225, alpha: 0.5, shouldSucceed: true},
		{raw: "rgba(0, 0, 0, 0)", r: 0, g: 0, b: 0, alpha: 0, shouldSucceed: true},
		{raw: "transparent", shouldSucceed: false},
		{raw: "currentColor", shouldSucceed: false},
		{raw: "", shouldSucceed: false},
	}
	for _, tc := range cases {
		got, ok := parseColor(tc.raw)
		if ok != tc.shouldSucceed {
			t.Errorf("parseColor(%q) ok = %v, want %v", tc.raw, ok, tc.shouldSucceed)
			continue
		}
		if !ok {
			continue
		}
		if got.R != tc.r || got.G != tc.g || got.B != tc.b || got.A != tc.alpha {
			t.Errorf("parseColor(%q) = %+v, want r=%v g=%v b=%v a=%v", tc.raw, got, tc.r, tc.g, tc.b, tc.alpha)
		}
	}
}

// TestLengthNearMiss_IsRelativeNotAbsolute pins the reason the length threshold
// is a ratio: the same 2px error means something different at 4px and at 64px.
func TestLengthNearMiss_IsRelativeNotAbsolute(t *testing.T) {
	t.Parallel()
	tokens := buildTokenTable(map[string]string{
		"--spacing-xs": "4px",
		"--spacing-md": "16px",
		"--spacing-xl": "64px",
	})

	cases := []struct {
		name      string
		value     float64
		wantToken string
		wantFound bool
	}{
		{name: "the issue's example, 15 against 16", value: 15, wantToken: "--spacing-md", wantFound: true},
		{name: "2px off a 64px token is a slip", value: 66, wantToken: "--spacing-xl", wantFound: true},
		{name: "2px off a 4px token is a different value", value: 6, wantFound: false},
		{name: "15 against 48 is not a near-miss of anything", value: 48, wantFound: false},
		{name: "exact match resolves to its own token", value: 16, wantToken: "--spacing-md", wantFound: true},
	}
	for _, tc := range cases {
		name, _, found := nearestLengthToken(tokens, "padding-top", tc.value)
		if found != tc.wantFound {
			t.Errorf("%s: nearestLengthToken(%v) found = %v, want %v", tc.name, tc.value, found, tc.wantFound)
			continue
		}
		if found && name != tc.wantToken {
			t.Errorf("%s: matched %s, want %s", tc.name, name, tc.wantToken)
		}
	}
}

func TestAnalyzeTokens_FlagsNearMissAndIgnoresExactMatch(t *testing.T) {
	t.Parallel()
	tokens := buildTokenTable(map[string]string{
		"--spacing-md":         "16px",
		"--color-primary-main": "#2a55e1",
	})

	drift := makeElement(0, "div.card--drift", map[string]string{
		"padding-top":      "15px",
		"padding-left":     "15px",
		"background-color": "#2b56e2",
	})
	exact := makeElement(1, "div.card--exact", map[string]string{
		"padding-top":      "16px",
		"padding-left":     "16px",
		"background-color": "#2a55e1",
	})

	findings, skip := analyzeTokens([]elementView{drift, exact}, tokens, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	for _, f := range findings {
		if f.ElementIndex == 1 {
			t.Errorf("the exact-match element produced a finding, so the success state is not silent: %+v", f)
		}
		// The page declared the token; it did not declare that this element must
		// use it. That last step is the analyzer's inference, so the finding is a
		// warning — see TestAnalyzeTokens_PageTokenNearMissIsInferredNotDeclared.
		if f.Severity != severityWarning {
			t.Errorf("an inferred near-miss should be a warning, got %q", f.Severity)
		}
		if f.ExpectedFrom != provenanceInferred {
			t.Errorf("a proximity guess is an inference, got provenance %q", f.ExpectedFrom)
		}
	}

	if got := propertiesOf(findings); !equalStringSets(got, []string{"padding-top", "padding-left", "background-color"}) {
		t.Errorf("flagged properties = %v, want the two paddings and the background", got)
	}
	for _, f := range findings {
		if !strings.Contains(f.Expected, "--spacing-md") && !strings.Contains(f.Expected, "--color-primary-main") {
			t.Errorf("finding does not name the token it near-missed: %+v", f)
		}
	}
}

// TestAnalyzeTokens_NoTokensIsSkippedNotClean is the honesty check: a page with
// no design system must not be reported as compliant, and must not have every
// literal value reported as hardcoded either.
func TestAnalyzeTokens_NoTokensIsSkippedNotClean(t *testing.T) {
	t.Parallel()
	el := makeElement(0, "div.card", map[string]string{"padding-top": "15px", "background-color": "#2b56e2"})

	findings, skip := analyzeTokens([]elementView{el}, buildTokenTable(nil), nil)
	if len(findings) != 0 {
		t.Errorf("a page with no tokens produced %d finding(s); every literal value would be noise", len(findings))
	}
	if skip == nil {
		t.Fatal("a page with no tokens reported no skip, which reads as a clean bill of health")
	}
	if skip.Reason != reasonNoTokens {
		t.Errorf("skip reason = %q, want %q", skip.Reason, reasonNoTokens)
	}
}

func TestAnalyzeTokens_ZeroLengthsAreNotTokenMisses(t *testing.T) {
	t.Parallel()
	tokens := buildTokenTable(map[string]string{"--spacing-xs": "4px"})
	el := makeElement(0, "div.card", map[string]string{"margin-top": "0px", "padding-left": "0px"})

	findings, skip := analyzeTokens([]elementView{el}, tokens, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Errorf("zero is the CSS default and carries no design intent, but produced %d finding(s): %+v", len(findings), findings)
	}
}

// TestDetectSpecConflicts_ReportsDisagreementRatherThanPickingAWinner covers the
// third decision in gmhw.3: when the caller's spec and the page's own tokens
// disagree, one of them is stale and silently resolving it hides which.
func TestDetectSpecConflicts_ReportsDisagreementRatherThanPickingAWinner(t *testing.T) {
	t.Parallel()
	tokens := buildTokenTable(map[string]string{
		"--color-primary-main": "#2a55e1",
		"--spacing-md":         "15px",
	})
	spec := &designSpec{
		Colors:       []string{"#c81e1e"},
		SpacingScale: []float64{8, 16, 32},
	}

	findings := detectSpecConflicts(spec, tokens)
	if len(findings) != 2 {
		t.Fatalf("expected a conflict for the colour token and the spacing token, got %d: %+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Selector != ":root" {
			t.Errorf("a token conflict is about the page's declaration, not an element: %+v", f)
		}
		if f.Severity != severityError {
			t.Errorf("conflict severity = %q, want error", f.Severity)
		}
		if !strings.Contains(f.Message, "disagree") {
			t.Errorf("conflict message should say the two sources disagree: %q", f.Message)
		}
	}
}

func TestDetectSpecConflicts_SilentWhenSpecAgreesWithTokens(t *testing.T) {
	t.Parallel()
	tokens := buildTokenTable(map[string]string{
		"--color-primary-main": "#2a55e1",
		"--spacing-md":         "16px",
	})
	spec := &designSpec{Colors: []string{"#2a55e1"}, SpacingScale: []float64{8, 16, 32}}

	if findings := detectSpecConflicts(spec, tokens); len(findings) != 0 {
		t.Errorf("spec and tokens agree but %d conflict(s) reported: %+v", len(findings), findings)
	}
}

func TestParseLength_OnlyAcceptsResolvedPixels(t *testing.T) {
	t.Parallel()
	// getComputedStyle resolves rem, em, % and calc() to px before we see them,
	// so anything that is not px is a value we cannot compare, not a bug.
	for _, raw := range []string{"16px", "15.5px", "-4px", "0px"} {
		if _, ok := parseLength(raw); !ok {
			t.Errorf("parseLength(%q) failed on a resolved pixel value", raw)
		}
	}
	for _, raw := range []string{"1rem", "auto", "normal", "50%", "", "16"} {
		if _, ok := parseLength(raw); ok {
			t.Errorf("parseLength(%q) succeeded on a value that is not resolved pixels", raw)
		}
	}
}

func propertiesOf(findings []finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Property)
	}
	return out
}

func equalStringSets(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(got))
	for _, v := range got {
		seen[v]++
	}
	for _, v := range want {
		seen[v]--
		if seen[v] < 0 {
			return false
		}
	}
	return true
}

// TestOKLab_MatchesPublishedReferenceValues pins the colour transform against
// an EXTERNAL reference (Björn Ottosson's published sRGB→OKLab values) rather
// than against itself.
//
// Without this the transform is only ever checked through distance
// comparisons, so any coefficient or the sRGB linearisation cutoff can be
// changed and every relative judgement still looks plausible.
func TestOKLab_MatchesPublishedReferenceValues(t *testing.T) {
	t.Parallel()
	const tolerance = 0.0005
	cases := []struct {
		hex     string
		l, a, b float64
	}{
		{"#ffffff", 1.0000, 0.0000, 0.0000},
		{"#000000", 0.0000, 0.0000, 0.0000},
		{"#ff0000", 0.6280, 0.2249, 0.1258},
		{"#00ff00", 0.8664, -0.2339, 0.1795},
		{"#0000ff", 0.4520, -0.0324, -0.3115},
	}
	for _, tc := range cases {
		c, ok := parseColor(tc.hex)
		if !ok {
			t.Fatalf("parseColor(%s) failed", tc.hex)
		}
		l, a, b := oklab(c)
		if absFloat(l-tc.l) > tolerance || absFloat(a-tc.a) > tolerance || absFloat(b-tc.b) > tolerance {
			t.Errorf("oklab(%s) = (%.4f, %.4f, %.4f), want (%.4f, %.4f, %.4f) within %v",
				tc.hex, l, a, b, tc.l, tc.a, tc.b, tolerance)
		}
	}
}

// TestColorNearMissThreshold_IsBracketedOnBothSides makes the threshold's value
// load-bearing. The original calibration compared a pair at distance 0.003
// against pairs at 0.35 — a 100x gap in which any threshold passes, so the
// constant was free to drift by 4x undetected.
func TestColorNearMissThreshold_IsBracketedOnBothSides(t *testing.T) {
	t.Parallel()
	base, _ := parseColor("#2a55e1")

	inside, _ := parseColor("#2d58e4")
	if d := colorDistance(base, inside); d > colorNearMissThreshold {
		t.Errorf("#2d58e4 is %.5f from the token, outside the %.3f band; a plausible typo would be missed", d, colorNearMissThreshold)
	}
	outside, _ := parseColor("#3060e8")
	if d := colorDistance(base, outside); d <= colorNearMissThreshold {
		t.Errorf("#3060e8 is %.5f from the token, inside the %.3f band; a visibly different colour would be called a typo", d, colorNearMissThreshold)
	}
}

// TestScaleContains_ToleranceBoundary pins subPixelTolerance from both sides.
func TestScaleContains_ToleranceBoundary(t *testing.T) {
	t.Parallel()
	scale := []float64{8, 16, 32}
	for _, tc := range []struct {
		value float64
		want  bool
		why   string
	}{
		{16.0, true, "exact"},
		{16.4, true, "within sub-pixel tolerance"},
		{15.6, true, "within sub-pixel tolerance below"},
		{16.6, false, "beyond tolerance"},
		{15.4, false, "beyond tolerance below"},
	} {
		if got := scaleContains(scale, tc.value); got != tc.want {
			t.Errorf("scaleContains(%v) = %v, want %v (%s)", tc.value, got, tc.want, tc.why)
		}
	}
}

// TestLengthNearMissRatio_IsBracketed keeps the relative band load-bearing.
func TestLengthNearMissRatio_IsBracketed(t *testing.T) {
	t.Parallel()
	tokens := buildTokenTable(map[string]string{"--spacing-md": "16px"})
	if _, _, found := nearestLengthToken(tokens, "padding-top", 14); !found {
		t.Errorf("14px is %.0f%% off a 16px token and should be a near-miss", 100*2.0/16)
	}
	if _, _, found := nearestLengthToken(tokens, "padding-top", 13); found {
		t.Errorf("13px is %.0f%% off a 16px token and is a different value, not a typo", 100*3.0/16)
	}
}

// TestAnalyzeTokens_DeclaredSpecIgnoresDefaultsAndTransparency covers the
// declared-spec branch, which had ZERO coverage while carrying the package's
// worst false-positive hazard. Every element on every page has margin:0 and a
// transparent background; losing either guard yields ~8 spurious errors per
// element, and nothing caught that.
func TestAnalyzeTokens_DeclaredSpecIgnoresDefaultsAndTransparency(t *testing.T) {
	t.Parallel()
	spec := &designSpec{SpacingScale: []float64{8, 16, 32}, Colors: []string{"#2a55e1"}}
	el := makeElement(0, "div.plain", map[string]string{
		"margin-top": "0px", "margin-left": "0px",
		"padding-top": "0px", "padding-bottom": "0px",
		"background-color": "rgba(0, 0, 0, 0)",
	})

	findings, skip := analyzeTokens([]elementView{el}, buildTokenTable(nil), spec)
	if skip != nil {
		t.Fatalf("a declared spec must run even with no page tokens, got skip %+v", skip)
	}
	if len(findings) != 0 {
		t.Errorf("CSS defaults were reported as spec violations (%d findings): %+v", len(findings), findings)
	}
}

// TestAnalyzeTokens_DeclaredSpecFlagsOffScaleAndOffPalette is the happy path of
// that same previously-uncovered branch.
func TestAnalyzeTokens_DeclaredSpecFlagsOffScaleAndOffPalette(t *testing.T) {
	t.Parallel()
	spec := &designSpec{SpacingScale: []float64{8, 16, 32}, Colors: []string{"#2a55e1"}}
	el := makeElement(0, "div.card", map[string]string{
		"padding-top":      "15px",
		"background-color": "#c81e1e",
	})

	findings, skip := analyzeTokens([]elementView{el}, buildTokenTable(nil), spec)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if !equalStringSets(propertiesOf(findings), []string{"padding-top", "background-color"}) {
		t.Fatalf("flagged %v, want padding-top and background-color", propertiesOf(findings))
	}
	for _, f := range findings {
		if f.Severity != severityError || f.ExpectedFrom != provenanceDeclared {
			t.Errorf("a declared-rule violation must be a declared error, got %s/%s", f.Severity, f.ExpectedFrom)
		}
	}
}

// --- Defect 1 (kaboom-63ip): a length token governs one property family ---

// TestAnalyzeTokens_AFontSizeTokenDoesNotGovernPadding is the false positive.
//
// Comparing padding against EVERY length token means the type scale and the
// radius scale get a vote on spacing. A design system that declares both
// --font-size-lg:18px and --radius-lg:16px turns any 17px padding into a
// "near-miss of --font-size-lg", which is advice the reader cannot act on: no
// font-size token has ever governed a padding.
func TestAnalyzeTokens_AFontSizeTokenDoesNotGovernPadding(t *testing.T) {
	t.Parallel()
	tokens := buildTokenTable(map[string]string{
		"--font-size-lg": "18px",
		"--radius-lg":    "16px",
	})
	el := makeElement(0, "div.card", map[string]string{"padding-top": "17px"})

	findings, skip := analyzeTokens([]elementView{el}, tokens, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Errorf("a type-scale/radius token was allowed to govern padding, producing %d finding(s): %+v",
			len(findings), findings)
	}
}

// TestAnalyzeTokens_AFontSizeTokenDoesNotExcuseASpacingNearMiss is the false
// negative, and the more dangerous half.
//
// --font-size-sm:14px is in nearly every design system. Letting it answer the
// question "is 14px a token?" silently excuses every 14px padding that is
// really a near-miss of --spacing-md:16px — the exact drift this analyzer
// exists to catch.
func TestAnalyzeTokens_AFontSizeTokenDoesNotExcuseASpacingNearMiss(t *testing.T) {
	t.Parallel()
	tokens := buildTokenTable(map[string]string{
		"--spacing-md":   "16px",
		"--font-size-sm": "14px",
	})
	el := makeElement(0, "div.card", map[string]string{"padding-top": "14px"})

	findings, skip := analyzeTokens([]elementView{el}, tokens, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 1 {
		t.Fatalf("14px padding near-misses --spacing-md:16px but produced %d finding(s): %+v",
			len(findings), findings)
	}
	if !strings.Contains(findings[0].Expected, "--spacing-md") {
		t.Errorf("finding names %q; the spacing token is the one that governs padding", findings[0].Expected)
	}
}

// TestAnalyzeTokens_AnUnclassifiableTokenGovernsNothing pins the conservative
// half of the family model. CSS gives a custom property no type, so a name we
// cannot classify (--sidebar-width, --breakpoint-md, --z-index-modal) has to
// govern nothing rather than everything.
func TestAnalyzeTokens_AnUnclassifiableTokenGovernsNothing(t *testing.T) {
	t.Parallel()
	tokens := buildTokenTable(map[string]string{"--sidebar-width": "240px"})
	el := makeElement(0, "div.card", map[string]string{"padding-left": "250px"})

	findings, skip := analyzeTokens([]elementView{el}, tokens, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Errorf("a token of unknown family was allowed to govern padding: %+v", findings)
	}
}

// TestLengthTokenFamily_PrecedenceKeepsOverlappingNamesInTheRightFamily pins the
// ORDER of tokenFamilyRules, which the hints make load-bearing: --letter-spacing
// contains "spacing" and --font-size contains neither, so a table that matched
// spacing first would hand --letter-spacing to the spacing family and let a
// type-scale token govern padding — the exact defect the family model exists to
// stop. Reordering the table is invisible to every other case here.
func TestLengthTokenFamily_PrecedenceKeepsOverlappingNamesInTheRightFamily(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{
		"--letter-spacing-wide": familyTypography,
		"--text-gap":            familyTypography,
		"--spacing-md":          familySpacing,
		"--grid-gutter":         familySpacing,
		"--radius-lg":           familyRadius,
		"--sidebar-width":       familyUnclassified,
	} {
		if got := lengthTokenFamily(name); got != want {
			t.Errorf("lengthTokenFamily(%q) = %q, want %q", name, got, want)
		}
	}
	if familyGoverns(familyTypography, "padding-top") {
		t.Error("a typography token governs padding-top; --letter-spacing would then set the padding norm")
	}
}

// --- Defect 2 (kaboom-w1et): a near-miss of a page token is an inference ---

// TestAnalyzeTokens_PageTokenNearMissIsInferredNotDeclared separates what the
// page stated from what the analyzer guessed.
//
// The page declared --spacing-md:16px. It never declared that THIS element's
// padding must use it — that is the analyzer's inference from proximity. Since
// error means "a stated rule was broken" and is the batch a caller may fix
// without review, a proximity guess must be a warning; a caller-supplied spec
// is the only thing that makes it an error.
func TestAnalyzeTokens_PageTokenNearMissIsInferredNotDeclared(t *testing.T) {
	t.Parallel()
	tokens := buildTokenTable(map[string]string{
		"--spacing-md":         "16px",
		"--color-primary-main": "#2a55e1",
	})
	el := makeElement(0, "div.card", map[string]string{
		"padding-top":      "15px",
		"background-color": "#2b56e2",
	})

	findings, skip := analyzeTokens([]elementView{el}, tokens, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 2 {
		t.Fatalf("expected the padding and the background, got %d: %+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.ExpectedFrom != provenanceInferred {
			t.Errorf("%s: provenance %q — the page declared the token, not that this element must use it",
				f.Property, f.ExpectedFrom)
		}
		if f.Severity != severityWarning {
			t.Errorf("%s: severity %q — error means a stated rule was broken and is safe to batch-fix",
				f.Property, f.Severity)
		}
	}

	// The bracket: a caller-supplied spec IS a stated rule, so it stays an error.
	spec := &designSpec{SpacingScale: []float64{8, 16, 32}, Colors: []string{"#2a55e1"}}
	declared, _ := analyzeTokens([]elementView{el}, buildTokenTable(nil), spec)
	if len(declared) == 0 {
		t.Fatal("a declared spec produced no findings")
	}
	for _, f := range declared {
		if f.ExpectedFrom != provenanceDeclared || f.Severity != severityError {
			t.Errorf("%s: a caller-supplied spec violation must stay declared/error, got %s/%s",
				f.Property, f.ExpectedFrom, f.Severity)
		}
	}
}

// --- Defect 3 (kaboom-d7f9): a spec judges choices, not layout output ---

// TestAnalyzeTokens_DeclaredScaleIgnoresValuesTheScaleNeverDescribed keeps the
// file's contract ("only near-misses, never every literal value") true on the
// declared-spec branch too.
//
// margin-left/right:137.5px is what `margin: 0 auto` resolves to; margin-top:
// -1px is the border-collapse pull. Neither is a spacing choice, and no
// positive scale can ever contain them, so reporting them makes a permanent
// unfixable error out of correct CSS and buries the 15px that is real drift.
func TestAnalyzeTokens_DeclaredScaleIgnoresValuesTheScaleNeverDescribed(t *testing.T) {
	t.Parallel()
	spec := &designSpec{SpacingScale: []float64{8, 16, 32}}
	el := makeElement(0, "div.centred", map[string]string{
		"margin-left":   "137.5px",
		"margin-right":  "137.5px",
		"margin-top":    "-1px",
		"margin-bottom": "0px",
		"padding-top":   "15px",
	})

	findings, skip := analyzeTokens([]elementView{el}, buildTokenTable(nil), spec)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if got := propertiesOf(findings); !equalStringSets(got, []string{"padding-top"}) {
		t.Errorf("flagged %v; only padding-top is reaching for a scale step and missing it", got)
	}
}

// TestAnalyzeTokens_DeclaredSpecDoesNotBlameTheElementForThePagesOwnToken is
// the exact-match control under a spec.
//
// When the caller's spec and the page's :root disagree, detectSpecConflicts
// reports it once against :root. Re-reporting it on every element that renders
// the page token multiplies one disagreement by the element count and blames
// the element for obeying its own design system — including the element that
// is the analyzer's exact-token-match success state.
func TestAnalyzeTokens_DeclaredSpecDoesNotBlameTheElementForThePagesOwnToken(t *testing.T) {
	t.Parallel()
	tokens := buildTokenTable(map[string]string{
		"--spacing-md":         "16px",
		"--spacing-lg":         "24px",
		"--color-primary-main": "#2a55e1",
		"--color-text":         "#111827",
	})
	spec := &designSpec{SpacingScale: []float64{8, 16, 32}, Colors: []string{"#111827"}}

	card := func(index int, pad, background string) elementView {
		return makeElement(index, "div.token-card", map[string]string{
			"margin-top": "0px", "margin-right": "0px", "margin-bottom": "0px", "margin-left": "0px",
			"padding-top": pad, "padding-right": pad, "padding-bottom": pad, "padding-left": pad,
			"color": "rgb(17, 24, 39)", "border-color": "rgb(17, 24, 39)",
			"background-color": background,
		})
	}
	drift := card(0, "15px", "rgb(43, 86, 226)") // #2b56e2 — nobody's token
	exact := card(1, "16px", "rgb(42, 85, 225)") // #2a55e1 — exactly --color-primary-main

	findings, skip := analyzeTokens([]elementView{drift, exact}, tokens, spec)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}

	var onExact, onDrift []finding
	rootConflicts := 0
	for _, f := range findings {
		switch {
		case f.Selector == ":root":
			rootConflicts++
		case f.ElementIndex == 1:
			onExact = append(onExact, f)
		default:
			onDrift = append(onDrift, f)
		}
	}
	if len(onExact) != 0 {
		t.Errorf("the exact-token-match control produced %d finding(s) under a spec: %+v", len(onExact), onExact)
	}
	if got := propertiesOf(onDrift); !equalStringSets(got,
		[]string{"padding-top", "padding-right", "padding-bottom", "padding-left", "background-color"}) {
		t.Errorf("drift element flagged %v, want the four paddings and the background", got)
	}
	// The spec/page disagreement is still reported — once, where it belongs.
	if rootConflicts != 2 {
		t.Errorf("expected :root conflicts for --spacing-lg and --color-primary-main, got %d", rootConflicts)
	}
}

// --- Defect 4: alpha is part of a colour ---

// TestColorDistance_CountsAlpha closes the hole where oklab() never reads c.A,
// so every translucent tint measured as an exact match of the opaque colour it
// derives from.
func TestColorDistance_CountsAlpha(t *testing.T) {
	t.Parallel()
	tint, ok := parseColor("rgba(42, 85, 225, 0.1)")
	if !ok {
		t.Fatal("parseColor(rgba) failed")
	}
	opaque, ok := parseColor("#2a55e1")
	if !ok {
		t.Fatal("parseColor(hex) failed")
	}

	d := colorDistance(tint, opaque)
	if d == 0 {
		t.Fatal("a 10% tint measures as an exact match of the opaque colour; alpha is ignored")
	}
	if d <= colorNearMissThreshold {
		t.Errorf("a 10%% tint is %.5f from the opaque colour, inside the %.3f near-miss band; a deliberate tint is not a typo",
			d, colorNearMissThreshold)
	}
	if same := colorDistance(tint, tint); same != 0 {
		t.Errorf("a colour differs from itself by %.5f", same)
	}
}

// TestAnalyzeTokens_AnAlphaTokenDoesNotShadowTheOpaqueTokenItDerivesFrom is the
// consequence: --color-brand-alpha sorts first and, with alpha ignored, ties
// the opaque token on distance, so the near-miss the issue is named for gets
// attributed to a token the element could not possibly have meant.
func TestAnalyzeTokens_AnAlphaTokenDoesNotShadowTheOpaqueTokenItDerivesFrom(t *testing.T) {
	t.Parallel()
	tokens := buildTokenTable(map[string]string{
		"--color-brand-alpha":  "rgba(42, 85, 225, 0.1)",
		"--color-primary-main": "#2a55e1",
	})
	el := makeElement(0, "div.card", map[string]string{"background-color": "#2b56e2"})

	findings, skip := analyzeTokens([]elementView{el}, tokens, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one background-color finding, got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Expected, "--color-primary-main") {
		t.Errorf("near-miss attributed to %q; #2b56e2 is a typo of the opaque token, not of a 10%% tint",
			findings[0].Expected)
	}
}

// TestParseAlpha_PercentAndDecimalForms pins the percent branch.
func TestParseAlpha_PercentAndDecimalForms(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw  string
		want float64
	}{
		{"0.5", 0.5}, {"1", 1}, {"50%", 0.5}, {"0%", 0}, {"100%", 1},
	} {
		if got := parseAlpha(tc.raw); got != tc.want {
			t.Errorf("parseAlpha(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
	c, ok := parseColor("rgba(1, 2, 3, 50%)")
	if !ok || c.A != 0.5 {
		t.Errorf("rgba(...,50%%) parsed as %+v, want alpha 0.5", c)
	}
}
