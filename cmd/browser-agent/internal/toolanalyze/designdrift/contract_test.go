// contract_test.go — The finding contract, spec precedence, and the mode surface.

package designdrift

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/styleprobe"
)

// TestSeverityIsDerivedFromProvenance pins the invariant the whole triage axis
// rests on. If a finding could claim an inferred expectation with error
// severity, "fix all errors" would stop being a safe batch operation.
func TestSeverityIsDerivedFromProvenance(t *testing.T) {
	t.Parallel()
	if got := severityFor(provenanceDeclared); got != severityError {
		t.Errorf("declared provenance gave severity %q, want error", got)
	}
	if got := severityFor(provenanceInferred); got != severityWarning {
		t.Errorf("inferred provenance gave severity %q, want warning", got)
	}

	// newFinding is the only constructor, so the pairing cannot be bypassed.
	f := newFinding(categorySpacing, "gap-vertical", makeElement(0, "div", nil),
		"14px", "24px", provenanceInferred, confidenceHigh, "evidence", "message")
	if f.Severity != severityWarning || f.ExpectedFrom != provenanceInferred {
		t.Errorf("newFinding produced a contradictory pair: %+v", f)
	}
}

// TestSpecPrecedenceIsPerPropertyNotPerCall is the rule from gmhw.3: a spec that
// names a spacing scale but no colours must leave colour judgement to
// inference, in the same response.
func TestSpecPrecedenceIsPerPropertyNotPerCall(t *testing.T) {
	t.Parallel()
	partial := &designSpec{SpacingScale: []float64{8, 16, 32}}

	if got := partial.provenanceForSpacing(); got != provenanceDeclared {
		t.Errorf("spacing provenance = %q, want declared — the spec names a scale", got)
	}
	for name, got := range map[string]string{
		"colors": partial.provenanceForColors(),
		"fonts":  partial.provenanceForFonts(),
		"sizes":  partial.provenanceForSizes(),
	} {
		if got != provenanceInferred {
			t.Errorf("%s provenance = %q, want inferred — a partial spec must not disable the families it does not name", name, got)
		}
	}

	var absent *designSpec
	if got := absent.provenanceForSpacing(); got != provenanceInferred {
		t.Errorf("with no spec at all, provenance = %q, want inferred", got)
	}
	if !absent.empty() {
		t.Error("a nil spec should read as empty")
	}
}

// TestMixedSpecProducesBothSeveritiesInOneResponse is the user-facing
// consequence of per-property precedence.
func TestMixedSpecProducesBothSeveritiesInOneResponse(t *testing.T) {
	t.Parallel()
	// Spacing is declared (and violated); fonts are not declared, so the font
	// outlier stays a warning judged against its peers.
	elements := []elementView{
		stackedElement(0, "div.card", 0, 32),
		stackedElement(1, "div.card", 56, 32),
		stackedElement(2, "div.card", 112, 32),
	}
	for i := range elements {
		elements[i].Styles = map[string]string{
			"font-family": "Inter, sans-serif", "font-size": "12px",
			"font-weight": "600", "line-height": "20px", "color": "rgb(17, 24, 39)",
		}
	}
	elements[1].Styles["font-family"] = "Roboto, sans-serif"

	spec := &designSpec{SpacingScale: []float64{8, 16, 32}}
	byCategory := map[string][]finding{}
	spacingFindings, _ := analyzeSpacing(elements, spec)
	consistencyFindings, _ := analyzeConsistency(elements, spec)
	byCategory[categorySpacing] = spacingFindings
	byCategory[categoryStyleConsistency] = consistencyFindings

	result := buildAuditResult(".card", elements, len(elements), false, byCategory, nil)
	// Exact counts, not "at least one of each": >0 would be satisfied by almost
	// any wrong behaviour, and the point of the case is that the two provenances
	// coexist in known quantity.
	if got := result.BySeverity[severityError]; got != 2 {
		t.Errorf("declared spacing violations = %d errors, want 2 (both gaps are off the scale)", got)
	}
	if got := result.BySeverity[severityWarning]; got != 1 {
		t.Errorf("inferred font drift = %d warnings, want 1", got)
	}
	if result.TotalFindings != 3 {
		t.Errorf("total_findings = %d, want 3", result.TotalFindings)
	}
}

