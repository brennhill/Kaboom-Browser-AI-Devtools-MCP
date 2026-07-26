// Purpose: Applies WebSocket lifecycle events (open, close, error, message) to per-connection state.
// Why: Separates event-driven state transitions from connection storage and query logic.
// Docs: docs/features/feature/observe/index.md

package wsconn

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const (
	maxActiveConns = 20
	maxClosedConns = 10
)

// Tracker groups WebSocket connection tracking fields.
// Protected by the caller's mutex (this type has no separate lock).
type Tracker struct {
	connections map[string]*connectionState       // Active WS connections by ID (max 20 total). LRU eviction via connOrder.
	closedConns []types.WebSocketClosedConnection // Ring buffer of closed connections (max 10, maxClosedConns). Preserves history for a while.
	connOrder   []string                          // Insertion order for LRU eviction of active connections.
}

// connectionState tracks state for an active connection
type connectionState struct {
	id         string
	url        string
	state      string
	openedAt   string
	incoming   directionStats
	outgoing   directionStats
	sampling   bool
	lastSample *types.SamplingInfo
}

type directionStats struct {
	total       int
	bytes       int
	lastAt      string
	lastData    string
	recentTimes []time.Time // timestamps within rate window for rate calculation
}

// NewTracker returns a Tracker with its maps and slices allocated.
func NewTracker() Tracker {
	return Tracker{
		connections: make(map[string]*connectionState),
		closedConns: make([]types.WebSocketClosedConnection, 0),
		connOrder:   make([]string, 0),
	}
}

// TrackEvent applies one event to per-connection lifecycle state.
//
// Failure semantics:
// - Events for unknown IDs are tolerated and ignored where state cannot be reconciled.
func (t *Tracker) TrackEvent(event types.WebSocketEvent) {
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
//   - Active connection map is bounded by maxActiveConns using oldest-id eviction.
//   - connOrder never contains duplicate IDs: re-opening a known ID moves it to
//     the most-recent slot instead of appending a second entry (which would make
//     eviction silently drop a still-open connection's order slot).
func (t *Tracker) trackConnOpen(event types.WebSocketEvent) {
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
func (t *Tracker) trackConnClose(event types.WebSocketEvent) {
	conn := t.connections[event.ID]
	if conn == nil {
		return
	}
	closed := types.WebSocketClosedConnection{
		ID: event.ID, URL: conn.url, State: "closed",
		OpenedAt: conn.openedAt, ClosedAt: event.Timestamp,
		CloseCode: event.CloseCode, CloseReason: event.CloseReason,
	}
	closed.TotalMessages.Incoming = conn.incoming.total
	closed.TotalMessages.Outgoing = conn.outgoing.total

	t.closedConns = append(t.closedConns, closed)
	if len(t.closedConns) > maxClosedConns {
		keep := len(t.closedConns) - maxClosedConns
		surviving := make([]types.WebSocketClosedConnection, maxClosedConns)
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
func (t *Tracker) trackConnMessage(event types.WebSocketEvent) {
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

// Count returns the number of currently-open websocket connections.
func (t *Tracker) Count() int {
	return len(t.connections)
}

// Clear resets all websocket connection-tracker state and returns open-connection count removed.
func (t *Tracker) Clear() int {
	removed := len(t.connections)
	t.connections = make(map[string]*connectionState)
	t.closedConns = make([]types.WebSocketClosedConnection, 0)
	t.connOrder = make([]string, 0)
	return removed
}

// removeFromSlice removes the first occurrence of item from a string slice,
// preserving the order of remaining elements. Allocates a new backing array to avoid GC pinning.
func removeFromSlice(slice []string, item string) []string {
	for i, v := range slice {
		if v == item {
			newSlice := make([]string, len(slice)-1)
			copy(newSlice, slice[:i])
			copy(newSlice[i:], slice[i+1:])
			return newSlice
		}
	}
	return slice
}
