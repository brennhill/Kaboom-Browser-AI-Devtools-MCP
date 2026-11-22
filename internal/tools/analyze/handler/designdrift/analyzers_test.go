// analyzers_test.go — Style-consistency (#693) hazards, and the shipped
// fixture's agreement with the expected-findings table.
//
// Every hazard test here is a false-positive hazard first and a detection check
// second. An analyzer that flags everything detects all three issues perfectly
// and is useless, so the controls are the part that constrains the design.
//
// The fixture-agreement half is the cross-analyzer half: it drives the real
// page's captured computed styles through every category and asserts the shared
// table in both directions. It lives here rather than in contract_test.go
// because that file holds the finding contract, the response envelope and the
// mode surface, and the two together exceed the 800-line file limit — the same
// split that moved the spacing hazards into spacing_test.go.

package designdrift

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeElement builds an in-flow element view for the analyzers.
func makeElement(index int, selector string, styles map[string]string) elementView {
	return elementView{Selector: selector, Index: index, Styles: styles, InFlow: true}
}

func header(index, size int, family string) elementView {
	return makeElement(index, "p.step-card__header", map[string]string{
		"font-family": family,
		"font-size":   itoaPx(size),
		"font-weight": "600",
		"line-height": "20px",
		"color":       "rgb(17, 24, 39)",
	})
}

func itoaPx(v int) string { return formatPx(float64(v)) }

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

// TestAnalyzeConsistency_DetectsTheIssueExample is GitHub #693 verbatim.
func TestAnalyzeConsistency_DetectsTheIssueExample(t *testing.T) {
	t.Parallel()
	findings, skip := analyzeConsistency([]elementView{
		header(0, 12, "Inter, sans-serif"),
		header(1, 11, "Roboto, sans-serif"),
		header(2, 12, "Inter, sans-serif"),
	}, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}

	if !equalStringSets(propertiesOf(findings), []string{"font-family", "font-size"}) {
		t.Fatalf("flagged %v, want exactly font-family and font-size", propertiesOf(findings))
	}
	for _, f := range findings {
		if f.ElementIndex != 1 {
			t.Errorf("finding blames element %d; the odd header is element 1: %+v", f.ElementIndex, f)
		}
		if f.Severity != severityWarning {
			t.Errorf("an inferred majority is a warning, got %q", f.Severity)
		}
		if f.ExpectedFrom != provenanceInferred {
			t.Errorf("expected provenance inferred, got %q", f.ExpectedFrom)
		}
		if !strings.Contains(f.Evidence, "2 of 3") {
			t.Errorf("evidence should quote the majority it relied on, got %q", f.Evidence)
		}
	}
}

// TestAnalyzeConsistency_ConfidenceTracksMajorityStrength: 9 of 10 is not 2 of 3.
func TestAnalyzeConsistency_ConfidenceTracksMajorityStrength(t *testing.T) {
	t.Parallel()

	weak := []elementView{header(0, 12, "Inter, sans-serif"), header(1, 11, "Roboto, sans-serif"), header(2, 12, "Inter, sans-serif")}
	weakFindings, _ := analyzeConsistency(weak, nil)
	for _, f := range weakFindings {
		if f.Confidence != confidenceLow {
			t.Errorf("a 2-of-3 majority should be low confidence, got %q", f.Confidence)
		}
	}

	strong := make([]elementView, 0, 10)
	for i := 0; i < 9; i++ {
		strong = append(strong, header(i, 12, "Inter, sans-serif"))
	}
	strong = append(strong, header(9, 11, "Roboto, sans-serif"))
	strongFindings, _ := analyzeConsistency(strong, nil)
	if len(strongFindings) == 0 {
		t.Fatal("a 9-of-10 majority found no outlier")
	}
	for _, f := range strongFindings {
		if f.Confidence != confidenceHigh {
			t.Errorf("a 9-of-10 majority should be high confidence, got %q", f.Confidence)
		}
	}
}

// TestAnalyzeConsistency_TwoElementsHaveNoMajority: with two values, each is as
// likely to be right, so a verdict would be a coin flip presented as a fact.
func TestAnalyzeConsistency_TwoElementsHaveNoMajority(t *testing.T) {
	t.Parallel()
	findings, skip := analyzeConsistency([]elementView{
		header(0, 12, "Inter, sans-serif"),
		header(1, 11, "Roboto, sans-serif"),
	}, nil)
	if len(findings) != 0 {
		t.Errorf("a two-element group produced %d finding(s): %+v", len(findings), findings)
	}
	if skip == nil || skip.Reason != reasonInsufficientPeers {
		t.Fatalf("expected an insufficient_peers skip, got %+v", skip)
	}
}

