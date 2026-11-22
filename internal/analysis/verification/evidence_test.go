// evidence_test.go — Content-addressed evidence creation and validation tests.

package verification

import (
	"strings"
	"testing"
	"time"
)

func TestBuildEvidenceRedactsBeforeStableHashing(t *testing.T) {
	capturedAt := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	input := EvidenceInput{
		Kind: "dom", Tool: "observe", Action: "dom", CorrelationID: "qa-123", CapturedAt: capturedAt,
		Content: map[string]any{"selector": "#total", "authorization": "Bearer secret-token-12345"},
	}
	first, err := BuildEvidence(input)
	if err != nil {
		t.Fatalf("BuildEvidence() error = %v", err)
	}
	second, err := BuildEvidence(input)
	if err != nil {
		t.Fatalf("BuildEvidence() second error = %v", err)
	}
	if first.Ref.ID != second.Ref.ID || !strings.HasPrefix(first.Ref.ID, "sha256:") {
		t.Fatalf("unstable content address: %q / %q", first.Ref.ID, second.Ref.ID)
	}
	if first.Ref.CorrelationID != "qa-123" || first.Ref.Kind != "dom" {
		t.Fatalf("incomplete reference: %#v", first.Ref)
	}
	encoded := string(first.Content)
	if strings.Contains(encoded, "secret-token") || !strings.Contains(encoded, "[REDACTED") {
		t.Fatalf("content was not redacted before storage: %s", encoded)
	}
}

func TestBuildEvidenceRejectsInvalidProvenanceAndOversizedContent(t *testing.T) {
	valid := EvidenceInput{Kind: "dom", Tool: "observe", Action: "dom", CorrelationID: "qa-123", CapturedAt: time.Now(), Content: map[string]any{"ok": true}}
	tests := []EvidenceInput{
		{Kind: valid.Kind, Tool: "sixth_tool", Action: valid.Action, CorrelationID: valid.CorrelationID, CapturedAt: valid.CapturedAt, Content: valid.Content},
		{Kind: valid.Kind, Tool: valid.Tool, Action: valid.Action, CapturedAt: valid.CapturedAt, Content: valid.Content},
		{Kind: valid.Kind, Tool: valid.Tool, Action: valid.Action, CorrelationID: valid.CorrelationID, CapturedAt: valid.CapturedAt, Content: map[string]any{"body": strings.Repeat("x", MaxEvidenceBytes+1)}},
	}
	for _, input := range tests {
		if _, err := BuildEvidence(input); err == nil {
			t.Fatalf("expected invalid evidence input: %#v", input)
		}
	}
}

func TestEvaluateRequiresPresentFreshUntamperedEvidence(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	contract := Contract{SchemaVersion: SchemaVersion, ID: "checkout", Assertions: []Assertion{{ID: "total", Description: "total visible", RequiredEvidence: []string{"dom"}}}}
	fresh := mustEvidence(t, EvidenceInput{Kind: "dom", Tool: "observe", Action: "dom", CorrelationID: "qa-1", CapturedAt: now.Add(-time.Minute), Content: map[string]any{"visible": true}})

	tests := []struct {
		name    string
		ref     EvidenceRef
		catalog []EvidenceArtifact
	}{
		{name: "missing", ref: fresh.Ref},
		{name: "stale", ref: mustEvidence(t, EvidenceInput{Kind: "dom", Tool: "observe", Action: "dom", CorrelationID: "qa-1", CapturedAt: now.Add(-25 * time.Hour), Content: map[string]any{"visible": true}}).Ref, catalog: []EvidenceArtifact{mustEvidence(t, EvidenceInput{Kind: "dom", Tool: "observe", Action: "dom", CorrelationID: "qa-1", CapturedAt: now.Add(-25 * time.Hour), Content: map[string]any{"visible": true}})}},
		{name: "tampered reference", ref: EvidenceRef{ID: fresh.Ref.ID, Kind: "screenshot", CorrelationID: fresh.Ref.CorrelationID, CapturedAt: fresh.Ref.CapturedAt}, catalog: []EvidenceArtifact{fresh}},
		{name: "tampered artifact", ref: fresh.Ref, catalog: []EvidenceArtifact{{SchemaVersion: fresh.SchemaVersion, Ref: fresh.Ref, Tool: fresh.Tool, Action: fresh.Action, Content: []byte(`{"visible":false}`)}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(contract, []AssertionResult{{AssertionID: "total", Verdict: VerdictPass, Evidence: []EvidenceRef{tt.ref}}}, tt.catalog, now, 24*time.Hour)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if result.Verdict != VerdictUnverified {
				t.Fatalf("verdict = %q, want UNVERIFIED", result.Verdict)
			}
		})
	}
}

func mustEvidence(t *testing.T, input EvidenceInput) EvidenceArtifact {
	t.Helper()
	artifact, err := BuildEvidence(input)
	if err != nil {
		t.Fatalf("BuildEvidence() error = %v", err)
	}
	return artifact
}
