// contract_test.go — The finding contract, spec precedence, the bounded
// response envelope, and the mode surface.
//
// The shipped fixture's agreement with the expected-findings table lives in
// analyzers_test.go; the two together exceed the 800-line file limit.

package designdrift

import (
	"encoding/json"
	"errors"
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
	f := newFinding(findingSpec{category: categorySpacing, property: "gap-vertical", el: makeElement(0, "div", nil),
		observed: "14px", expected: "24px", provenance: provenanceInferred, confidence: confidenceHigh, evidence: "evidence", message: "message"})
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

	result := buildAuditResult(auditInputs{selector: ".card", elements: elements, matchCount: len(elements),
		byCategory: byCategory, window: normalizeWindow(0, 0)})
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
	result := buildAuditResult(auditInputs{selector: ".card",
		byCategory: map[string][]finding{categorySpacing: nil},
		skips:      []skipped{{Category: categoryDesignTokens, Reason: reasonNoTokens}},
		window:     normalizeWindow(0, 0)})

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
	result := buildAuditResult(auditInputs{selector: ".card", elements: make([]elementView, 6), matchCount: 6,
		byCategory: byCategory, window: normalizeWindow(0, 0)})

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

// --- The bounded response (kaboom-4h7j) ---

// driftingCardsProbe builds a probe payload of identically-broken cards: one
// wrong padding, one wrong margin and three wrong colours each, against a page
// that declares the tokens they near-miss. It is the shape that made the
// response unbounded — forty such cards is one CSS edit and used to be 200
// findings.
func driftingCardsProbe(count int) json.RawMessage {
	elements := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		elements = append(elements, map[string]any{
			"selector": "div.card", "tag": "div", "index": i, "in_flow": true,
			"computed_styles": map[string]string{
				"padding-top": "15px", "padding-right": "15px", "padding-bottom": "15px", "padding-left": "15px",
				"margin-top": "15px", "margin-right": "15px", "margin-bottom": "15px", "margin-left": "15px",
				"color": "#2b56e2", "background-color": "#2b56e2", "border-color": "#2b56e2",
			},
			// A uniform 40px gap: the spacing analyzer finds a clean rhythm, so
			// every finding below belongs to design_tokens and the section cap
			// is measured on one section rather than spread across three.
			"box_model": map[string]any{
				"top": float64(i * 100), "bottom": float64(i*100 + 60),
				"left": 0, "right": 400, "width": 400, "height": 60,
			},
		})
	}
	payload, err := json.Marshal(map[string]any{
		"elements": elements, "count": count, "match_count": count, "truncated": false,
		"root_tokens": map[string]string{"--spacing-md": "16px", "--color-primary-main": "#2a55e1"},
	})
	if err != nil {
		panic(err)
	}
	return payload
}

