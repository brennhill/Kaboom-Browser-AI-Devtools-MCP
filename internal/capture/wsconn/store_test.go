// store_test.go — Verifies coherent WebSocket event and connection ownership.
package wsconn

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestStoreCommitsDetachedEventsAndConnectionStateTogether(t *testing.T) {
	store := NewStore(3, 4096)
	now := time.Unix(1700000000, 0)
	sampled := &types.SamplingInfo{Rate: "1:1"}
	events := []types.WebSocketEvent{
		{Event: "open", ID: "ws-1", URL: "wss://example.test/socket", Timestamp: "2023-11-14T22:13:20Z"},
		{Event: "message", ID: "ws-1", URL: "wss://example.test/socket", Direction: "incoming", Data: "hello", Timestamp: "2023-11-14T22:13:20Z", Sampled: sampled, TestIDs: []string{"test-1"}},
	}
	store.Add(events, now)
	events[1].Data = "mutated"
	events[1].TestIDs[0] = "mutated"
	sampled.Rate = "mutated"

	snapshot := store.Snapshot()
	if len(snapshot.Events) != 2 || snapshot.Events[1].Data != "hello" || snapshot.Events[1].TestIDs[0] != "test-1" || snapshot.Events[1].Sampled.Rate != "1:1" {
		t.Fatalf("stored events alias caller input: %+v", snapshot.Events)
	}
	snapshot.Events[1].TestIDs[0] = "snapshot-mutated"
	if store.Snapshot().Events[1].TestIDs[0] != "test-1" {
		t.Fatal("stored event aliases returned snapshot")
	}
	status := store.Status(types.WebSocketStatusFilter{})
	if len(status.Connections) != 1 || status.Connections[0].ID != "ws-1" || status.Connections[0].MessageRate.Incoming.Total != 1 {
		t.Fatalf("connection state was not committed with events: %+v", status)
	}
}

func TestStoreBoundsFiltersAndClearsOneWebSocketOwner(t *testing.T) {
	store := NewStore(2, 4096)
	now := time.Unix(1700000000, 0)
	store.Add([]types.WebSocketEvent{
		{Event: "open", ID: "ws-1", URL: "wss://example.test/one", Timestamp: "2023-11-14T22:13:20Z"},
		{Event: "message", ID: "ws-1", URL: "wss://example.test/one", Direction: "incoming", Data: "one", Timestamp: "2023-11-14T22:13:20Z"},
		{Event: "message", ID: "ws-1", URL: "wss://example.test/one", Direction: "outgoing", Data: "two", Timestamp: "2023-11-14T22:13:20Z"},
	}, now)

	state := store.statsAt(now.Add(time.Second))
	if state.Count != 2 || state.Capacity != 2 || state.TotalAdded != 3 || state.ConnectionCount != 1 || state.Pressure.Dropped != 1 || state.MemoryBytes <= 0 {
		t.Fatalf("Stats() = %+v, want bounded coherent state", state)
	}
	if state.Pressure.OldestAge != time.Second {
		t.Fatalf("oldest age = %v, want 1s", state.Pressure.OldestAge)
	}
	filtered := store.Events(types.WebSocketEventFilter{Direction: "incoming", Limit: 10})
	if len(filtered) != 1 || filtered[0].Data != "one" {
		t.Fatalf("filtered events = %+v, want incoming event", filtered)
	}
	bounded := store.Events(types.WebSocketEventFilter{Limit: int(^uint(0) >> 1)})
	if cap(bounded) > state.Count {
		t.Fatalf("oversized requested limit allocated capacity %d for %d retained events", cap(bounded), state.Count)
	}
	cleared := store.Clear()
	if cleared.Events != 2 || cleared.Connections != 1 {
		t.Fatalf("Clear() = %+v, want two events and one connection", cleared)
	}
	clearedState := store.statsAt(now)
	if clearedState.Count != 0 || clearedState.TotalAdded != 0 || clearedState.MemoryBytes != 0 || clearedState.ConnectionCount != 0 || clearedState.Pressure.Dropped != 1 {
		t.Fatalf("state after clear = %+v", clearedState)
	}
}

func TestStoreEvictsForMemoryInOnePassAndStatsDoNotAllocate(t *testing.T) {
	store := NewStore(10, 450)
	now := time.Unix(1700000000, 0)
	store.Add([]types.WebSocketEvent{
		{Event: "message", ID: "ws-1", Data: "123456789012345678901234567890"},
		{Event: "message", ID: "ws-1", Data: "123456789012345678901234567890"},
	}, now)
	state := store.statsAt(now)
	if state.Count != 1 || state.Pressure.Dropped != 1 {
		t.Fatalf("memory-bounded state = %+v, want one retained and one dropped", state)
	}
	if allocations := testing.AllocsPerRun(1000, func() { _ = store.Stats() }); allocations != 0 {
		t.Fatalf("Stats allocated %.2f times per call, want 0", allocations)
	}
}