// TestAnalyzeConsistency_StateVariantsAreNotDrift covers the .active hazard.
func TestAnalyzeConsistency_StateVariantsAreNotDrift(t *testing.T) {
	t.Parallel()
	base := func(index int, selector string) elementView {
		return makeElement(index, selector, map[string]string{
			"font-family": "Inter, sans-serif", "font-size": "13px",
			"font-weight": "400", "color": "rgb(17, 24, 39)",
		})
	}
	active := makeElement(1, "div.state-item.state-item--active", map[string]string{
		"font-family": "Inter, sans-serif", "font-size": "13px",
		"font-weight": "700", "color": "rgb(42, 85, 225)",
	})

	findings, skip := analyzeConsistency([]elementView{
		base(0, "div.state-item"), active, base(2, "div.state-item"), base(3, "div.state-item"),
	}, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Errorf("the deliberately-styled active item was reported as drift: %+v", findings)
	}
}

// TestAnalyzeConsistency_EvenSplitIsNotAnOutlier: two equally-sized variants are
// a design choice, and calling either one wrong is arbitrary.
func TestAnalyzeConsistency_EvenSplitIsNotAnOutlier(t *testing.T) {
	t.Parallel()
	findings, skip := analyzeConsistency([]elementView{
		header(0, 12, "Inter, sans-serif"),
		header(1, 12, "Inter, sans-serif"),
		header(2, 11, "Roboto, sans-serif"),
		header(3, 11, "Roboto, sans-serif"),
	}, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Errorf("an even 2/2 split has no majority but produced %d finding(s): %+v", len(findings), findings)
	}
}

// TestAnalyzeConsistency_DeclaredSpecCatchesAUniformlyWrongPage is the case
// inference cannot reach: when every element is wrong, the majority is wrong.
func TestAnalyzeConsistency_DeclaredSpecCatchesAUniformlyWrongPage(t *testing.T) {
	t.Parallel()
	elements := []elementView{
		header(0, 12, "Comic Sans MS, cursive"),
		header(1, 12, "Comic Sans MS, cursive"),
		header(2, 12, "Inter, sans-serif"),
	}
	spec := &designSpec{FontFamilies: []string{"Inter"}}

	findings, skip := analyzeConsistency(elements, spec)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 2 {
		t.Fatalf("expected both Comic Sans elements flagged against the declared font, got %d: %+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Severity != severityError {
			t.Errorf("breaking a declared rule is an error, got %q", f.Severity)
		}
		if f.ExpectedFrom != provenanceDeclared {
			t.Errorf("expected declared provenance, got %q", f.ExpectedFrom)
		}
		if f.ElementIndex == 2 {
			t.Error("element 2 matches the declared font and must not be flagged for disagreeing with its drifted peers")
		}
	}
}

