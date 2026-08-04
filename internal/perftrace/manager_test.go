// manager_test.go — Verifies bounded, ordered Chrome trace artifact persistence.

package perftrace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManagerWritesImportableTraceArtifact(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	started, err := m.Start(42)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := m.Append(WirePerformanceTraceChunkRequest{TraceID: started.TraceID, Sequence: 0, Events: []json.RawMessage{
		json.RawMessage(`{"name":"RunTask","ph":"X","ts":1}`),
	}}); err != nil {
		t.Fatalf("Append first chunk: %v", err)
	}
	if err := m.Append(WirePerformanceTraceChunkRequest{TraceID: started.TraceID, Sequence: 1, Events: []json.RawMessage{
		json.RawMessage(`{"name":"FunctionCall","ph":"X","ts":2}`),
	}}); err != nil {
		t.Fatalf("Append second chunk: %v", err)
	}

	result, err := m.Finish(WirePerformanceTraceFinishRequest{
		TraceID: started.TraceID, TabID: 42, URL: "https://app.test/design",
		NavigationID: "nav-123", BuildSHA: "build-abc123",
	})
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if result.EventCount != 2 || result.ChunkCount != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.TabID != 42 || result.URL != "https://app.test/design" || result.NavigationID != "nav-123" || result.BuildSHA != "build-abc123" {
		t.Fatalf("missing target metadata: %+v", result)
	}
	if filepath.Ext(result.ArtifactPath) != ".json" {
		t.Fatalf("artifact path = %q", result.ArtifactPath)
	}
	data, err := os.ReadFile(result.ArtifactPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var trace struct {
		TraceEvents []map[string]any `json:"traceEvents"`
	}
	if err := json.Unmarshal(data, &trace); err != nil {
		t.Fatalf("artifact is not valid trace JSON: %v\n%s", err, data)
	}
	if len(trace.TraceEvents) != 2 || trace.TraceEvents[1]["name"] != "FunctionCall" {
		t.Fatalf("trace events = %#v", trace.TraceEvents)
	}
}

func TestManagerRejectsConcurrentAndOutOfOrderWrites(t *testing.T) {
	m := NewManager(t.TempDir())
	started, err := m.Start(7)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := m.Start(8); err == nil {
		t.Fatal("second Start succeeded")
	}
	if err := m.Append(WirePerformanceTraceChunkRequest{TraceID: started.TraceID, Sequence: 1}); err == nil {
		t.Fatal("out-of-order append succeeded")
	}
	if err := m.Append(WirePerformanceTraceChunkRequest{TraceID: "stale", Sequence: 0}); err == nil {
		t.Fatal("stale trace append succeeded")
	}
}

func TestManagerAbortRemovesPartialArtifact(t *testing.T) {
	m := NewManager(t.TempDir())
	started, err := m.Start(9)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := m.Abort(started.TraceID); err != nil {
		t.Fatalf("Abort: %v", err)
	}
	if _, err := os.Stat(started.PartialPath); !os.IsNotExist(err) {
		t.Fatalf("partial artifact still exists: %v", err)
	}
	if _, err := m.Start(10); err != nil {
		t.Fatalf("Start after abort: %v", err)
	}
}

func TestManagerRejectsInvalidChunkAtomically(t *testing.T) {
	m := NewManager(t.TempDir())
	started, err := m.Start(11)
	if err != nil {
		t.Fatal(err)
	}
	err = m.Append(WirePerformanceTraceChunkRequest{TraceID: started.TraceID, Sequence: 0, Events: []json.RawMessage{
		json.RawMessage(`{"name":"valid"}`), json.RawMessage(`{"name":`),
	}})
	if err == nil {
		t.Fatal("invalid chunk succeeded")
	}
	result, err := m.Finish(WirePerformanceTraceFinishRequest{TraceID: started.TraceID})
	if err != nil {
		t.Fatal(err)
	}
	if result.EventCount != 0 {
		t.Fatalf("invalid chunk partially wrote %d events", result.EventCount)
	}
}
