// Purpose: Projects tracked WebSocket connection state into the observe-tool status response.
// Why: Isolates status/rate formatting from the lifecycle state machine that feeds it.
// Docs: docs/features/feature/observe/index.md

package wsconn

import (
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const rateWindow = 5 * time.Second // rolling window for msg/s calculation

// Status builds websocket status response with optional URL/connection filters.
func (t *Tracker) Status(filter types.WebSocketStatusFilter) types.WebSocketStatusResponse {
	resp := types.WebSocketStatusResponse{
		Connections: make([]types.WebSocketConnection, 0),
		Closed:      make([]types.WebSocketClosedConnection, 0),
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

// updateDirectionStats mutates per-direction counters and recency windows.
//
// Invariants:
// - recentTimes contains only timestamps within rateWindow after appendAndPrune.
func updateDirectionStats(stats *directionStats, event types.WebSocketEvent, msgTime time.Time) {
	stats.total++
	stats.bytes += event.Size
	stats.lastAt = event.Timestamp
	stats.lastData = event.Data
	stats.recentTimes = appendAndPrune(stats.recentTimes, msgTime)
}

// appendAndPrune maintains a bounded-by-time event window.
//
// Invariants:
// - Returned slice preserves chronological order of surviving timestamps.
// - Prunes in-place to avoid allocation on every call.
func appendAndPrune(times []time.Time, t time.Time) []time.Time {
	cutoff := time.Now().Add(-rateWindow)
	// Prune old entries in-place
	start := 0
	for start < len(times) && times[start].Before(cutoff) {
		start++
	}
	times = times[start:]
	if !t.IsZero() {
		times = append(times, t)
	}
	return times
}

// calcRate returns messages per second from recent timestamps within the rate window
func calcRate(times []time.Time) float64 {
	now := time.Now()
	cutoff := now.Add(-rateWindow)
	count := 0
	for _, t := range times {
		if t.After(cutoff) {
			count++
		}
	}
	if count == 0 {
		return 0.0
	}
	return float64(count) / rateWindow.Seconds()
}

// formatDuration delegates to util.FormatDuration for human-readable duration formatting.
func formatDuration(d time.Duration) string {
	return util.FormatDuration(d)
}

// formatAge formats the age of a timestamp relative to now (e.g., "0.2s", "3s", "2m30s")
func formatAge(ts string) string {
	t := util.ParseTimestamp(ts)
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	return formatDuration(d)
}

// buildWSConnection converts internal connection state to the API response type.
func buildWSConnection(conn *connectionState) types.WebSocketConnection {
	wc := types.WebSocketConnection{
		ID:       conn.id,
		URL:      conn.url,
		State:    conn.state,
		OpenedAt: conn.openedAt,
		MessageRate: types.WebSocketMessageRate{
			Incoming: types.WebSocketDirectionStats{
				PerSecond: calcRate(conn.incoming.recentTimes),
				Total:     conn.incoming.total,
				Bytes:     conn.incoming.bytes,
			},
			Outgoing: types.WebSocketDirectionStats{
				PerSecond: calcRate(conn.outgoing.recentTimes),
				Total:     conn.outgoing.total,
				Bytes:     conn.outgoing.bytes,
			},
		},
		Sampling: types.WebSocketSamplingStatus{Active: conn.sampling},
	}
	if openedTime := util.ParseTimestamp(conn.openedAt); !openedTime.IsZero() {
		wc.Duration = formatDuration(time.Since(openedTime))
	}
	if conn.incoming.lastData != "" {
		wc.LastMessage.Incoming = &types.WebSocketMessagePreview{
			At: conn.incoming.lastAt, Age: formatAge(conn.incoming.lastAt), Preview: conn.incoming.lastData,
		}
	}
	if conn.outgoing.lastData != "" {
		wc.LastMessage.Outgoing = &types.WebSocketMessagePreview{
			At: conn.outgoing.lastAt, Age: formatAge(conn.outgoing.lastAt), Preview: conn.outgoing.lastData,
		}
	}
	return wc
}
