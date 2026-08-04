// correlate_test.go — Tests local browser-to-backend trace correlation.
// Docs: docs/features/feature/backend-trace-correlation/index.md

package tracecorr

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestCorrelateLinksTraceparentToBackendBreakdown(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "traces.json")
	payload := `{"spans":[
      {"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","span_id":"a","name":"edge","start_time_unix_nano":1000000,"end_time_unix_nano":11000000,"attributes":{"service.name":"edge"}},
      {"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","span_id":"b","parent_span_id":"a","name":"SELECT projects","start_time_unix_nano":3000000,"end_time_unix_nano":8000000,"attributes":{"db.system":"postgresql"}}
    ]}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	entries := []types.NetworkWaterfallEntry{{
		URL: "https://app.test/api", Traceparent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	}}
	result := CorrelateFile(path, entries)
	if result.Status != "correlated" || len(result.Requests) != 1 || len(result.Requests[0].Spans) != 2 {
		t.Fatalf("correlation = %+v", result)
	}
	if result.Requests[0].Breakdown["sql"] != 5 {
		t.Fatalf("SQL breakdown = %+v", result.Requests[0].Breakdown)
	}
}

func TestCorrelateDistinguishesMissingAndAmbiguousEvidence(t *testing.T) {
	missing := CorrelateFile("", []types.NetworkWaterfallEntry{{Traceparent: "00-abc-def-01"}})
	if missing.Status != "not_configured" {
		t.Fatalf("missing source = %+v", missing)
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "traces.json")
	if err := os.WriteFile(path, []byte(`{"spans":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	unmatched := CorrelateFile(path, []types.NetworkWaterfallEntry{{Traceparent: "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01"}})
	if unmatched.Status != "no_matches" || unmatched.Requests[0].Status != "unmatched" {
		t.Fatalf("unmatched = %+v", unmatched)
	}
	ambiguousPath := filepath.Join(directory, "ambiguous.json")
	ambiguousPayload := `{"spans":[{"trace_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","span_id":"a","name":"one","start_time_unix_nano":1,"end_time_unix_nano":2,"attributes":{"http.request_id":"req-7"}},{"trace_id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","span_id":"b","name":"two","start_time_unix_nano":1,"end_time_unix_nano":2,"attributes":{"http.request_id":"req-7"}}]}`
	if err := os.WriteFile(ambiguousPath, []byte(ambiguousPayload), 0o600); err != nil {
		t.Fatal(err)
	}
	ambiguous := CorrelateFile(ambiguousPath, []types.NetworkWaterfallEntry{{RequestID: "req-7"}})
	if ambiguous.Status != "ambiguous" || ambiguous.Requests[0].Status != "ambiguous" {
		t.Fatalf("ambiguous request ID = %+v", ambiguous)
	}
}

func TestCorrelateReadsOTLPJSONExportShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "otlp.json")
	payload := `{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"api"}}]},"scopeSpans":[{"spans":[{"traceId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","spanId":"bbbbbbbbbbbbbbbb","name":"redis GET","startTimeUnixNano":"1000000","endTimeUnixNano":"5000000","attributes":[{"key":"db.system","value":{"stringValue":"redis"}}]}]}]}]}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	result := CorrelateFile(path, []types.NetworkWaterfallEntry{{Traceparent: "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-cccccccccccccccc-01"}})
	if result.Status != "correlated" || result.Requests[0].Spans[0].Service != "api" || result.Requests[0].Breakdown["redis"] != 4 {
		t.Fatalf("OTLP correlation = %+v", result)
	}
}
