// manager_test.go — Verifies bounded, ordered Chrome trace artifact persistence.

package perftrace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestManagerEnforcesTotalBytesAndDuration(t *testing.T) {
	now := time.Unix(100, 0)
	m := newManagerWithLimits(t.TempDir(), 48, time.Second, func() time.Time { return now })
	started, err := m.Start(42)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Append(WirePerformanceTraceChunkRequest{TraceID: started.TraceID, Sequence: 0, Events: []json.RawMessage{json.RawMessage(`{"payload":"` + strings.Repeat("x", 64) + `"}`)}}); !errors.Is(err, ErrTraceSizeLimit) {
		t.Fatalf("size error = %v", err)
	}
	now = now.Add(2 * time.Second)
	if err := m.Append(WirePerformanceTraceChunkRequest{TraceID: started.TraceID, Sequence: 0}); !errors.Is(err, ErrTraceDurationLimit) {
		t.Fatalf("duration error = %v", err)
	}
}

func TestManagerReplacesOrphanedActiveTrace(t *testing.T) {
	m := NewManager(t.TempDir())
	first, err := m.Start(7)
	if err != nil {
		t.Fatal(err)
	}
	second, recovered, err := m.StartReplacing(8, true)
	if err != nil || !recovered || second.TraceID == first.TraceID {
		t.Fatalf("replacement = %+v recovered=%v err=%v", second, recovered, err)
	}
	if _, err := os.Stat(first.PartialPath); !os.IsNotExist(err) {
		t.Fatalf("orphan partial remains: %v", err)
	}
}

func TestManagerDoesNotDiscardActiveTraceForInvalidReplacement(t *testing.T) {
	m := NewManager(t.TempDir())
	first, err := m.Start(7)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.StartReplacing(0, true); err == nil {
		t.Fatal("invalid replacement succeeded")
	}
	if err := m.Abort(first.TraceID); err != nil {
		t.Fatalf("original trace was discarded: %v", err)
	}
}

func TestManagerRecoversPartialArtifactAfterDaemonRestart(t *testing.T) {
	dir := t.TempDir()
	partial := filepath.Join(dir, "cpu-stale.json.partial")
	if err := os.WriteFile(partial, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := NewManager(dir)
	_, recovered, err := m.StartReplacing(8, true)
	if err != nil || !recovered {
		t.Fatalf("recovered=%v err=%v", recovered, err)
	}
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatalf("stale partial remains: %v", err)
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
