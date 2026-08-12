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
		name, _, found := nearestLengthToken(tokens, tc.value)
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
		if f.Severity != severityError {
			t.Errorf("a broken page token should be an error, got %q", f.Severity)
		}
		if f.ExpectedFrom != provenanceDeclared {
			t.Errorf("a page token is a declaration, got provenance %q", f.ExpectedFrom)
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
