// handler_test.go — analyze verification contract handler tests.

package verificationhandler

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/verification"
)

func TestHandleDefinesValidVersionedContract(t *testing.T) {
	response := Handle(request(), json.RawMessage(`{
		"what":"verification",
		"operation":"define",
		"contract":{"schema_version":"1","contract_id":"checkout","assertions":[{"assertion_id":"total","description":"total is visible","required_evidence":["dom"]}]}
	}`))
	isError, text := responseText(t, response)
	if isError {
		t.Fatalf("unexpected error: %s", text)
	}
	for _, want := range []string{`"schema_version":"1"`, `"contract_id":"checkout"`, `"status":"defined"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("response missing %s: %s", want, text)
		}
	}
}

func TestHandleEvaluatesContractAndRejectsInvalidOperation(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("KABOOM_STATE_DIR", stateDir)
	args := json.RawMessage(fmt.Sprintf(`{
		"what":"verification",
		"operation":"evaluate",
		"contract":{"schema_version":"1","contract_id":"checkout","assertions":[{"assertion_id":"total","description":"total is visible","required_evidence":["dom"]}]},
		"results":[{"assertion_id":"total","verdict":"PASS"}],
		"evidence":[{"assertion_id":"total","kind":"dom","tool":"observe","action":"dom","correlation_id":"qa-1","captured_at":%q,"content":{"visible":true,"authorization":"Bearer private-token-12345"}}]
	}`, time.Now().UTC().Format(time.RFC3339Nano)))
	isError, text := responseText(t, Handle(request(), args))
	if isError || !strings.Contains(text, `"verdict":"PASS"`) {
		t.Fatalf("unexpected evaluation response: error=%v text=%s", isError, text)
	}
	if !strings.Contains(text, `"evidence_id":"sha256:`) || !strings.Contains(text, `"correlation_id":"qa-1"`) {
		t.Fatalf("evaluation response lacks durable evidence reference: %s", text)
	}
	if strings.Contains(text, "private-token") || !strings.Contains(text, "[REDACTED") {
		t.Fatalf("evaluation response leaked unredacted evidence: %s", text)
	}
	entries, err := os.ReadDir(stateDir + "/evidence")
	if err != nil || len(entries) != 1 {
		t.Fatalf("durable evidence entries = %d, err = %v", len(entries), err)
	}

	isError, _ = responseText(t, Handle(request(), json.RawMessage(`{"operation":"guess"}`)))
	if !isError {
		t.Fatal("unknown operation should fail")
	}
}

func TestHandleRejectsInvalidContract(t *testing.T) {
	isError, text := responseText(t, Handle(request(), json.RawMessage(`{"operation":"define","contract":{"schema_version":"1","contract_id":"empty"}}`)))
	if !isError || !strings.Contains(text, "assertion") {
		t.Fatalf("invalid contract response: error=%v text=%s", isError, text)
	}
}

func TestPersistAndResolveEvidenceReloadsReferencesAndAllowsExpectedAbsence(t *testing.T) {
	store := verification.Store{Dir: t.TempDir()}
	artifact, err := verification.BuildEvidence(verification.EvidenceInput{
		Kind: "dom", Tool: "observe", Action: "dom", CorrelationID: "qa-1",
		CapturedAt: time.Now().UTC(), Content: map[string]any{"visible": true},
	})
	if err != nil {
		t.Fatalf("BuildEvidence() error = %v", err)
	}
	if _, err := persistAndResolveEvidence(store, nil, []verification.EvidenceArtifact{artifact}); err != nil {
		t.Fatalf("persist evidence: %v", err)
	}
	resolved, err := persistAndResolveEvidence(store, []verification.AssertionResult{{AssertionID: "total", Evidence: []verification.EvidenceRef{artifact.Ref}}}, nil)
	if err != nil || len(resolved) != 1 || resolved[0].Ref.ID != artifact.Ref.ID {
		t.Fatalf("resolved = %#v, err = %v", resolved, err)
	}
	missing := artifact.Ref
	missing.ID = "sha256:" + strings.Repeat("0", 64)
	resolved, err = persistAndResolveEvidence(store, []verification.AssertionResult{{AssertionID: "total", Evidence: []verification.EvidenceRef{missing}}}, nil)
	if err != nil || len(resolved) != 0 {
		t.Fatalf("expected missing evidence to remain unresolved: %#v, err = %v", resolved, err)
	}
	missing.ID = "../../private"
	resolved, err = persistAndResolveEvidence(store, []verification.AssertionResult{{AssertionID: "total", Evidence: []verification.EvidenceRef{missing}}}, nil)
	if err != nil || len(resolved) != 0 {
		t.Fatalf("expected malformed evidence to remain unresolved: %#v, err = %v", resolved, err)
	}
}

func request() mcp.JSONRPCRequest { return mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1} }

func responseText(t *testing.T, response mcp.JSONRPCResponse) (bool, string) {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content blocks = %d, want 1", len(result.Content))
	}
	return result.IsError, result.Content[0].Text
}
