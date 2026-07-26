// Purpose: Tests WebSocket connection tracker status filtering, LRU order, and reset behavior.
// Docs: docs/features/feature/observe/index.md

package wsconn

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestWSConnectionTracker_StatusFiltersOpenAndClosed(t *testing.T) {
	tracker := Tracker{
		connections: map[string]*connectionState{
			"conn-a": {id: "conn-a", url: "wss://chat.example/ws", state: "open", openedAt: "2026-03-03T09:00:00Z"},
			"conn-b": {id: "conn-b", url: "wss://prices.example/ws", state: "open", openedAt: "2026-03-03T09:00:01Z"},
		},
		closedConns: []types.WebSocketClosedConnection{
			{ID: "conn-c", URL: "wss://chat.example/ws", State: "closed"},
		},
		connOrder: []string{"conn-a", "conn-b"},
	}

	status := tracker.Status(types.WebSocketStatusFilter{URLFilter: "chat"})
	if len(status.Connections) != 1 {
		t.Fatalf("connections len = %d, want 1", len(status.Connections))
	}
	if status.Connections[0].ID != "conn-a" {
		t.Fatalf("connection id = %q, want conn-a", status.Connections[0].ID)
	}
	if len(status.Closed) != 1 {
		t.Fatalf("closed len = %d, want 1", len(status.Closed))
	}
	if status.Closed[0].ID != "conn-c" {
		t.Fatalf("closed id = %q, want conn-c", status.Closed[0].ID)
	}
}

// TestWSConnectionTracker_ReopenDeduplicatesConnOrder is the regression test
// for duplicate connOrder entries: re-opening an existing connection ID
// appended a second order slot, so LRU eviction could pop a stale slot and
// evict the wrong (still-open) connection.
func TestWSConnectionTracker_ReopenDeduplicatesConnOrder(t *testing.T) {
	tracker := Tracker{
		connections: map[string]*connectionState{},
	}

	tracker.trackConnOpen(types.WebSocketEvent{Event: "open", ID: "conn-a", URL: "wss://a.example/ws", Timestamp: "2026-06-10T09:00:00Z"})
	tracker.trackConnOpen(types.WebSocketEvent{Event: "open", ID: "conn-b", URL: "wss://b.example/ws", Timestamp: "2026-06-10T09:00:01Z"})

	// Re-open conn-a: it must move to the most-recent slot, not duplicate.
	tracker.trackConnOpen(types.WebSocketEvent{Event: "open", ID: "conn-a", URL: "wss://a.example/ws", Timestamp: "2026-06-10T09:00:02Z"})

	if len(tracker.connOrder) != 2 {
		t.Fatalf("connOrder len = %d, want 2 (no duplicates), got %v", len(tracker.connOrder), tracker.connOrder)
	}
	if tracker.connOrder[0] != "conn-b" || tracker.connOrder[1] != "conn-a" {
		t.Fatalf("connOrder = %v, want [conn-b conn-a] (re-open moves to most recent)", tracker.connOrder)
	}
	if len(tracker.connections) != 2 {
		t.Fatalf("connections len = %d, want 2", len(tracker.connections))
	}

	// Closing the re-opened connection must leave no stale order entry behind.
	tracker.trackConnClose(types.WebSocketEvent{Event: "close", ID: "conn-a", Timestamp: "2026-06-10T09:00:03Z"})
	if len(tracker.connOrder) != 1 || tracker.connOrder[0] != "conn-b" {
		t.Fatalf("connOrder after close = %v, want [conn-b]", tracker.connOrder)
	}
	if _, ok := tracker.connections["conn-a"]; ok {
		t.Fatal("conn-a should be removed from connections after close")
	}
}

func TestWSConnectionTracker_ClearResetsState(t *testing.T) {
	tracker := Tracker{
		connections: map[string]*connectionState{
			"conn-a": {id: "conn-a", url: "wss://chat.example/ws", state: "open"},
			"conn-b": {id: "conn-b", url: "wss://prices.example/ws", state: "open"},
		},
		closedConns: []types.WebSocketClosedConnection{
			{ID: "conn-c", URL: "wss://chat.example/ws", State: "closed"},
		},
		connOrder: []string{"conn-a", "conn-b"},
	}

	removed := tracker.Clear()
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	if tracker.Count() != 0 {
		t.Fatalf("connection count = %d, want 0", tracker.Count())
	}
	if len(tracker.closedConns) != 0 {
		t.Fatalf("closedConns len = %d, want 0", len(tracker.closedConns))
	}
	if len(tracker.connOrder) != 0 {
		t.Fatalf("connOrder len = %d, want 0", len(tracker.connOrder))
	}
}