// TestBuildAuditResult_SkippedIsNotTheSameAsClean.
func TestBuildAuditResult_SkippedIsNotTheSameAsClean(t *testing.T) {
	t.Parallel()
	result := buildAuditResult(".card", nil, 0, false,
		map[string][]finding{categorySpacing: nil},
		[]skipped{{Category: categoryDesignTokens, Reason: reasonNoTokens}})

	if len(result.ChecksCompleted) != 1 || result.ChecksCompleted[0] != categorySpacing {
		t.Errorf("checks_completed = %v, want just spacing", result.ChecksCompleted)
	}
	if len(result.ChecksSkipped) != 1 || result.ChecksSkipped[0].Reason != reasonNoTokens {
		t.Errorf("checks_skipped = %+v, want the design_tokens skip with its reason", result.ChecksSkipped)
	}
	if _, present := result.Sections[categoryDesignTokens]; present {
		t.Error("a skipped category must not appear as an empty section, which reads as a clean result")
	}
}

// TestSortFindingsIsStableAndErrorsFirst keeps the UAT comparison deterministic.
func TestSortFindingsIsStableAndErrorsFirst(t *testing.T) {
	t.Parallel()
	findings := []finding{
		{Severity: severityWarning, ElementIndex: 0, Property: "font-size"},
		{Severity: severityError, ElementIndex: 5, Property: "padding-top"},
		{Severity: severityWarning, ElementIndex: 0, Property: "color"},
		{Severity: severityError, ElementIndex: 1, Property: "padding-left"},
	}
	sortFindings(findings)

	if findings[0].Severity != severityError || findings[1].Severity != severityError {
		t.Fatalf("errors did not sort first: %+v", findings)
	}
	if findings[0].ElementIndex != 1 || findings[1].ElementIndex != 5 {
		t.Errorf("errors not ordered by element: %+v", findings[:2])
	}
	if findings[2].Property != "color" || findings[3].Property != "font-size" {
		t.Errorf("warnings not ordered by property: %+v", findings[2:])
	}
}

// --- Mode surface ---

func probeDeps(payload string, err error) Deps {
	return Deps{
		ProbeStyles: func(string, int, bool) (json.RawMessage, error) {
			if err != nil {
				return nil, err
			}
			return json.RawMessage(payload), nil
		},
		TrackingStatus: func() (bool, string) { return true, "https://app.example.test" },
	}
}

func request() mcp.JSONRPCRequest {
	return mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
}

func TestHandle_RequiresASelector(t *testing.T) {
	t.Parallel()
	resp := Handle(probeDeps(`{}`, nil), request(), json.RawMessage(`{"what":"design_audit"}`))
	if !strings.Contains(string(resp.Result), "missing_param") {
		t.Fatalf("a design audit without a selector should be a missing_param error: %s", resp.Result)
	}
	// "selector" appears as the envelope's own field name on every response, so
	// assert the structured param rather than a bare substring.
	if !strings.Contains(string(resp.Result), `\"param\":\"selector\"`) {
		t.Errorf("the error should name selector as the missing param: %s", resp.Result)
	}
}

