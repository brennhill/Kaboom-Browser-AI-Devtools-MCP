// execute_test.go — Tests IndexedDB query dispatch and result normalization.

package idbquery

import (
	"encoding/json"
	"runtime"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func respondToNextQuery(t *testing.T, cap *capture.Capture, result json.RawMessage) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			pending := cap.Queries().GetPendingQueries()
			if len(pending) > 0 {
				cap.Queries().SetQueryResult(pending[0].ID, result)
				return
			}
			runtime.Gosched()
		}
	}()
	t.Cleanup(func() { <-done })
}

func TestListingSortsAndSuppliesDefaults(t *testing.T) {
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	respondToNextQuery(t, cap, json.RawMessage(`{
		"success":true,
		"result":{"databases":[{"name":"zeta"},{"name":"alpha"}]}
	}`))

	result, err := Listing(cap)
	if err != nil {
		t.Fatal(err)
	}
	if result["supported"] != true {
		t.Fatalf("supported = %v", result["supported"])
	}
	databases := result["databases"].([]any)
	if databases[0].(map[string]any)["name"] != "alpha" {
		t.Fatalf("databases not sorted: %#v", databases)
	}
}

func TestEntriesNormalizesSuccessAndPageErrors(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cap := capture.NewCapture()
		t.Cleanup(cap.Close)
		respondToNextQuery(t, cap, json.RawMessage(`{
			"success":true,
			"result":{"entries":[{"key":1}]}
		}`))
		result, err := Entries(cap, "app", "users", 5)
		if err != nil {
			t.Fatal(err)
		}
		if result["count"] != 1 || result["database"] != "app" || result["store"] != "users" {
			t.Fatalf("normalized result = %#v", result)
		}
	})

	t.Run("page error", func(t *testing.T) {
		cap := capture.NewCapture()
		t.Cleanup(cap.Close)
		respondToNextQuery(t, cap, json.RawMessage(`{
			"success":true,
			"result":{"ok":false,"message":"store missing"}
		}`))
		if _, err := Entries(cap, "app", "missing", 5); err == nil || err.Error() != "store missing" {
			t.Fatalf("error = %v", err)
		}
	})
}