// TestAnalyzeConsistency_AuditsColour keeps `color` in the audited set. It is
// the one audited property with no dedicated case, so dropping it from
// auditedProperties() changed nothing that any test observed.
func TestAnalyzeConsistency_AuditsColour(t *testing.T) {
	t.Parallel()
	peer := func(i int, colour string) elementView {
		return makeElement(i, "p.label", map[string]string{
			"font-family": "Inter, sans-serif", "font-size": "13px",
			"font-weight": "400", "line-height": "20px", "color": colour,
		})
	}
	findings, skip := analyzeConsistency([]elementView{
		peer(0, "rgb(17, 24, 39)"), peer(1, "rgb(17, 24, 39)"),
		peer(2, "rgb(200, 30, 30)"), peer(3, "rgb(17, 24, 39)"),
	}, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 1 || findings[0].Property != "color" {
		t.Fatalf("expected one colour finding, got %+v", findings)
	}
	if findings[0].ElementIndex != 2 {
		t.Errorf("blamed element %d, want 2", findings[0].ElementIndex)
	}
}

// TestAnalyzeConsistency_AuditsLineHeight keeps line-height in the audited set.
//
// A 20px line-height among 24s shifts every baseline below it and is invisible
// to any test that checks text is present. It had no dedicated case, so dropping
// it from auditedProperties() changed nothing that any test observed.
func TestAnalyzeConsistency_AuditsLineHeight(t *testing.T) {
	t.Parallel()
	peer := func(i int, lineHeight string) elementView {
		return makeElement(i, "p.row", map[string]string{
			"font-family": "Inter, sans-serif", "font-size": "13px",
			"font-weight": "400", "line-height": lineHeight, "color": "rgb(17, 24, 39)",
		})
	}
	findings, skip := analyzeConsistency([]elementView{
		peer(0, "24px"), peer(1, "24px"), peer(2, "20px"), peer(3, "24px"),
	}, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 1 || findings[0].Property != "line-height" || findings[0].ElementIndex != 2 {
		t.Fatalf("expected one line-height finding on element 2, got %+v", findings)
	}
}

// TestNormalizeFontFamily_IgnoresCaseAndQuoting: two elements resolving to the
// same face through differently-written stacks are not drifting.
//
// getComputedStyle quotes a family whose name needs it and leaves the author's
// capitalisation alone, so `"Inter", sans-serif` and `Inter, sans-serif` are one
// font on the page. Comparing them raw splits a uniform group into two
// "variants", and whichever spelling is rarer gets reported as drift on every
// element that uses it.
func TestNormalizeFontFamily_IgnoresCaseAndQuoting(t *testing.T) {
	t.Parallel()
	for _, stack := range []string{"Inter, sans-serif", `"Inter", sans-serif`, "inter, Helvetica", "'INTER'"} {
		if got := normalizeFontFamily(stack); got != "inter" {
			t.Errorf("normalizeFontFamily(%q) = %q, want inter", stack, got)
		}
	}
	// Control: a genuinely different face still normalises differently, so the
	// rule is not collapsing every stack onto one value.
	if got := normalizeFontFamily("Roboto, sans-serif"); got == "inter" {
		t.Error("Roboto normalised onto Inter; every font would then look uniform")
	}

	peer := func(i int, family string) elementView {
		return makeElement(i, "p.row", map[string]string{"font-family": family, "font-size": "13px"})
	}
	findings, skip := analyzeConsistency([]elementView{
		peer(0, "Inter, sans-serif"), peer(1, `"Inter", sans-serif`), peer(2, "INTER, Helvetica"),
	}, nil)
	if skip != nil {
		t.Fatalf("unexpected skip: %+v", skip)
	}
	if len(findings) != 0 {
		t.Errorf("one face written three ways was reported as drift: %+v", findings)
	}
}

// TestTieBreaksAreDeterministicOnTheLowestValue pins both majority tie-breaks.
// With counts equal, `>` keeps the first candidate in sorted order; `>=` would
// keep the last. Either is defensible, but the choice must be fixed or the
// reported "expected" value flips between runs of identical input.
func TestTieBreaksAreDeterministicOnTheLowestValue(t *testing.T) {
	t.Parallel()
	value, count := dominantValue(map[string]int{"beta": 2, "alpha": 2})
	if value != "alpha" || count != 2 {
		t.Errorf("dominantValue tie = (%q, %d), want (alpha, 2) — the first value in sorted order", value, count)
	}

	gap, gapCount := modalGap([]siblingGap{{size: 24}, {size: 16}, {size: 24}, {size: 16}})
	if gap != 16 || gapCount != 2 {
		t.Errorf("modalGap tie = (%v, %d), want (16, 2) — the smallest gap in sorted order", gap, gapCount)
	}
}

// TestClassMarksState_MatchesStateNamesNotStateWords pins all three shapes the
// exclusion recognises and, more importantly, the ordinary component names it
// must leave alone. Substring matching silently deleted every one of the
// negatives below from every audit.
func TestClassMarksState_MatchesStateNamesNotStateWords(t *testing.T) {
	t.Parallel()
	states := []string{
		"active", "selected", "disabled", // bare state word
		"tab--active", "card--selected", "pricing-card--primary", // BEM modifier
		"is-active", "is-open", "has-error", // stateful prefix
	}
	for _, class := range states {
		if !classMarksState(class) {
			t.Errorf("%q names a state and should be excluded from the peer group", class)
		}
	}

	components := []string{
		"error-message", "success-banner", "open-hours", "interactive-tile",
		"featured-post", "focus-area", "current-balance", "danger-zone",
		"checked-baggage", "readonly-viewer", "highlight-reel", "expanded-content",
		"is-drifted", "is-squeezed", // the fixture's own defect markers
	}
	for _, class := range components {
		if classMarksState(class) {
			t.Errorf("%q is an ordinary component name; excluding it deletes real drift from the audit", class)
		}
	}
}

// TestEligiblePeers_KeepsComponentsWhoseNamesContainStateWords is the same rule
// at the analyzer boundary: drift inside .error-message peers must be found.
func TestEligiblePeers_KeepsComponentsWhoseNamesContainStateWords(t *testing.T) {
	t.Parallel()
	peer := func(i int, family, size string) elementView {
		return makeElement(i, "div.error-message", map[string]string{
			"font-family": family, "font-size": size,
			"font-weight": "400", "line-height": "20px", "color": "rgb(200, 30, 30)",
		})
	}
	findings, skip := analyzeConsistency([]elementView{
		peer(0, "Inter, sans-serif", "14px"), peer(1, "Inter, sans-serif", "14px"),
		peer(2, "Roboto, sans-serif", "11px"),
		peer(3, "Inter, sans-serif", "14px"),
	}, nil)
	if skip != nil {
		t.Fatalf("unexpected skip %+v — the whole group was excluded by its block name", skip)
	}
	if !equalStringSets(propertiesOf(findings), []string{"font-family", "font-size"}) {
		t.Fatalf("flagged %v, want the Roboto/11px outlier's two properties", propertiesOf(findings))
	}
	for _, f := range findings {
		if f.ElementIndex != 2 {
			t.Errorf("blamed element %d, want 2", f.ElementIndex)
		}
	}
}

// TestEnvelopeReportsWhatWasActuallyJudged covers kaboom-zxa0.
//
// elements_audited counts PROBED elements, not judged ones, and there is no
// field for exclusions. A group where the state-variant filter removed three of
// seven reports "across 7 element(s)" while the evidence says "3 of 4 peers" —
// and when everything is excluded it reports insufficient_peers, sending the
// reader to look for elements that are present.
func TestEnvelopeReportsWhatWasActuallyJudged(t *testing.T) {
	t.Parallel()
	mk := func(i int, sel, family string) elementView {
		return makeElement(i, sel, map[string]string{"font-family": family, "font-size": "14px"})
	}
	els := []elementView{
		mk(0, "div.card", "Inter, sans-serif"), mk(1, "div.card", "Inter, sans-serif"),
		mk(2, "div.card", "Inter, sans-serif"), mk(3, "div.card", "Roboto, sans-serif"),
		mk(4, "div.interactive-card", "Inter, sans-serif"),
		mk(5, "div.opening-hours", "Inter, sans-serif"),
	}
	if excluded := len(els) - len(eligiblePeers(els)); excluded != 0 {
		t.Errorf("%d ordinary component(s) were excluded as state variants: substring matching is too broad", excluded)
	}
}

// TestAllPeersExcludedIsItsOwnReason: when the state filter removes every peer,
// "insufficient_peers" sends the reader hunting for elements that are present
// and visible. The two situations have different fixes and must be
// distinguishable.
func TestAllPeersExcludedIsItsOwnReason(t *testing.T) {
	t.Parallel()
	variant := func(i int) elementView {
		return makeElement(i, "div.tab.tab--active", map[string]string{
			"font-family": "Inter, sans-serif", "font-size": "14px",
		})
	}
	findings, skip := analyzeConsistency([]elementView{variant(0), variant(1), variant(2), variant(3)}, nil)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
	if skip == nil || skip.Reason != reasonAllPeersExcluded {
		t.Fatalf("skip = %+v, want reason %q", skip, reasonAllPeersExcluded)
	}
}

// --- The expected-findings table ---

type expectedTable struct {
	Fixture string `json:"fixture"`
	Cases   []struct {
		Name           string   `json:"name"`
		Kind           string   `json:"kind"`
		Selector       string   `json:"selector"`
		ExpectElements int      `json:"expect_elements"`
		Categories     []string `json:"categories"`
		ExpectFindings []struct {
			Category     string `json:"category"`
			Property     string `json:"property"`
			ElementIndex int    `json:"element_index"`
			Severity     string `json:"severity"`
			ExpectedFrom string `json:"expected_from"`
			Confidence   string `json:"confidence"`
		} `json:"expect_findings"`
		ExpectSkipped []struct {
			Category string `json:"category"`
			Reason   string `json:"reason"`
		} `json:"expect_skipped"`
	} `json:"cases"`
}

func loadExpectedTable(t *testing.T) expectedTable {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "expected-findings.json"))
	if err != nil {
		t.Fatalf("read expected-findings table: %v", err)
	}
	var table expectedTable
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatalf("parse expected-findings table: %v", err)
	}
	return table
}

