// contract.go — Versioned QA verification contracts and deterministic verdict evaluation.

package verification

import (
	"fmt"
	"strings"
)

const SchemaVersion = "1"

type Verdict string

const (
	VerdictPass       Verdict = "PASS"
	VerdictFail       Verdict = "FAIL"
	VerdictBlocked    Verdict = "BLOCKED"
	VerdictUnverified Verdict = "UNVERIFIED"
	VerdictFlaky      Verdict = "FLAKY"
)

type Contract struct {
	SchemaVersion string      `json:"schema_version"`
	ID            string      `json:"contract_id"`
	Subject       string      `json:"subject,omitempty"`
	Assertions    []Assertion `json:"assertions"`
}

type Assertion struct {
	ID               string   `json:"assertion_id"`
	Description      string   `json:"description"`
	RequiredEvidence []string `json:"required_evidence,omitempty"`
}

type EvidenceRef struct {
	ID   string `json:"evidence_id"`
	Kind string `json:"kind"`
}

type AssertionResult struct {
	AssertionID string        `json:"assertion_id"`
	Verdict     Verdict       `json:"verdict"`
	Evidence    []EvidenceRef `json:"evidence,omitempty"`
	Reason      string        `json:"reason,omitempty"`
}

type Result struct {
	SchemaVersion string            `json:"schema_version"`
	ContractID    string            `json:"contract_id"`
	Verdict       Verdict           `json:"verdict"`
	Assertions    []AssertionResult `json:"assertions"`
}

func ValidateContract(contract Contract) error {
	if contract.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version must be %q", SchemaVersion)
	}
	if strings.TrimSpace(contract.ID) == "" {
		return fmt.Errorf("contract_id is required")
	}
	if len(contract.Assertions) == 0 {
		return fmt.Errorf("at least one assertion is required")
	}
	seen := make(map[string]struct{}, len(contract.Assertions))
	for _, assertion := range contract.Assertions {
		if strings.TrimSpace(assertion.ID) == "" || strings.TrimSpace(assertion.Description) == "" {
			return fmt.Errorf("every assertion requires assertion_id and description")
		}
		if _, exists := seen[assertion.ID]; exists {
			return fmt.Errorf("duplicate assertion_id %q", assertion.ID)
		}
		seen[assertion.ID] = struct{}{}
		for _, kind := range assertion.RequiredEvidence {
			if strings.TrimSpace(kind) == "" {
				return fmt.Errorf("assertion %q has an empty required_evidence kind", assertion.ID)
			}
		}
	}
	return nil
}

func Evaluate(contract Contract, supplied []AssertionResult) (Result, error) {
	if err := ValidateContract(contract); err != nil {
		return Result{}, err
	}
	known := make(map[string]Assertion, len(contract.Assertions))
	for _, assertion := range contract.Assertions {
		known[assertion.ID] = assertion
	}
	byID := make(map[string]AssertionResult, len(supplied))
	for _, result := range supplied {
		assertion, exists := known[result.AssertionID]
		if !exists {
			return Result{}, fmt.Errorf("unknown assertion_id %q", result.AssertionID)
		}
		if _, duplicate := byID[result.AssertionID]; duplicate {
			return Result{}, fmt.Errorf("duplicate result for assertion_id %q", result.AssertionID)
		}
		if !validVerdict(result.Verdict) {
			return Result{}, fmt.Errorf("invalid verdict %q", result.Verdict)
		}
		if result.Verdict == VerdictPass && !hasRequiredEvidence(assertion.RequiredEvidence, result.Evidence) {
			result.Verdict = VerdictUnverified
			result.Reason = "required evidence is incomplete"
		}
		byID[result.AssertionID] = result
	}

	result := Result{SchemaVersion: SchemaVersion, ContractID: contract.ID, Verdict: VerdictPass}
	for _, assertion := range contract.Assertions {
		assertionResult, exists := byID[assertion.ID]
		if !exists {
			assertionResult = AssertionResult{AssertionID: assertion.ID, Verdict: VerdictUnverified, Reason: "assertion was not evaluated"}
		}
		result.Assertions = append(result.Assertions, assertionResult)
		result.Verdict = combineVerdicts(result.Verdict, assertionResult.Verdict)
	}
	return result, nil
}

func validVerdict(verdict Verdict) bool {
	switch verdict {
	case VerdictPass, VerdictFail, VerdictBlocked, VerdictUnverified, VerdictFlaky:
		return true
	default:
		return false
	}
}

func hasRequiredEvidence(required []string, evidence []EvidenceRef) bool {
	found := make(map[string]bool, len(evidence))
	for _, ref := range evidence {
		if strings.TrimSpace(ref.ID) != "" && strings.TrimSpace(ref.Kind) != "" {
			found[ref.Kind] = true
		}
	}
	for _, kind := range required {
		if !found[kind] {
			return false
		}
	}
	return true
}

func combineVerdicts(current, next Verdict) Verdict {
	priority := map[Verdict]int{VerdictPass: 0, VerdictUnverified: 1, VerdictFlaky: 2, VerdictBlocked: 3, VerdictFail: 4}
	if priority[next] > priority[current] {
		return next
	}
	return current
}
