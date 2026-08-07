// normalization_test.go — Contract tests for list_interactive payload hardening,
// CSP failure annotation and the lifecycle_status vocabulary. These paths were
// previously reached only through cmd/browser-agent's integration tests; the
// behavior they pin is the agent-visible response shape, so it is tested here
// against the functions that produce it.

package asyncresult

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/tutorial"
)

func decode(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("result is not a JSON object (%v): %s", err, raw)
	}
	return m
}

// TestNormalizeCompletedCommandResult_OnlyTouchesListInteractive pins the
// correlation-ID gate: normalization is scoped to dom_list_* commands and every
// other command's payload must come through byte-identical.
func TestNormalizeCompletedCommandResult_OnlyTouchesListInteractive(t *testing.T) {
	t.Parallel()

	payload := json.RawMessage(`{"value":null}`)
	got, errCode := NormalizeCompletedCommandResult("dom_click_1", payload)
	if errCode != "" {
		t.Fatalf("error code = %q for a non-list command, want empty", errCode)
	}
	if string(got) != string(payload) {
		t.Fatalf("payload = %s, want it untouched (%s)", got, payload)
	}

	// The same payload under a dom_list_ correlation ID IS normalized.
	got, errCode = NormalizeCompletedCommandResult("dom_list_1", payload)
	if errCode != "list_interactive_missing_payload" {
		t.Fatalf("error code = %q, want list_interactive_missing_payload", errCode)
	}
	if string(got) == string(payload) {
		t.Fatal("dom_list_ payload was not normalized")
	}
}

func TestNormalizeListInteractive_PayloadShapes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		in        string
		wantErr   string
		wantAsIs  bool
		wantEmpty bool // expect an "elements": [] repair payload
	}{
		{"empty bytes", "", "list_interactive_missing_payload", false, true},
		{"json null", "null", "list_interactive_missing_payload", false, true},
		{"non-object", `["a"]`, "list_interactive_missing_payload", false, true},
		{"elements array", `{"elements":[{"id":"e1"}]}`, "", true, false},
		{"elements null", `{"elements":null}`, "", false, true},
		{"elements wrong type", `{"elements":"nope"}`, "list_interactive_missing_payload", false, true},
		{"value null, no elements", `{"value":null}`, "list_interactive_missing_payload", false, true},
		{"no elements key", `{"ok":true}`, "list_interactive_missing_payload", false, true},
		{"malformed json", `{not json`, "", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := json.RawMessage(tc.in)
			got, errCode := normalizeListInteractiveResult(in)
			if errCode != tc.wantErr {
				t.Fatalf("error code = %q, want %q", errCode, tc.wantErr)
			}
			if tc.wantAsIs {
				if string(got) != tc.in {
					t.Fatalf("payload = %s, want it passed through unchanged", got)
				}
				return
			}
			m := decode(t, got)
			if tc.wantEmpty {
				elems, ok := m["elements"].([]any)
				if !ok || len(elems) != 0 {
					t.Fatalf("elements = %v, want an empty array so agents can iterate safely", m["elements"])
				}
			}
		})
	}
}

// TestNormalizeListInteractive_RepairPreservesTabContext pins that the repair
// payload carries the tab/routing fields forward — the agent still needs to
// know which tab answered even when the element list was unusable.
func TestNormalizeListInteractive_RepairPreservesTabContext(t *testing.T) {
	t.Parallel()

	in := json.RawMessage(`{
		"value": null,
		"resolved_tab_id": 7,
		"resolved_url": "https://example.com/a",
		"target_context": "main",
		"effective_tab_id": 9,
		"effective_url": "https://example.com/b",
		"effective_title": "B",
		"dropped_field": "gone"
	}`)
	got, errCode := normalizeListInteractiveResult(in)
	if errCode != "list_interactive_missing_payload" {
		t.Fatalf("error code = %q, want list_interactive_missing_payload", errCode)
	}
	m := decode(t, got)
	for k, want := range map[string]any{
		"resolved_tab_id":  float64(7),
		"resolved_url":     "https://example.com/a",
		"target_context":   "main",
		"effective_tab_id": float64(9),
		"effective_url":    "https://example.com/b",
		"effective_title":  "B",
	} {
		if m[k] != want {
			t.Errorf("%s = %v, want %v", k, m[k], want)
		}
	}
	if _, present := m["dropped_field"]; present {
		t.Error("repair payload must not carry arbitrary fields through")
	}
	if m["success"] != false || m["error"] != "list_interactive_missing_payload" {
		t.Errorf("repair payload = %v, want success:false + the missing-payload error code", m)
	}
	if msg, _ := m["message"].(string); msg == "" {
		t.Error("repair payload must explain itself in message")
	}
}