// TestExpectedFindingsTableIsConsistentWithTheCode keeps the shared table
// honest.
//
// The table is the single source the UAT asserts against, and the UAT runs in a
// browser where a typo in a category or severity name would simply never match
// and quietly report zero findings. Validating it here means a rename in the Go
// code breaks the build rather than silently disarming the UAT.
func TestExpectedFindingsTableIsConsistentWithTheCode(t *testing.T) {
	t.Parallel()
	table := loadExpectedTable(t)
	if len(table.Cases) == 0 {
		t.Fatal("the expected-findings table is empty")
	}

	knownReasons := map[string]bool{reasonNoTokens: true, reasonInsufficientPeers: true, reasonNoElements: true, reasonNoInFlowElements: true, reasonAllPeersExcluded: true}
	knownSeverity := map[string]bool{severityError: true, severityWarning: true}
	knownProvenance := map[string]bool{provenanceDeclared: true, provenanceInferred: true}
	knownConfidence := map[string]bool{confidenceHigh: true, confidenceLow: true}

	positives, controls := 0, 0
	for _, tc := range table.Cases {
		if tc.Name == "" || tc.Selector == "" {
			t.Errorf("case %+v is missing a name or selector", tc)
		}
		switch tc.Kind {
		case "positive":
			positives++
		case "control":
			controls++
		default:
			t.Errorf("case %q has kind %q, want positive or control", tc.Name, tc.Kind)
		}
		for _, category := range tc.Categories {
			if !isKnownCategory(category) {
				t.Errorf("case %q names unknown category %q", tc.Name, category)
			}
		}
		for _, f := range tc.ExpectFindings {
			if !isKnownCategory(f.Category) {
				t.Errorf("case %q expects unknown category %q", tc.Name, f.Category)
			}
			if !knownSeverity[f.Severity] {
				t.Errorf("case %q expects unknown severity %q", tc.Name, f.Severity)
			}
			if !knownProvenance[f.ExpectedFrom] {
				t.Errorf("case %q expects unknown provenance %q", tc.Name, f.ExpectedFrom)
			}
			if !knownConfidence[f.Confidence] {
				t.Errorf("case %q expects unknown confidence %q", tc.Name, f.Confidence)
			}
			if want := severityFor(f.ExpectedFrom); f.ExpectedFrom != "" && f.Severity != want {
				t.Errorf("case %q pairs provenance %q with severity %q; the code derives %q",
					tc.Name, f.ExpectedFrom, f.Severity, want)
			}
			if !isProducibleProperty(f.Category, f.Property) {
				t.Errorf("case %q expects property %q, which the %s analyzer never emits",
					tc.Name, f.Property, f.Category)
			}
		}
		for _, s := range tc.ExpectSkipped {
			if !knownReasons[s.Reason] {
				t.Errorf("case %q expects unknown skip reason %q", tc.Name, s.Reason)
			}
		}
	}

	// The controls are the half that constrains precision. A table that drifts
	// into positives-only would pass an analyzer that flags everything.
	if positives == 0 || controls == 0 {
		t.Fatalf("table has %d positives and %d controls; both are required", positives, controls)
	}
}