// TestHandle_UnknownCategoryNamesTheValidSet: answering with an empty result for
// a misspelled category would look like a clean page.
func TestHandle_UnknownCategoryNamesTheValidSet(t *testing.T) {
	t.Parallel()
	resp := Handle(probeDeps(`{}`, nil), request(),
		json.RawMessage(`{"what":"design_audit","selector":".card","categories":["spacing","typography"]}`))

	body := string(resp.Result)
	if !strings.Contains(body, "invalid_param") {
		t.Fatalf("unknown category should be invalid_param, got: %s", body)
	}
	if !strings.Contains(body, "typography") {
		t.Errorf("the error should quote the offending category: %s", body)
	}
	for _, category := range allCategories() {
		if !strings.Contains(body, category) {
			t.Errorf("the error should name the valid set; %q is missing: %s", category, body)
		}
	}
}

func TestHandle_RequiresATrackedTab(t *testing.T) {
	t.Parallel()
	deps := probeDeps(`{}`, nil)
	deps.TrackingStatus = func() (bool, string) { return false, "" }

	resp := Handle(deps, request(), json.RawMessage(`{"what":"design_audit","selector":".card"}`))
	if !strings.Contains(string(resp.Result), "No tab is being tracked") {
		t.Fatalf("unexpected response: %s", resp.Result)
	}
}

// TestHandle_ProbeFailureIsReportedNotSwallowed.
func TestHandle_ProbeFailureIsReportedNotSwallowed(t *testing.T) {
	t.Parallel()
	resp := Handle(probeDeps("", errors.New("extension unavailable")), request(),
		json.RawMessage(`{"what":"design_audit","selector":".card"}`))

	body := string(resp.Result)
	if !strings.Contains(body, "extension unavailable") {
		t.Errorf("the underlying failure should reach the caller: %s", body)
	}
	if strings.Contains(body, "no drift") {
		t.Error("a failed probe was reported as a clean page")
	}
}

// TestHandle_EmptyMatchIsSkippedNotClean.
func TestHandle_EmptyMatchIsSkippedNotClean(t *testing.T) {
	t.Parallel()
	resp := Handle(probeDeps(`{"elements":[],"count":0,"match_count":0,"truncated":false}`, nil), request(),
		json.RawMessage(`{"what":"design_audit","selector":".absent"}`))

	body := string(resp.Result)
	if !strings.Contains(body, reasonNoElements) {
		t.Errorf("a selector matching nothing should be reported as skipped with a reason: %s", body)
	}
	if strings.Contains(body, "found no drift") {
		t.Error("a selector that matched nothing was reported as a clean page")
	}
}

// TestHandle_TruncationSurvivesIntoTheEnvelope: a verdict over a truncated set
// must say so.
func TestHandle_TruncationSurvivesIntoTheEnvelope(t *testing.T) {
	t.Parallel()
	payload := `{"elements":[{"selector":"div.card","tag":"div","computed_styles":{},"box_model":{},"index":0,"in_flow":true}],
	             "count":1,"match_count":60,"truncated":true}`
	resp := Handle(probeDeps(payload, nil), request(), json.RawMessage(`{"what":"design_audit","selector":".card"}`))

	body := string(resp.Result)
	for _, want := range []string{`truncated\":true`, `match_count\":60`} {
		if !strings.Contains(body, want) {
			t.Errorf("response missing %q, so a partial-evidence verdict looks authoritative: %s", want, body)
		}
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
		for _, candidate := range append(append([]string{}, spacingProperties()...), colorProperties()...) {
			if candidate == property {
				return true
			}
		}
		return strings.HasPrefix(property, "--")
	case categorySpacing:
		return strings.HasPrefix(property, "gap-") || strings.HasPrefix(property, "overlap-")
	}
	return false
}

