package capture

import "testing"

func TestWSConnectionTracker_StatusFiltersOpenAndClosed(t *testing.T) {
	tracker := WSConnectionTracker{
		connections: map[string]*connectionState{
			"conn-a": {id: "conn-a", url: "wss://chat.example/ws", state: "open", openedAt: "2026-03-03T09:00:00Z"},
			"conn-b": {id: "conn-b", url: "wss://prices.example/ws", state: "open", openedAt: "2026-03-03T09:00:01Z"},
		},
		closedConns: []WebSocketClosedConnection{
			{ID: "conn-c", URL: "wss://chat.example/ws", State: "closed"},
		},
		connOrder: []string{"conn-a", "conn-b"},
	}

	status := tracker.status(WebSocketStatusFilter{URLFilter: "chat"})
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
	tracker := WSConnectionTracker{
		connections: map[string]*connectionState{},
	}

	tracker.trackConnOpen(WebSocketEvent{Event: "open", ID: "conn-a", URL: "wss://a.example/ws", Timestamp: "2026-06-10T09:00:00Z"})
	tracker.trackConnOpen(WebSocketEvent{Event: "open", ID: "conn-b", URL: "wss://b.example/ws", Timestamp: "2026-06-10T09:00:01Z"})

	// Re-open conn-a: it must move to the most-recent slot, not duplicate.
	tracker.trackConnOpen(WebSocketEvent{Event: "open", ID: "conn-a", URL: "wss://a.example/ws", Timestamp: "2026-06-10T09:00:02Z"})

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
	tracker.trackConnClose(WebSocketEvent{Event: "close", ID: "conn-a", Timestamp: "2026-06-10T09:00:03Z"})
	if len(tracker.connOrder) != 1 || tracker.connOrder[0] != "conn-b" {
		t.Fatalf("connOrder after close = %v, want [conn-b]", tracker.connOrder)
	}
	if _, ok := tracker.connections["conn-a"]; ok {
		t.Fatal("conn-a should be removed from connections after close")
	}
}

func TestWSConnectionTracker_ClearResetsState(t *testing.T) {
	tracker := WSConnectionTracker{
		connections: map[string]*connectionState{
			"conn-a": {id: "conn-a", url: "wss://chat.example/ws", state: "open"},
			"conn-b": {id: "conn-b", url: "wss://prices.example/ws", state: "open"},
		},
		closedConns: []WebSocketClosedConnection{
			{ID: "conn-c", URL: "wss://chat.example/ws", State: "closed"},
		},
		connOrder: []string{"conn-a", "conn-b"},
	}

	removed := tracker.clear()
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	if tracker.connectionCount() != 0 {
		t.Fatalf("connection count = %d, want 0", tracker.connectionCount())
	}
	if len(tracker.closedConns) != 0 {
		t.Fatalf("closedConns len = %d, want 0", len(tracker.closedConns))
	}
	if len(tracker.connOrder) != 0 {
		t.Fatalf("connOrder len = %d, want 0", len(tracker.connOrder))
	}
}