// TestEnvelopeBoundsEverySectionAndPagesToEveryFinding covers kaboom-4h7j.
//
// At the mode's own element cap the envelope measured 588KB, of which
// mcp.ClampResponseSize kept 46KB — it cuts the JSON mid-string, so the rest
// was neither readable nor recoverable, and the clamp note told the caller to
// page with parameters design_audit did not accept. Both halves are asserted
// here: the response fits, AND every finding the audit made is reachable.
func TestEnvelopeBoundsEverySectionAndPagesToEveryFinding(t *testing.T) {
	t.Parallel()
	const cards = 40
	payload := driftingCardsProbe(cards)
	deps := Deps{
		ProbeStyles:    func(string, int, bool) (json.RawMessage, error) { return payload, nil },
		TrackingStatus: func() (bool, string) { return true, "https://app.example.test" },
	}

	resp := Handle(deps, request(), json.RawMessage(`{"what":"design_audit","selector":".card"}`))
	if _, report := mcp.ClampResponseSize(resp.Result); report.Truncated {
		t.Fatalf("the default response was clamped at %d bytes; the findings past the cut are unrecoverable", len(resp.Result))
	}

	probe, err := captureProbe(deps, ".card")
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	categories, _ := resolveCategories(nil)
	first := runAudit(auditParams{Selector: ".card"}, probe, categories)

	// Control: the subject must produce more findings than one page holds, or
	// every bound below would pass vacuously on a small audit.
	if first.TotalFindings <= maxFindingsPerSection {
		t.Fatalf("total_findings = %d; this case needs more findings than one page holds", first.TotalFindings)
	}
	if first.ReturnedFindings >= first.TotalFindings {
		t.Fatalf("returned_findings = %d of %d; nothing was bounded", first.ReturnedFindings, first.TotalFindings)
	}
	if first.NextOffset == 0 {
		t.Fatal("next_offset is absent on a bounded response, so the caller cannot reach the rest")
	}
	for category, section := range first.Sections {
		bucket := section.(map[string]any)
		if returned := bucket["returned"].(int); returned > maxFindingsPerSection {
			t.Errorf("section %s returned %d findings, above the %d cap", category, returned, maxFindingsPerSection)
		}
	}

	// Walk the pages the response points at and prove they cover the census.
	seen := make(map[findingKey]bool, first.TotalFindings)
	for offset, pages := 0, 0; ; pages++ {
		if pages > cards {
			t.Fatal("paging did not terminate")
		}
		page := runAudit(auditParams{Selector: ".card", Offset: offset}, probe, categories)
		for key := range collectFindings(page) {
			seen[key] = true
		}
		if page.NextOffset == 0 {
			break
		}
		offset = page.NextOffset
	}
	if len(seen) != first.TotalFindings {
		t.Errorf("paging reached %d distinct findings, but the audit made %d — the rest are unreachable",
			len(seen), first.TotalFindings)
	}
}

// TestNormalizeWindow_CannotBeWidenedPastTheCap: a caller asking for everything
// in one call is the request that puts the silent truncation back within reach.
func TestNormalizeWindow_CannotBeWidenedPastTheCap(t *testing.T) {
	t.Parallel()
	for _, limit := range []int{0, -5, maxFindingsPerSection + 1, 100000} {
		if got := normalizeWindow(limit, 0).limit; got > maxFindingsPerSection {
			t.Errorf("limit %d normalized to %d, above the %d cap", limit, got, maxFindingsPerSection)
		}
	}
	// Control: a limit inside the cap is honoured rather than silently reset.
	if got := normalizeWindow(5, 0).limit; got != 5 {
		t.Errorf("limit 5 normalized to %d; a smaller page must be respected", got)
	}
	if got := normalizeWindow(5, -1).offset; got != 0 {
		t.Errorf("a negative offset normalized to %d, want 0", got)
	}
}

// --- Shorthand collapse (kaboom-4h7j) ---

func longhandFinding(property, observed string) finding {
	return newFinding(findingSpec{category: categoryDesignTokens, property: property,
		el: makeElement(0, "div.card", nil), observed: observed, expected: "--spacing-md (16px)",
		provenance: provenanceInferred, confidence: confidenceHigh, evidence: "page token --spacing-md = 16px",
		message: property + " is " + observed + ", a near-miss of the page token --spacing-md (16px)"})
}

// TestCollapseShorthandDuplicates_OnlyTheUniformGroupCollapses covers the third
// missing bound of kaboom-4h7j: `padding: 15px` produced four byte-identical
// findings differing only in longhand name, for one edit.
func TestCollapseShorthandDuplicates_OnlyTheUniformGroupCollapses(t *testing.T) {
	t.Parallel()
	uniform := collapseShorthandDuplicates([]finding{
		longhandFinding("padding-top", "15px"), longhandFinding("padding-right", "15px"),
		longhandFinding("padding-bottom", "15px"), longhandFinding("padding-left", "15px"),
	})
	if len(uniform) != 1 {
		t.Fatalf("one `padding: 15px` produced %d findings, want 1: %+v", len(uniform), uniform)
	}
	if uniform[0].Property != "padding" {
		t.Errorf("collapsed property = %q, want padding", uniform[0].Property)
	}
	if !strings.HasPrefix(uniform[0].Message, "padding is 15px") {
		t.Errorf("collapsed message still names a longhand: %q", uniform[0].Message)
	}

	// Control: a partial group must NOT collapse. `padding: 15px 16px` drifts on
	// two sides only, and "padding is 15px" would be false about the other two.
	partial := collapseShorthandDuplicates([]finding{
		longhandFinding("padding-top", "15px"), longhandFinding("padding-bottom", "15px"),
		longhandFinding("padding-left", "17px"),
	})
	if len(partial) != 3 {
		t.Fatalf("a partial group collapsed: %d finding(s), want 3: %+v", len(partial), partial)
	}
	for _, f := range partial {
		if f.Property == "padding" {
			t.Errorf("a partial group was reported as the whole shorthand: %+v", f)
		}
	}
}

