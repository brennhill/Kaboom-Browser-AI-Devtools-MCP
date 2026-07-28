// runtime_reader.go — Adapts runtime telemetry into session snapshot reader interfaces.
// Why: Keeps session snapshot projection independent of the MCP ToolHandler.
// Docs: docs/features/feature/enterprise-audit/index.md

package session

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"sort"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

type RuntimeCaptureReader interface {
	GetNetworkBodies() []types.NetworkBody
	GetWebSocketStatus(types.WebSocketStatusFilter) types.WebSocketStatusResponse
	GetPerformanceSnapshots() []performance.PerformanceSnapshot
	GetTrackingStatus() (bool, int, string)
}

type runtimeStateReader struct {
	entries func() []mcp.LogEntry
	capture RuntimeCaptureReader
}

func NewRuntimeStateReader(entries func() []mcp.LogEntry, captureReader RuntimeCaptureReader) CaptureStateReader {
	return &runtimeStateReader{entries: entries, capture: captureReader}
}

func (r *runtimeStateReader) GetConsoleErrors() []SnapshotError {
	return r.collectConsoleByLevel(map[string]bool{"error": true})
}

func (r *runtimeStateReader) GetConsoleWarnings() []SnapshotError {
	return r.collectConsoleByLevel(map[string]bool{"warn": true, "warning": true})
}

func (r *runtimeStateReader) collectConsoleByLevel(levels map[string]bool) []SnapshotError {
	if r.entries == nil {
		return []SnapshotError{}
	}

	entries := r.entries()

	type key struct {
		level string
		msg   string
	}
	counts := make(map[key]int)
	for _, entry := range entries {
		level, _ := entry["level"].(string)
		if !levels[level] {
			continue
		}
		msg, _ := entry["message"].(string)
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}
		k := key{level: level, msg: msg}
		counts[k]++
	}

	keys := make([]key, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].level != keys[j].level {
			return keys[i].level < keys[j].level
		}
		return keys[i].msg < keys[j].msg
	})

	out := make([]SnapshotError, 0, len(keys))
	for _, k := range keys {
		out = append(out, SnapshotError{
			Type:    k.level,
			Message: k.msg,
			Count:   counts[k],
		})
	}
	return out
}

func (r *runtimeStateReader) GetNetworkRequests() []SnapshotNetworkRequest {
	if r.capture == nil {
		return []SnapshotNetworkRequest{}
	}
	bodies := r.capture.GetNetworkBodies()
	out := make([]SnapshotNetworkRequest, 0, len(bodies))
	for _, body := range bodies {
		out = append(out, SnapshotNetworkRequest{
			Method:       body.Method,
			URL:          body.URL,
			Status:       body.Status,
			Duration:     body.Duration,
			ResponseSize: len(body.ResponseBody),
			ContentType:  body.ContentType,
		})
	}
	return out
}

func (r *runtimeStateReader) GetWSConnections() []SnapshotWSConnection {
	if r.capture == nil {
		return []SnapshotWSConnection{}
	}
	status := r.capture.GetWebSocketStatus(types.WebSocketStatusFilter{})
	out := make([]SnapshotWSConnection, 0, len(status.Connections))
	for _, conn := range status.Connections {
		out = append(out, SnapshotWSConnection{
			URL:         conn.URL,
			State:       conn.State,
			MessageRate: conn.MessageRate.Incoming.PerSecond + conn.MessageRate.Outgoing.PerSecond,
		})
	}
	return out
}

func (r *runtimeStateReader) GetPerformance() *performance.PerformanceSnapshot {
	if r.capture == nil {
		return nil
	}
	snapshots := r.capture.GetPerformanceSnapshots()
	if len(snapshots) == 0 {
		return nil
	}

	var best *performance.PerformanceSnapshot
	var bestTS time.Time
	for i := range snapshots {
		s := snapshots[i]
		ts, err := time.Parse(time.RFC3339Nano, s.Timestamp)
		if err != nil {
			ts, _ = time.Parse(time.RFC3339, s.Timestamp)
		}
		if best == nil || ts.After(bestTS) {
			copied := s
			best = &copied
			bestTS = ts
		}
	}
	return best
}

func (r *runtimeStateReader) GetCurrentPageURL() string {
	if r.capture == nil {
		return ""
	}
	_, _, trackedURL := r.capture.GetTrackingStatus()
	if trackedURL != "" {
		return trackedURL
	}
	if snap := r.GetPerformance(); snap != nil && snap.URL != "" {
		return snap.URL
	}
	bodies := r.capture.GetNetworkBodies()
	if len(bodies) > 0 {
		return bodies[len(bodies)-1].URL
	}
	return ""
}