func TestCanonicalLifecycleStatus(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"queued":           "queued",
		"pending":          "running",
		"running":          "running",
		"still_processing": "running",
		"complete":         "complete",
		"error":            "error",
		"timeout":          "timeout",
		"expired":          "timeout",
		"cancelled":        "cancelled",
		"canceled":         "cancelled",
		"  COMPLETE  ":     "complete",
		"weird_state":      "weird_state", // unknown values pass through verbatim
	}
	for in, want := range cases {
		if got := CanonicalLifecycleStatus(in); got != want {
			t.Errorf("CanonicalLifecycleStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAnnotateCSPFailure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		cmdError string
		result   string
		wantCSP  bool
	}{
		{"explicit csp_blocked flag", "", `{"csp_blocked":true}`, true},
		{"failure_cause csp", "", `{"failure_cause":"CSP"}`, true},
		{"error text mentions CSP", "", `{"error":"csp_violation"}`, true},
		{"message mentions trusted types", "", `{"message":"Trusted Type policy blocked it"}`, true},
		{"hint mentions unsafe-eval", "", `{"hint":"page forbids unsafe-eval"}`, true},
		{"restricted page", "", `{"error":"restricted_page"}`, true},
		{"command error mentions CSP", "content security policy blocked injection", ``, true},
		{"unrelated failure", "element not found", `{"error":"element_not_found"}`, false},
		{"empty everything", "", ``, false},
		{"malformed result json", "", `{oops`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			responseData := map[string]any{}
			AnnotateCSPFailure(responseData, tc.cmdError, json.RawMessage(tc.result))
			_, annotated := responseData["csp_blocked"]
			if annotated != tc.wantCSP {
				t.Fatalf("csp_blocked present = %v, want %v (responseData=%v)", annotated, tc.wantCSP, responseData)
			}
			if !tc.wantCSP {
				return
			}
			if responseData["failure_cause"] != "csp" {
				t.Errorf("failure_cause = %v, want csp", responseData["failure_cause"])
			}
			if responseData["retry"] != tutorial.CSPRetryNavigationGuidance {
				t.Errorf("retry = %v, want the CSP navigation guidance", responseData["retry"])
			}
		})
	}
}

// TestAnnotateCSPFailure_DoesNotOverwriteExisting pins that extension-supplied
// message/retry text wins over the generic CSP guidance.
func TestAnnotateCSPFailure_DoesNotOverwriteExisting(t *testing.T) {
	t.Parallel()

	responseData := map[string]any{"message": "extension said this", "retry": "extension retry"}
	AnnotateCSPFailure(responseData, "", json.RawMessage(`{"csp_blocked":true,"error":"csp_eval","message":"generic"}`))

	if responseData["message"] != "extension said this" {
		t.Errorf("message = %v, want the pre-existing message preserved", responseData["message"])
	}
	if responseData["retry"] != "extension retry" {
		t.Errorf("retry = %v, want the pre-existing retry preserved", responseData["retry"])
	}
	if responseData["error_code"] != "csp_eval" {
		t.Errorf("error_code = %v, want csp_eval surfaced from the result", responseData["error_code"])
	}
}