// --- Cross-category reconciliation (kaboom-w1et) ---

func measuredGapFinding(index int, observed, expected string) finding {
	return newFinding(findingSpec{category: categorySpacing, property: "gap-vertical",
		el: makeElement(index, "div.rhythm-card", nil), observed: observed, expected: expected,
		provenance: provenanceInferred, confidence: confidenceLow,
		evidence: "3 of 4 vertical gaps measure " + expected, message: "gap"})
}

func proximityGuessFinding(index int, property, observed string) finding {
	return newFinding(findingSpec{category: categoryDesignTokens, property: property,
		el: makeElement(index, "div.rhythm-card", nil), observed: observed, expected: "--spacing-md (16px)",
		provenance: provenanceInferred, confidence: confidenceHigh,
		evidence: "page token --spacing-md = 16px", message: property + " near-miss"})
}

// TestReconcileAcrossCategories_ProximityLosesToTheMeasuredGap covers kaboom-w1et.
//
// On the shipped fixture the DEFAULT call reported the same 14px twice with
// contradictory targets, and the wrong one (16px, from proximity) carried
// confidence:high while the right one (24px, from the measured rhythm) carried
// confidence:low — so an agent triaging by confidence applied the target that
// makes the page worse.
func TestReconcileAcrossCategories_ProximityLosesToTheMeasuredGap(t *testing.T) {
	t.Parallel()
	result := buildAuditResult(auditInputs{selector: ".rhythm-card", elements: make([]elementView, 5), matchCount: 5,
		byCategory: map[string][]finding{
			categorySpacing:      {measuredGapFinding(3, "14px", "24px")},
			categoryDesignTokens: {proximityGuessFinding(3, "margin-top", "14px")},
		}, window: normalizeWindow(0, 0)})

	tokens := result.Sections[categoryDesignTokens].(map[string]any)
	if got := tokens["total"].(int); got != 0 {
		t.Errorf("the contradicted proximity guess survived: %d design_tokens finding(s)", got)
	}
	survivors := result.Sections[categorySpacing].(map[string]any)["findings"].([]finding)
	if len(survivors) != 1 || survivors[0].Expected != "24px" {
		t.Fatalf("the measured verdict did not survive intact: %+v", survivors)
	}
	// Nothing is discarded silently: the rejected expectation stays visible.
	if !strings.Contains(survivors[0].Evidence, "--spacing-md (16px)") {
		t.Errorf("the superseded expectation vanished from the evidence: %q", survivors[0].Evidence)
	}

	// Control 1: a token finding about a DIFFERENT element is not about this
	// gap and must survive. Control 2: so must one about a property that
	// produces no gap. Without both, "reconciliation" would just be a filter
	// that deletes the design_tokens section whenever spacing reports anything.
	kept := buildAuditResult(auditInputs{selector: ".rhythm-card", elements: make([]elementView, 5), matchCount: 5,
		byCategory: map[string][]finding{
			categorySpacing: {measuredGapFinding(3, "14px", "24px")},
			categoryDesignTokens: {
				proximityGuessFinding(2, "margin-top", "14px"),
				proximityGuessFinding(3, "padding-top", "14px"),
			},
		}, window: normalizeWindow(0, 0)})
	if got := kept.Sections[categoryDesignTokens].(map[string]any)["total"].(int); got != 2 {
		t.Errorf("reconciliation deleted findings it does not answer: %d design_tokens finding(s), want 2", got)
	}
}
