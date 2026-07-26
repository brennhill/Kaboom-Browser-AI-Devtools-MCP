// Purpose: Encapsulates websocket connection-tracker view/reset helpers behind focused methods.
// Why: Reduces direct map/slice manipulation across Capture methods during god-object decomposition.
// Docs: docs/features/feature/observe/index.md

package capture

import "strings"

// status builds websocket status response with optional URL/connection filters.
func (t *WSConnectionTracker) status(filter WebSocketStatusFilter) WebSocketStatusResponse {
	resp := WebSocketStatusResponse{
		Connections: make([]WebSocketConnection, 0),
		Closed:      make([]WebSocketClosedConnection, 0),
	}

	for _, conn := range t.connections {
		if filter.URLFilter != "" && !strings.Contains(conn.url, filter.URLFilter) {
			continue
		}
		if filter.ConnectionID != "" && conn.id != filter.ConnectionID {
			continue
		}
		resp.Connections = append(resp.Connections, buildWSConnection(conn))
	}

	for _, closed := range t.closedConns {
		if filter.URLFilter != "" && !strings.Contains(closed.URL, filter.URLFilter) {
			continue
		}
		if filter.ConnectionID != "" && closed.ID != filter.ConnectionID {
			continue
		}
		resp.Closed = append(resp.Closed, closed)
	}

	return resp
}

// connectionCount returns the number of currently-open websocket connections.
func (t *WSConnectionTracker) connectionCount() int {
	return len(t.connections)
}

// clear resets all websocket connection-tracker state and returns open-connection count removed.
func (t *WSConnectionTracker) clear() int {
	removed := len(t.connections)
	t.connections = make(map[string]*connectionState)
	t.closedConns = make([]WebSocketClosedConnection, 0)
	t.connOrder = make([]string, 0)
	return removed
}