// TestNewTracker_AllocatesContainers pins the initialised-empty (not nil) state that
// Capture relied on when it built the struct literal itself.
func TestNewTracker_AllocatesContainers(t *testing.T) {
	tr := NewTracker()
	if tr.connections == nil {
		t.Error("connections map must be allocated")
	}
	if tr.closedConns == nil {
		t.Error("closedConns slice must be allocated")
	}
	if tr.connOrder == nil {
		t.Error("connOrder slice must be allocated")
	}
	if tr.Count() != 0 {
		t.Errorf("Count() = %d on a fresh tracker, want 0", tr.Count())
	}
}

// TestTrackEvent_DispatchesByEventKind covers the TrackEvent switch: open, message,
// error and close each reach their handler, and unknown kinds are inert.
func TestTrackEvent_DispatchesByEventKind(t *testing.T) {
	tr := NewTracker()

	tr.TrackEvent(types.WebSocketEvent{Event: "open", ID: "c1", URL: "wss://x/ws", Timestamp: "2026-06-10T09:00:00Z"})
	if tr.Count() != 1 {
		t.Fatalf("after open Count() = %d, want 1", tr.Count())
	}

	tr.TrackEvent(types.WebSocketEvent{Event: "message", ID: "c1", Direction: "incoming", Data: "hello", Size: 5, Timestamp: "2026-06-10T09:00:01Z"})
	tr.TrackEvent(types.WebSocketEvent{Event: "message", ID: "c1", Direction: "outgoing", Data: "bye", Size: 3, Timestamp: "2026-06-10T09:00:02Z"})
	conn := tr.connections["c1"]
	if conn.incoming.total != 1 || conn.incoming.bytes != 5 || conn.incoming.lastData != "hello" {
		t.Errorf("incoming stats = %+v, want total 1 / bytes 5 / lastData hello", conn.incoming)
	}
	if conn.outgoing.total != 1 || conn.outgoing.bytes != 3 || conn.outgoing.lastData != "bye" {
		t.Errorf("outgoing stats = %+v, want total 1 / bytes 3 / lastData bye", conn.outgoing)
	}

	// Unknown event kinds must not mutate state.
	tr.TrackEvent(types.WebSocketEvent{Event: "frame", ID: "c1"})
	if tr.connections["c1"].incoming.total != 1 {
		t.Error("unknown event kind must not change direction counters")
	}

	tr.TrackEvent(types.WebSocketEvent{Event: "error", ID: "c1"})
	if tr.connections["c1"].state != "error" {
		t.Errorf("state = %q after error, want error", tr.connections["c1"].state)
	}

	tr.TrackEvent(types.WebSocketEvent{Event: "close", ID: "c1", Timestamp: "2026-06-10T09:00:03Z", CloseCode: 1000})
	if tr.Count() != 0 {
		t.Fatalf("after close Count() = %d, want 0", tr.Count())
	}
	if len(tr.closedConns) != 1 || tr.closedConns[0].TotalMessages.Incoming != 1 || tr.closedConns[0].TotalMessages.Outgoing != 1 {
		t.Errorf("closed summary = %+v, want 1 incoming / 1 outgoing", tr.closedConns)
	}
}

// TestTrackConnMessage_IgnoresUnknownAndRecordsSampling covers the nil-connection
// guard and the sampling side-effect inside trackConnMessage.
func TestTrackConnMessage_IgnoresUnknownAndRecordsSampling(t *testing.T) {
	tr := NewTracker()
	tr.TrackEvent(types.WebSocketEvent{Event: "message", ID: "ghost", Direction: "incoming", Data: "x"})
	if tr.Count() != 0 {
		t.Fatal("a message for an unknown connection must not create one")
	}

	tr.TrackEvent(types.WebSocketEvent{Event: "open", ID: "c1", URL: "wss://x/ws"})
	sampled := &types.SamplingInfo{}
	tr.TrackEvent(types.WebSocketEvent{Event: "message", ID: "c1", Direction: "incoming", Data: "x", Sampled: sampled})
	if !tr.connections["c1"].sampling {
		t.Error("sampling flag must be set when the event carries sampling info")
	}
	if tr.connections["c1"].lastSample != sampled {
		t.Error("lastSample must retain the reported sampling info")
	}
}

// TestAppendAndPrune_DropsTimestampsOutsideWindow pins the rolling-window bound that
// keeps per-connection rate memory from growing without limit.
func TestAppendAndPrune_DropsTimestampsOutsideWindow(t *testing.T) {
	now := time.Now()
	stale := []time.Time{now.Add(-10 * rateWindow), now.Add(-9 * rateWindow), now.Add(-time.Second)}

	got := appendAndPrune(stale, now)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (two stale entries pruned, one appended)", len(got))
	}
	if !got[len(got)-1].Equal(now) {
		t.Error("newest timestamp must be appended last")
	}

	// A zero timestamp prunes but appends nothing.
	if got := appendAndPrune([]time.Time{now.Add(-10 * rateWindow)}, time.Time{}); len(got) != 0 {
		t.Fatalf("len = %d, want 0 (stale pruned, zero time not appended)", len(got))
	}
}