// TestFixtureProducesExactlyTheExpectedFindings runs the real page's captured
// computed styles through the analyzers and asserts the expected-findings table
// in both directions.
//
// This is the check that the analyzers and the fixture actually agree. The unit
// tests above use synthetic inputs, which prove the hazards are handled but not
// that the fixture plants what the table claims; the UAT proves the live
// extension path but only runs with a browser attached. This closes the gap in
// CI: every planted positive must be found, and every negative control must
// produce nothing.
func TestFixtureProducesExactlyTheExpectedFindings(t *testing.T) {
	t.Parallel()
	table := loadExpectedTable(t)

	raw, err := os.ReadFile(filepath.Join("testdata", "fixture-probe.json"))
	if err != nil {
		t.Fatalf("read captured fixture payload: %v", err)
	}
	var payloads map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payloads); err != nil {
		t.Fatalf("parse captured fixture payload: %v", err)
	}

	for _, tc := range table.Cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			payload, ok := payloads[tc.Selector]
			if !ok {
				t.Fatalf("no captured payload for selector %q; re-capture the fixture", tc.Selector)
			}

			categories, invalid := resolveCategories(tc.Categories)
			if invalid != "" {
				t.Fatalf("case names invalid category %q", invalid)
			}

			params := auditParams{Selector: tc.Selector, Categories: tc.Categories}
			if spec := specFor(t, table, tc.Name); spec != nil {
				params.Spec = spec
			}

			deps := Deps{
				ProbeStyles:    func(string, int, bool) (json.RawMessage, error) { return payload, nil },
				TrackingStatus: func() (bool, string) { return true, "http://127.0.0.1/tests/design-drift.html" },
			}
			probe, err := captureProbe(deps, tc.Selector)
			if err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			result := runAudit(params, probe, categories)

			// Positive assertions first. A case whose only checks are "these
			// findings are present" and "nothing extra" is satisfied by an empty
			// result — which is how three of the four UAT controls used to pass
			// on a blank response.
			if tc.ExpectElements == 0 {
				t.Fatalf("case %q declares no expect_elements; every case must assert something positive", tc.Name)
			}
			if result.ElementsAudited != tc.ExpectElements {
				t.Errorf("audited %d element(s), table expects %d — fixture and table disagree",
					result.ElementsAudited, tc.ExpectElements)
			}
			if accounted := len(result.ChecksCompleted) + len(result.ChecksSkipped); accounted != len(categories) {
				t.Errorf("%d of %d requested categories accounted for; each must be completed or skipped",
					accounted, len(categories))
			}

			assertFindingsMatch(t, tc.Name, tc.ExpectFindings, collectFindings(result))
			assertSkipsMatch(t, tc.ExpectSkipped, result.ChecksSkipped)
		})
	}
}

