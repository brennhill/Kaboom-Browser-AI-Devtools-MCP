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
	if result.BySeverity[severityError] == 0 {
		t.Error("the declared spacing scale was violated but no error was reported")
	}
	if result.BySeverity[severityWarning] == 0 {
		t.Error("the undeclared font family drifted but no warning was reported")
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
	if !strings.Contains(string(resp.Result), "selector") {
		t.Errorf("the error should name the missing parameter: %s", resp.Result)
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

	knownReasons := map[string]bool{reasonNoTokens: true, reasonInsufficientPeers: true, reasonNoElements: true, reasonNoInFlowElements: true}
	knownSeverity := map[string]bool{severityError: true, severityWarning: true}
	knownProvenance := map[string]bool{provenanceDeclared: true, provenanceInferred: true}
	knownConfidence := map[string]bool{confidenceHigh: true, confidenceLow: true, "": true}

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
		if expected.Confidence != "" && actual.Confidence != expected.Confidence {
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
