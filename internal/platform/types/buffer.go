// Purpose: Defines shared buffer cursor and clear-count contracts used by ring-buffer lifecycle operations.
// Why: Keeps pagination/clear semantics aligned across capture, observe, and health reporting paths.
// Docs: docs/features/feature/ring-buffer/index.md

package types

// ============================================
// Buffer Cursor Types
// ============================================

// BufferCursor tracks pagination state across ring buffers.
// Used for delta queries to avoid re-streaming data.
type BufferCursor struct {
	TotalAdded int64 // Monotonic total (never reset during eviction)
	Count      int   // Current count in buffer (may be less than TotalAdded due to eviction)
}

// ============================================
// Buffer Clear Counts
// ============================================

// BufferClearCounts holds counts of cleared items from each buffer.
// Used to report how many entries were removed during clear operations.
type BufferClearCounts struct {
	NetworkWaterfall int `json:"network_waterfall,omitempty"`
	NetworkBodies    int `json:"network_bodies,omitempty"`
	WebSocketEvents  int `json:"websocket_events,omitempty"`
	WebSocketStatus  int `json:"websocket_status,omitempty"`
	Actions          int `json:"actions,omitempty"`
	Logs             int `json:"logs,omitempty"`
	ExtensionLogs    int `json:"extension_logs,omitempty"`
}

// Total returns sum of all cleared items.
func (c *BufferClearCounts) Total() int {
	return c.NetworkWaterfall + c.NetworkBodies + c.WebSocketEvents +
		c.WebSocketStatus + c.Actions + c.Logs + c.ExtensionLogs
}
