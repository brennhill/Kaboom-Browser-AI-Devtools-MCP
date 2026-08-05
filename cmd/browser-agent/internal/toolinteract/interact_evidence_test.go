// interact_evidence_test.go — Tests evidence configuration and screenshot lifecycle.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func TestEvidenceArgumentAndEnvironmentParsing(t *testing.T) {
	for _, tc := range []struct {
		args string
		want evidenceMode
		ok   bool
	}{
		{args: `{}`, want: evidenceModeOff, ok: true},
		{args: `{"evidence":" ALWAYS "}`, want: evidenceModeAlways, ok: true},
		{args: `{"evidence":"on_mutation"}`, want: evidenceModeOnMutation, ok: true},
		{args: `{"evidence":"sometimes"}`, want: evidenceModeOff, ok: false},
	} {
		got, err := ParseEvidenceMode(json.RawMessage(tc.args))
		if (err == nil) != tc.ok || got != tc.want {
			t.Fatalf("ParseEvidenceMode(%s) = %q, %v", tc.args, got, err)
		}
	}
	t.Setenv(evidenceRetryEnv, "bad")
	if got := evidenceRetryCount(); got != 1 {
		t.Fatalf("bad retry = %d", got)
	}
	t.Setenv(evidenceRetryEnv, "-3")
	if got := evidenceRetryCount(); got != 0 {
		t.Fatalf("low retry = %d", got)
	}
	t.Setenv(evidenceRetryEnv, "99")
	if got := evidenceRetryCount(); got != 3 {
		t.Fatalf("high retry = %d", got)
	}
	t.Setenv(evidenceMaxCapturesEnv, "1")
	if got := evidenceMaxCapturesPerCommand(); got != 1 {
		t.Fatalf("capture max = %d", got)
	}
	if got := canonicalActionFromInteractArgs(json.RawMessage(`{"action":" CLICK "}`)); got != "click" {
		t.Fatalf("canonical action = %q", got)
	}
	if !isMutationAction(" Navigate ") || isMutationAction("get_text") {
		t.Fatal("mutation classification mismatch")
	}
}

func TestCaptureEvidencePreconditions(t *testing.T) {
	if got := CaptureEvidence(nil, "client"); got.Error != "capture_not_initialized" {
		t.Fatalf("nil capture = %+v", got)
	}
	store := capture.NewCapture()
	if got := CaptureEvidence(store, "client"); got.Error != "no_tracked_tab" {
		t.Fatalf("untracked capture = %+v", got)
	}
}

func TestCaptureEvidenceResultContracts(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    EvidenceShot
	}{
		{name: "success", payload: `{"path":"/tmp/shot.png","filename":"shot.png"}`, want: EvidenceShot{Path: "/tmp/shot.png", Filename: "shot.png"}},
		{name: "extension error", payload: `{"error":"capture denied"}`, want: EvidenceShot{Error: "capture denied"}},
		{name: "missing path", payload: `{"filename":"shot.png"}`, want: EvidenceShot{Filename: "shot.png", Error: "screenshot_missing_path"}},
		{name: "invalid JSON", payload: `{bad`, want: EvidenceShot{Error: "screenshot_parse_error:"}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store := capture.NewCapture()
			t.Cleanup(store.Close)
			store.Extension().UpdateTrackedTab(7, "https://example.test", "Example")
			done := make(chan EvidenceShot, 1)
			go func() { done <- CaptureEvidence(store, "client") }()
			store.Queries().WaitForPendingQueries(time.Second)
			queryID := ""
			for _, query := range store.Queries().GetPendingQueries() {
				if query.Type == "screenshot" {
					queryID = query.ID
					break
				}
			}
			if queryID == "" {
				t.Fatal("screenshot query was not queued")
			}
			store.Queries().SetQueryResult(queryID, json.RawMessage(tc.payload))
			select {
			case got := <-done:
				if tc.name == "invalid JSON" {
					if len(got.Error) < len(tc.want.Error) || got.Error[:len(tc.want.Error)] != tc.want.Error {
						t.Fatalf("shot = %+v", got)
					}
				} else if got != tc.want {
					t.Fatalf("shot = %+v, want %+v", got, tc.want)
				}
			case <-time.After(time.Second):
				t.Fatal("CaptureEvidence did not return")
			}
		})
	}
}
