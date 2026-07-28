// tool_test.go — Tests security diff tool dispatch and summaries.
// Docs: docs/features/feature/security-hardening/index.md

package diff

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestSecurityDiffHandleDiffSecurity(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	bodies := []types.NetworkBody{
		{
			URL:         "https://myapp.com/",
			Method:      "GET",
			ContentType: "text/html",
			ResponseHeaders: map[string]string{
				"X-Frame-Options": "DENY",
			},
			HasAuthHeader: true,
		},
	}

	// Test snapshot action
	snapshotParams, _ := json.Marshal(map[string]string{
		"action": "snapshot",
		"name":   "test-snap",
	})
	result, err := mgr.HandleDiffSecurity(json.RawMessage(snapshotParams), bodies)
	if err != nil {
		t.Fatal(err)
	}
	snap, ok := result.(*Snapshot)
	if !ok {
		t.Fatalf("expected *Snapshot, got %T", result)
	}
	if snap.Name != "test-snap" {
		t.Errorf("expected name 'test-snap', got %q", snap.Name)
	}

	// Test list action
	listParams, _ := json.Marshal(map[string]string{
		"action": "list",
	})
	listResult, err := mgr.HandleDiffSecurity(json.RawMessage(listParams), bodies)
	if err != nil {
		t.Fatal(err)
	}
	entries, ok := listResult.([]SnapshotListEntry)
	if !ok {
		t.Fatalf("expected []SnapshotListEntry, got %T", listResult)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Take another snapshot for compare
	snap2Params, _ := json.Marshal(map[string]string{
		"action": "snapshot",
		"name":   "test-snap2",
	})
	_, err = mgr.HandleDiffSecurity(json.RawMessage(snap2Params), bodies)
	if err != nil {
		t.Fatal(err)
	}

	// Test compare action
	compareParams, _ := json.Marshal(map[string]string{
		"action":       "compare",
		"compare_from": "test-snap",
		"compare_to":   "test-snap2",
	})
	compareResult, err := mgr.HandleDiffSecurity(json.RawMessage(compareParams), bodies)
	if err != nil {
		t.Fatal(err)
	}
	diffResult, ok := compareResult.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", compareResult)
	}
	if diffResult.Verdict != "unchanged" {
		t.Errorf("expected 'unchanged' verdict, got %q", diffResult.Verdict)
	}
}

func TestSummary(t *testing.T) {
	t.Parallel()
	mgr := NewManager()

	// Before: multiple headers, auth, HTTPS
	beforeBodies := []types.NetworkBody{
		{
			URL:         "https://myapp.com/",
			Method:      "GET",
			ContentType: "text/html",
			ResponseHeaders: map[string]string{
				"X-Frame-Options":         "DENY",
				"X-Content-Type-Options":  "nosniff",
				"Content-Security-Policy": "default-src 'self'",
			},
			HasAuthHeader: true,
		},
	}

	// After: all headers removed, auth dropped
	afterBodies := []types.NetworkBody{
		{
			URL:             "https://myapp.com/",
			Method:          "GET",
			ContentType:     "text/html",
			ResponseHeaders: map[string]string{},
			HasAuthHeader:   false,
		},
	}

	result := mustCompareSnapshots(t, mgr, beforeBodies, afterBodies)

	if result.Summary.TotalRegressions == 0 {
		t.Error("expected non-zero total regressions")
	}
	if result.Summary.BySeverity == nil {
		t.Error("expected non-nil BySeverity map")
	}
	if result.Summary.ByCategory == nil {
		t.Error("expected non-nil ByCategory map")
	}
	if result.Summary.ByCategory["headers"] == 0 {
		t.Error("expected headers category in summary")
	}
}

func TestHandleDiffSecurityInvalidAction(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	params := []byte(`{"action":"invalid"}`)
	_, err := mgr.HandleDiffSecurity(params, nil)
	if err == nil {
		t.Error("expected error for invalid action")
	}
}

func TestHandleDiffSecurityInvalidJSON(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	params := []byte(`{invalid}`)
	_, err := mgr.HandleDiffSecurity(params, nil)
	if err == nil {
		t.Error("expected error for invalid JSON params")
	}
}
