// Purpose: Applies WebSocket lifecycle events (open, close, error, message) to per-connection state.
// Why: Separates event-driven state transitions from connection storage and query logic.
package capture

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"

// trackEvent applies one event to per-connection lifecycle state.
//
// Failure semantics:
// - Events for unknown IDs are tolerated and ignored where state cannot be reconciled.
func (t *WSConnectionTracker) trackEvent(event WebSocketEvent) {
	switch event.Event {
	case "open":
		t.trackConnOpen(event)
	case "close":
		t.trackConnClose(event)
	case "error":
		if conn := t.connections[event.ID]; conn != nil {
			conn.state = "error"
		}
	case "message":
		t.trackConnMessage(event)
	}
}

// trackConnOpen registers/refreshes active connection metadata.
//
// Invariants:
// - Active connection map is bounded by maxActiveConns using oldest-id eviction.
// - connOrder never contains duplicate IDs: re-opening a known ID moves it to
//   the most-recent slot instead of appending a second entry (which would make
//   eviction silently drop a still-open connection's order slot).
func (t *WSConnectionTracker) trackConnOpen(event WebSocketEvent) {
	if _, exists := t.connections[event.ID]; exists {
		// Re-open of a known ID refreshes state; no eviction needed since the
		// connection count does not grow.
		t.connOrder = removeFromSlice(t.connOrder, event.ID)
	} else if len(t.connections) >= maxActiveConns && len(t.connOrder) > 0 {
		oldestID := t.connOrder[0]
		delete(t.connections, oldestID)
		newOrder := make([]string, len(t.connOrder)-1)
		copy(newOrder, t.connOrder[1:])
		t.connOrder = newOrder
	}
	t.connections[event.ID] = &connectionState{
		id: event.ID, url: event.URL, state: "open", openedAt: event.Timestamp,
	}
	t.connOrder = append(t.connOrder, event.ID)
}

// trackConnClose finalizes a connection and moves summary into closed history.
//
// Invariants:
// - Closed connection history is bounded by maxClosedConns.
//
// Failure semantics:
// - Unknown close events are ignored; no synthetic connection is created.
func (t *WSConnectionTracker) trackConnClose(event WebSocketEvent) {
	conn := t.connections[event.ID]
	if conn == nil {
		return
	}
	closed := WebSocketClosedConnection{
		ID: event.ID, URL: conn.url, State: "closed",
		OpenedAt: conn.openedAt, ClosedAt: event.Timestamp,
		CloseCode: event.CloseCode, CloseReason: event.CloseReason,
	}
	closed.TotalMessages.Incoming = conn.incoming.total
	closed.TotalMessages.Outgoing = conn.outgoing.total

	t.closedConns = append(t.closedConns, closed)
	if len(t.closedConns) > maxClosedConns {
		keep := len(t.closedConns) - maxClosedConns
		surviving := make([]WebSocketClosedConnection, maxClosedConns)
		copy(surviving, t.closedConns[keep:])
		t.closedConns = surviving
	}
	delete(t.connections, event.ID)
	t.connOrder = removeFromSlice(t.connOrder, event.ID)
}

// trackConnMessage updates rate/counter state for an active connection.
//
// Failure semantics:
// - Messages on unknown connections are ignored instead of creating implicit connection records.
func (t *WSConnectionTracker) trackConnMessage(event WebSocketEvent) {
	conn := t.connections[event.ID]
	if conn == nil {
		return
	}
	msgTime := util.ParseTimestamp(event.Timestamp)
	switch event.Direction {
	case "incoming":
		updateDirectionStats(&conn.incoming, event, msgTime)
	case "outgoing":
		updateDirectionStats(&conn.outgoing, event, msgTime)
	}
	if event.Sampled != nil {
		conn.sampling = true
		conn.lastSample = event.Sampled
	}
}