// TestAnnotateInteractFailureRecovery_SuggestsFirstVisibleCandidate pins the
// single-retry recovery hint for ambiguous_target: the suggestion must be the
// first VISIBLE candidate, not simply the first one.
func TestAnnotateInteractFailureRecovery_SuggestsFirstVisibleCandidate(t *testing.T) {
	t.Parallel()

	responseData := map[string]any{
		"error_code": "ambiguous_target",
		"candidates": []any{
			map[string]any{"element_id": "hidden-1", "visible": false},
			map[string]any{"element_id": "", "visible": true},
			map[string]any{"element_id": "visible-2", "visible": true},
			map[string]any{"element_id": "visible-3", "visible": true},
		},
	}
	AnnotateInteractFailureRecovery(responseData, "", nil)

	if got := responseData["suggested_element_id"]; got != "visible-2" {
		t.Fatalf("suggested_element_id = %v, want visible-2 (first visible with an id)", got)
	}
	retry, _ := responseData["retry"].(string)
	for _, phrase := range []string{"candidates", "element_id", "suggested_element_id"} {
		if !strings.Contains(strings.ToLower(retry), phrase) {
			t.Fatalf("ambiguous retry guidance missing %q: %s", phrase, retry)
		}
	}

	// An extension-supplied suggestion is authoritative and must not be replaced.
	responseData = map[string]any{
		"error_code":           "ambiguous_target",
		"suggested_element_id": "ranked-choice",
		"candidates":           []any{map[string]any{"element_id": "visible-1", "visible": true}},
	}
	AnnotateInteractFailureRecovery(responseData, "", nil)
	if got := responseData["suggested_element_id"]; got != "ranked-choice" {
		t.Fatalf("suggested_element_id = %v, want the extension's ranked-choice preserved", got)
	}

	// No visible candidate: no suggestion at all (better than a wrong one).
	responseData = map[string]any{
		"error_code": "ambiguous_target",
		"candidates": []any{map[string]any{"element_id": "hidden", "visible": false}, "not-a-map"},
	}
	AnnotateInteractFailureRecovery(responseData, "", nil)
	if _, present := responseData["suggested_element_id"]; present {
		t.Fatalf("suggested_element_id = %v, want none when no candidate is visible", responseData["suggested_element_id"])
	}

	// A non-ambiguous playbook must not trigger the candidate scan.
	responseData = map[string]any{
		"error_code": "element_not_found",
		"candidates": []any{map[string]any{"element_id": "visible-1", "visible": true}},
	}
	AnnotateInteractFailureRecovery(responseData, "", nil)
	if _, present := responseData["suggested_element_id"]; present {
		t.Fatal("suggested_element_id set for a non-ambiguous_target failure")
	}
}

// TestDetectInteractFailureCode_CandidatePrecedence pins which signal wins when
// several are present: the response's own error_code, then its error, then the
// extension result's error / error_code, then the raw command error. Getting the
// order wrong picks the wrong recovery playbook.
func TestDetectInteractFailureCode_CandidatePrecedence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		responseData map[string]any
		cmdError     string
		result       string
		want         string
	}{
		{
			name:         "response error_code wins over everything",
			responseData: map[string]any{"error_code": "stale_element_id", "error": "ambiguous_target"},
			cmdError:     "element_not_found",
			result:       `{"error":"scope_not_found"}`,
			want:         "stale_element_id",
		},
		{
			name:         "response error used when error_code is blank",
			responseData: map[string]any{"error_code": "   ", "error": "ambiguous_target"},
			cmdError:     "element_not_found",
			result:       `{"error":"scope_not_found"}`,
			want:         "ambiguous_target",
		},
		{
			name:         "extension result error_code used when response has nothing",
			responseData: map[string]any{},
			cmdError:     "element_not_found",
			result:       `{"error_code":"scope_not_found"}`,
			want:         "scope_not_found",
		},
		{
			name:         "command error is the last resort",
			responseData: map[string]any{},
			cmdError:     "element_not_found",
			result:       `{}`,
			want:         "element_not_found",
		},
		{
			name:         "unrecognized signals yield no code",
			responseData: map[string]any{"error_code": "nonsense"},
			cmdError:     "also nonsense",
			result:       `{oops`,
			want:         "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectInteractFailureCode(tc.responseData, tc.cmdError, json.RawMessage(tc.result)); got != tc.want {
				t.Fatalf("detectInteractFailureCode() = %q, want %q", got, tc.want)
			}
		})
	}
}
