// Purpose: Exposes the locked accessor for WebSocket connection status snapshots.
// Why: Keeps the Capture-mutex boundary in capture while connection state lives in capture/wsconn.
// Docs: docs/features/feature/observe/index.md

package capture

// GetWebSocketStatus returns current connection states
func (c *Capture) GetWebSocketStatus(filter WebSocketStatusFilter) WebSocketStatusResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.wsConnections.Status(filter)
}