// TestViewsFrom_CarriesEveryWireFieldIntoTheAnalyzers covers the wire→analyzer
// translation, which had no test at all. Every analyzer test builds elementView
// directly and every element in the captured fixture is in flow, so viewsFrom
// could hardcode InFlow:true and the whole suite stayed green while the
// out-of-flow protection silently stopped working in production.
func TestViewsFrom_CarriesEveryWireFieldIntoTheAnalyzers(t *testing.T) {
	t.Parallel()
	payload := `{"elements":[
	  {"selector":"div.a","tag":"div","computed_styles":{"color":"#111"},"index":0,"in_flow":true,
	   "parent_display":"flex","parent_gap":"24px","custom_properties":{"--x":"1px"},
	   "box_model":{"top":10,"bottom":42,"left":1,"right":201,"width":200,"height":32}},
	  {"selector":"div.b","tag":"div","computed_styles":{},"index":1,"in_flow":false,
	   "box_model":{}}],
	  "count":2,"match_count":2,"truncated":false}`
	var probe styleprobe.WireStyleProbeResult
	if err := json.Unmarshal([]byte(payload), &probe); err != nil {
		t.Fatal(err)
	}
	views := viewsFrom(probe)
	if len(views) != 2 {
		t.Fatalf("got %d views, want 2", len(views))
	}
	if !views[0].InFlow {
		t.Error("in_flow:true did not survive translation")
	}
	if views[1].InFlow {
		t.Error("in_flow:false did not survive translation — out-of-flow elements would rejoin the rhythm")
	}
	if views[1].Index != 1 {
		t.Errorf("index = %d, want 1", views[1].Index)
	}
	if views[0].ParentDisplay != "flex" || views[0].ParentGap != "24px" {
		t.Errorf("parent layout context lost: %q/%q", views[0].ParentDisplay, views[0].ParentGap)
	}
	if views[0].Box.Top != 10 || views[0].Box.Bottom != 42 {
		t.Errorf("box geometry lost: %+v", views[0].Box)
	}
	if views[0].Styles["color"] != "#111" {
		t.Errorf("computed styles lost: %v", views[0].Styles)
	}
}

// TestCaptureProbe_RejectsAMalformedPayload: a payload that is not JSON must be
// an error, not an empty audit that reads as a clean page.
func TestCaptureProbe_RejectsAMalformedPayload(t *testing.T) {
	t.Parallel()
	deps := Deps{
		ProbeStyles:    func(string, int, bool) (json.RawMessage, error) { return json.RawMessage(`not json`), nil },
		TrackingStatus: func() (bool, string) { return true, "x" },
	}
	if _, err := captureProbe(deps, ".card"); err == nil {
		t.Fatal("a malformed probe payload was accepted; the audit would report a clean page")
	}
}

// TestBuildAuditResult_CountsAndOrdersFindings pins the envelope arithmetic.
// total_findings, by_severity and the sort were all unasserted, so the totals
// could report zero while the sections were full.
func TestBuildAuditResult_CountsAndOrdersFindings(t *testing.T) {
	t.Parallel()
	byCategory := map[string][]finding{
		categorySpacing: {
			{Category: categorySpacing, Severity: severityWarning, ElementIndex: 4, Property: "gap-vertical"},
			{Category: categorySpacing, Severity: severityError, ElementIndex: 2, Property: "gap-vertical"},
		},
		categoryDesignTokens: {
			{Category: categoryDesignTokens, Severity: severityError, ElementIndex: 1, Property: "padding-top"},
		},
	}
	result := buildAuditResult(".card", make([]elementView, 6), 6, false, byCategory, nil)

	if result.TotalFindings != 3 {
		t.Errorf("total_findings = %d, want 3", result.TotalFindings)
	}
	if result.BySeverity[severityError] != 2 || result.BySeverity[severityWarning] != 1 {
		t.Errorf("by_severity = %v, want 2 errors and 1 warning", result.BySeverity)
	}
	if result.ElementsAudited != 6 {
		t.Errorf("elements_audited = %d, want 6", result.ElementsAudited)
	}
	section := result.Sections[categorySpacing].(map[string]any)
	ordered := section["findings"].([]finding)
	if ordered[0].Severity != severityError {
		t.Errorf("findings were not sorted errors-first: %+v", ordered)
	}
}

