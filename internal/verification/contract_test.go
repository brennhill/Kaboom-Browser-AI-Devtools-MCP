// contract_test.go — Verification contract validation and verdict tests.

package verification

import "testing"

func TestValidateContractRejectsIncompleteAndUnknownSchemas(t *testing.T) {
	tests := []struct {
		name     string
		contract Contract
	}{
		{name: "missing schema", contract: Contract{ID: "checkout", Assertions: []Assertion{{ID: "total", Description: "total is visible"}}}},
		{name: "unknown schema", contract: Contract{SchemaVersion: "2", ID: "checkout", Assertions: []Assertion{{ID: "total", Description: "total is visible"}}}},
		{name: "missing id", contract: Contract{SchemaVersion: SchemaVersion, Assertions: []Assertion{{ID: "total", Description: "total is visible"}}}},
		{name: "no assertions", contract: Contract{SchemaVersion: SchemaVersion, ID: "checkout"}},
		{name: "incomplete assertion", contract: Contract{SchemaVersion: SchemaVersion, ID: "checkout", Assertions: []Assertion{{ID: "total"}}}},
		{name: "duplicate assertion", contract: Contract{SchemaVersion: SchemaVersion, ID: "checkout", Assertions: []Assertion{{ID: "total", Description: "one"}, {ID: "total", Description: "two"}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateContract(tt.contract); err == nil {
				t.Fatal("expected invalid contract")
			}
		})
	}
}

func TestEvaluateUsesExplicitVerdicts(t *testing.T) {
	contract := Contract{
		SchemaVersion: SchemaVersion,
		ID:            "checkout",
		Assertions: []Assertion{
			{ID: "total", Description: "total is visible", RequiredEvidence: []string{"dom"}},
		},
	}

	tests := []struct {
		name    string
		results []AssertionResult
		want    Verdict
	}{
		{name: "pass", results: []AssertionResult{{AssertionID: "total", Verdict: VerdictPass, Evidence: []EvidenceRef{{ID: "dom-1", Kind: "dom"}}}}, want: VerdictPass},
		{name: "fail", results: []AssertionResult{{AssertionID: "total", Verdict: VerdictFail, Evidence: []EvidenceRef{{ID: "dom-1", Kind: "dom"}}}}, want: VerdictFail},
		{name: "blocked", results: []AssertionResult{{AssertionID: "total", Verdict: VerdictBlocked}}, want: VerdictBlocked},
		{name: "unverified", results: []AssertionResult{{AssertionID: "total", Verdict: VerdictUnverified}}, want: VerdictUnverified},
		{name: "flaky", results: []AssertionResult{{AssertionID: "total", Verdict: VerdictFlaky, Evidence: []EvidenceRef{{ID: "dom-1", Kind: "dom"}}}}, want: VerdictFlaky},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(contract, tt.results)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if result.Verdict != tt.want {
				t.Fatalf("verdict = %q, want %q", result.Verdict, tt.want)
			}
		})
	}
}

func TestEvaluateNeverPassesMissingEvidenceOrAssertionResults(t *testing.T) {
	contract := Contract{
		SchemaVersion: SchemaVersion,
		ID:            "checkout",
		Assertions: []Assertion{
			{ID: "total", Description: "total is visible", RequiredEvidence: []string{"dom", "screenshot"}},
			{ID: "submit", Description: "submit succeeds"},
		},
	}

	result, err := Evaluate(contract, []AssertionResult{{
		AssertionID: "total",
		Verdict:     VerdictPass,
		Evidence:    []EvidenceRef{{ID: "dom-1", Kind: "dom"}},
	}})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if result.Verdict != VerdictUnverified {
		t.Fatalf("verdict = %q, want %q", result.Verdict, VerdictUnverified)
	}
	if result.Assertions[0].Verdict != VerdictUnverified {
		t.Fatalf("missing evidence assertion verdict = %q", result.Assertions[0].Verdict)
	}
	if result.Assertions[1].Verdict != VerdictUnverified {
		t.Fatalf("missing assertion result verdict = %q", result.Assertions[1].Verdict)
	}
}

func TestEvaluateRejectsUnknownAssertionsAndVerdicts(t *testing.T) {
	contract := Contract{SchemaVersion: SchemaVersion, ID: "checkout", Assertions: []Assertion{{ID: "total", Description: "total is visible"}}}
	for _, result := range []AssertionResult{
		{AssertionID: "other", Verdict: VerdictPass},
		{AssertionID: "total", Verdict: Verdict("MAYBE")},
	} {
		if _, err := Evaluate(contract, []AssertionResult{result}); err == nil {
			t.Fatalf("expected error for result %#v", result)
		}
	}
}