// specFor re-reads the raw spec for a case, since the typed table above omits it.
func specFor(t *testing.T, _ expectedTable, caseName string) *designSpec {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "expected-findings.json"))
	if err != nil {
		t.Fatalf("read table: %v", err)
	}
	var raw struct {
		Cases []struct {
			Name string      `json:"name"`
			Spec *designSpec `json:"spec"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse table: %v", err)
	}
	for _, c := range raw.Cases {
		if c.Name == caseName {
			return c.Spec
		}
	}
	return nil
}

type findingKey struct {
	category string
	property string
	index    int
}

func collectFindings(result auditResult) map[findingKey]finding {
	out := make(map[findingKey]finding)
	for _, section := range result.Sections {
		bucket, ok := section.(map[string]any)
		if !ok {
			continue
		}
		findings, ok := bucket["findings"].([]finding)
		if !ok {
			continue
		}
		for _, f := range findings {
			out[findingKey{f.Category, f.Property, f.ElementIndex}] = f
		}
	}
	return out
}

func assertFindingsMatch(t *testing.T, caseName string, want []struct {
	Category     string `json:"category"`
	Property     string `json:"property"`
	ElementIndex int    `json:"element_index"`
	Severity     string `json:"severity"`
	ExpectedFrom string `json:"expected_from"`
	Confidence   string `json:"confidence"`
}, got map[findingKey]finding) {
	t.Helper()

	for _, expected := range want {
		key := findingKey{expected.Category, expected.Property, expected.ElementIndex}
		actual, found := got[key]
		if !found {
			t.Errorf("planted drift was not detected: %s element %d %s",
				expected.Category, expected.ElementIndex, expected.Property)
			continue
		}
		if actual.Severity != expected.Severity {
			t.Errorf("%s element %d %s: severity %q, want %q",
				expected.Category, expected.ElementIndex, expected.Property, actual.Severity, expected.Severity)
		}
		if expected.ExpectedFrom != "" && actual.ExpectedFrom != expected.ExpectedFrom {
			t.Errorf("%s element %d %s: provenance %q, want %q",
				expected.Category, expected.ElementIndex, expected.Property, actual.ExpectedFrom, expected.ExpectedFrom)
		}
		if actual.Confidence != expected.Confidence {
			t.Errorf("%s element %d %s: confidence %q, want %q",
				expected.Category, expected.ElementIndex, expected.Property, actual.Confidence, expected.Confidence)
		}
		delete(got, key)
	}

	// Anything left over is a false positive. For a control case this is the
	// entire point of the case; for a positive it means the analyzer found more
	// than was planted, which is just as wrong as finding less.
	for key, extra := range got {
		t.Errorf("case %q produced an unexpected finding — %s element %d %s (%s): %s",
			caseName, key.category, key.index, key.property, extra.Severity, extra.Message)
	}
}

func assertSkipsMatch(t *testing.T, want []struct {
	Category string `json:"category"`
	Reason   string `json:"reason"`
}, got []skipped) {
	t.Helper()
	remaining := make(map[skipped]bool, len(got))
	for _, s := range got {
		remaining[s] = true
	}
	for _, expected := range want {
		key := skipped{Category: expected.Category, Reason: expected.Reason}
		if !remaining[key] {
			t.Errorf("expected %s to be skipped as %s, got skips %+v", expected.Category, expected.Reason, got)
			continue
		}
		delete(remaining, key)
	}
	for extra := range remaining {
		t.Errorf("unexpected skip: %s was skipped as %s", extra.Category, extra.Reason)
	}
}

// isProducibleProperty reports whether an analyzer can emit a finding for this
// property, so the table cannot reference something that will never appear.
func isProducibleProperty(category, property string) bool {
	switch category {
	case categoryStyleConsistency:
		for _, candidate := range auditedProperties() {
			if candidate == property {
				return true
			}
		}
	case categoryDesignTokens:
		producible := append(append([]string{}, spacingProperties()...), colorProperties()...)
		for _, shorthand := range boxShorthands() {
			// A uniform four-side group collapses onto the shorthand, so the
			// shorthand is an emittable property too.
			producible = append(producible, shorthand.name)
		}
		if namesOneOf(producible, property) {
			return true
		}
		return strings.HasPrefix(property, "--")
	case categorySpacing:
		return strings.HasPrefix(property, "gap-") || strings.HasPrefix(property, "overlap-")
	}
	return false
}

// TestAnalyzeConsistency_DeclaredSpecIsEnforceableOnAPair covers kaboom-d7f9
// sub-defect 2.
//
// analyzeConsistency returned insufficient_peers before any spec was consulted,
// so two elements both rendering Comic Sans against a spec naming Inter
// answered "Design audit ran no checks". A declared rule needs no peer group —
// declaredFindings already argues exactly that for the majority case, where a
// uniformly wrong page has a wrong majority.
func TestAnalyzeConsistency_DeclaredSpecIsEnforceableOnAPair(t *testing.T) {
	t.Parallel()
	pair := []elementView{
		makeElement(0, "p.pair-item", map[string]string{"font-family": "Inter, sans-serif", "font-size": "12px"}),
		makeElement(1, "p.pair-item", map[string]string{"font-family": "Roboto, sans-serif", "font-size": "12px"}),
	}
	spec := &designSpec{FontFamilies: []string{"Inter"}}

	declared, skip := analyzeConsistency(pair, spec)
	if skip != nil {
		t.Fatalf("a stated rule needs no majority, but the category was skipped: %+v", skip)
	}
	if len(declared) != 1 {
		t.Fatalf("expected the one element the spec forbids, got %d: %+v", len(declared), declared)
	}
	if declared[0].ElementIndex != 1 {
		t.Errorf("the finding blames element %d; element 1 is the Roboto one", declared[0].ElementIndex)
	}
	if declared[0].Severity != severityError || declared[0].ExpectedFrom != provenanceDeclared {
		t.Errorf("a declared violation reported %s/%s", declared[0].Severity, declared[0].ExpectedFrom)
	}

	// Control 1: without the spec the same pair still refuses to guess, so the
	// case above is the spec working rather than the peer minimum being gone.
	if inferred, skip := analyzeConsistency(pair, nil); len(inferred) != 0 || skip == nil || skip.Reason != reasonInsufficientPeers {
		t.Errorf("with no spec a pair must stay insufficient_peers; got %d finding(s) and skip %+v", len(inferred), skip)
	}
	// Control 2: a pair that both obey the spec produces nothing, so the path
	// is judging values rather than reporting every element it now reaches.
	compliant := []elementView{
		makeElement(0, "p.pair-item", map[string]string{"font-family": "Inter, sans-serif"}),
		makeElement(1, "p.pair-item", map[string]string{"font-family": "Inter, sans-serif"}),
	}
	if clean, skip := analyzeConsistency(compliant, spec); len(clean) != 0 || skip != nil {
		t.Errorf("a compliant pair is clean; got %d finding(s) and skip %+v", len(clean), skip)
	}
}
