// execute_test.go — Tests IndexedDB query dispatch and result normalization.

package idbquery

import (
	"encoding/json"
	"runtime"
	"strings"
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

func TestListingDefaultsMissingAndNonArrayDatabasePayloads(t *testing.T) {
	for _, payload := range []string{
		`{"success":true,"result":{}}`,
		`{"success":true,"result":{"databases":"unavailable"}}`,
	} {
		t.Run(payload, func(t *testing.T) {
			cap := capture.NewCapture()
			t.Cleanup(cap.Close)
			respondToNextQuery(t, cap, json.RawMessage(payload))
			result, err := Listing(cap)
			if err != nil {
				t.Fatal(err)
			}
			if result["supported"] != true {
				t.Fatalf("supported = %v, want true", result["supported"])
			}
			if _, ok := result["databases"]; !ok {
				t.Fatal("databases default missing")
			}
		})
	}
}

func TestEntriesNormalizesNonArrayAndExistingMetadata(t *testing.T) {
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	respondToNextQuery(t, cap, json.RawMessage(`{
		"success":true,
		"result":{"entries":"opaque","database":"returned-db","store":"returned-store"}
	}`))
	result, err := Entries(cap, "requested-db", "requested-store", 5)
	if err != nil {
		t.Fatal(err)
	}
	if result["count"] != 0 || result["database"] != "returned-db" || result["store"] != "returned-store" {
		t.Fatalf("normalized result = %#v", result)
	}
}

func TestExecuteScriptNormalizesScalarAndErrorEnvelopes(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    any
		wantErr string
	}{
		{"scalar_result", `{"success":true,"result":42}`, float64(42), ""},
		{"failed_with_error", `{"success":false,"error":"denied"}`, nil, "denied"},
		{"failed_with_nested_message", `{"success":false,"result":{"message":"blocked"}}`, nil, "blocked"},
		{"failed_without_detail", `{"success":false}`, nil, "extension execution failed"},
		{"top_level_error", `{"error":"query failed"}`, nil, "query failed"},
		{"plain_payload", `{"ok":true}`, true, ""},
		{"invalid_json", `{`, nil, "failed to parse execute result"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cap := capture.NewCapture()
			t.Cleanup(cap.Close)
			respondToNextQuery(t, cap, json.RawMessage(tc.payload))
			result, err := executeScript(cap, "return 1", "coverage_contract", time.Second)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if value, ok := result["value"]; ok {
				if value != tc.want {
					t.Fatalf("value = %#v, want %#v", value, tc.want)
				}
			} else if result["ok"] != tc.want {
				t.Fatalf("result = %#v, want marker %#v", result, tc.want)
			}
		})
	}
}