// TestSummarize_ReportsWhatWasFound: two tests assert the "no drift" headline is
// ABSENT on error paths; none asserted it is correct when there are findings, so
// summarize could always claim a clean page.
func TestSummarize_ReportsWhatWasFound(t *testing.T) {
	t.Parallel()
	withFindings := auditResult{
		TotalFindings: 3, ElementsAudited: 6,
		BySeverity:      map[string]int{severityError: 2, severityWarning: 1},
		ChecksCompleted: []string{categorySpacing},
	}
	got := summarize(withFindings)
	for _, want := range []string{"3 finding", "6 element", "2 error", "1 warning"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q is missing %q", got, want)
		}
	}
	clean := auditResult{ChecksCompleted: []string{categorySpacing}, ElementsAudited: 4, BySeverity: map[string]int{}}
	if !strings.Contains(summarize(clean), "no drift") {
		t.Errorf("a clean audit should say so, got %q", summarize(clean))
	}
	if strings.Contains(summarize(auditResult{BySeverity: map[string]int{}}), "no drift") {
		t.Error("an audit that ran no checks must not claim a clean page")
	}
}

// TestResolveCategories_DefaultsToEveryCategory: no test called the mode without
// narrowing categories, so the default set could have shrunk unnoticed.
func TestResolveCategories_DefaultsToEveryCategory(t *testing.T) {
	t.Parallel()
	selected, invalid := resolveCategories(nil)
	if invalid != "" {
		t.Fatalf("unexpected invalid category %q", invalid)
	}
	if len(selected) != len(allCategories()) {
		t.Fatalf("default set has %d categories, want all %d", len(selected), len(allCategories()))
	}
	for _, category := range allCategories() {
		if !selected[category] {
			t.Errorf("default set is missing %q", category)
		}
	}
}

// --- TDD red: defects confirmed by the code review ---

// TestCaptureProbe_RejectsInBandExtensionErrors covers kaboom-ccog.
//
// The extension reports failures as a JSON body carrying an `error` key, not as
// a transport error. Go ignores unknown fields, so those payloads decode
// cleanly into a zero-valued result and the audit reports "ran no checks" — a
// dead service worker, a timeout and a selector that matched nothing become
// indistinguishable. CLAUDE.md rule 25 forbids masking a failure as an expected
// state.
func TestCaptureProbe_RejectsInBandExtensionErrors(t *testing.T) {
	t.Parallel()
	for _, payload := range []string{
		`{"error":"computed_styles_failed","message":"Could not establish connection. Receiving end does not exist."}`,
		`{"error":"Computed styles query timeout"}`,
		`{"error":"computed_styles_error","message":"Failed to query computed styles"}`,
	} {
		deps := Deps{
			ProbeStyles:    func(string, int, bool) (json.RawMessage, error) { return json.RawMessage(payload), nil },
			TrackingStatus: func() (bool, string) { return true, "x" },
		}
		if _, err := captureProbe(deps, ".card"); err == nil {
			t.Errorf("in-band extension error was accepted as a clean probe: %s", payload)
		}
	}
}

// TestHandle_InBandExtensionErrorIsNotACleanAudit is the same defect at the
// mode boundary: the caller must see a failure, not a clean page.
func TestHandle_InBandExtensionErrorIsNotACleanAudit(t *testing.T) {
	t.Parallel()
	deps := Deps{
		ProbeStyles: func(string, int, bool) (json.RawMessage, error) {
			return json.RawMessage(`{"error":"computed_styles_failed","message":"Receiving end does not exist"}`), nil
		},
		TrackingStatus: func() (bool, string) { return true, "x" },
	}
	body := string(Handle(deps, request(), json.RawMessage(`{"what":"design_audit","selector":".card"}`)).Result)
	if strings.Contains(body, reasonNoElements) {
		t.Errorf("an extension failure was reported as 'no elements matched': %s", body)
	}
	if !strings.Contains(body, "Receiving end does not exist") {
		t.Errorf("the underlying extension error did not reach the caller: %s", body)
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
