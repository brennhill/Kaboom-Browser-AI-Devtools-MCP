// runtime_reader.go — Adapts runtime telemetry into session snapshot reader interfaces.
// Why: Keeps session snapshot projection independent of the MCP ToolHandler.
// Docs: docs/features/feature/enterprise-audit/index.md

package session

import (
	"sort"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type RuntimeCaptureReader interface {
	Extension() *capture.ExtensionRuntime
	Telemetry() *capture.TelemetryStore
}

type runtimeStateReader struct {
	entries            func() []types.LogEntry
	performanceEntries func() []performance.PerformanceSnapshot
	capture            RuntimeCaptureReader
}

func NewRuntimeStateReader(
	entries func() []types.LogEntry,
	performanceEntries func() []performance.PerformanceSnapshot,
	captureReader RuntimeCaptureReader,
) CaptureStateReader {
	return &runtimeStateReader{
		entries:            entries,
		performanceEntries: performanceEntries,
		capture:            captureReader,
	}
}

func (r *runtimeStateReader) GetConsoleErrors() []types.SnapshotError {
	return r.collectConsoleByLevel(map[string]bool{"error": true})
}

func (r *runtimeStateReader) GetConsoleWarnings() []types.SnapshotError {
	return r.collectConsoleByLevel(map[string]bool{"warn": true, "warning": true})
}

func (r *runtimeStateReader) collectConsoleByLevel(levels map[string]bool) []types.SnapshotError {
	if r.entries == nil {
		return []types.SnapshotError{}
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

	out := make([]types.SnapshotError, 0, len(keys))
	for _, k := range keys {
		out = append(out, types.SnapshotError{
			Type:    k.level,
			Message: k.msg,
			Count:   counts[k],
		})
	}
	return out
}

func (r *runtimeStateReader) GetNetworkRequests() []types.SnapshotNetworkRequest {
	if r.capture == nil {
		return []types.SnapshotNetworkRequest{}
	}
	bodies := r.capture.Telemetry().GetNetworkBodies()
	out := make([]types.SnapshotNetworkRequest, 0, len(bodies))
	for _, body := range bodies {
		out = append(out, types.SnapshotNetworkRequest{
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

func (r *runtimeStateReader) GetWSConnections() []types.SnapshotWSConnection {
	if r.capture == nil {
		return []types.SnapshotWSConnection{}
	}
	status := r.capture.Telemetry().GetWebSocketStatus(types.WebSocketStatusFilter{})
	out := make([]types.SnapshotWSConnection, 0, len(status.Connections))
	for _, conn := range status.Connections {
		out = append(out, types.SnapshotWSConnection{
			URL:         conn.URL,
			State:       conn.State,
			MessageRate: conn.MessageRate.Incoming.PerSecond + conn.MessageRate.Outgoing.PerSecond,
		})
	}
	return out
}

func (r *runtimeStateReader) GetPerformance() *performance.PerformanceSnapshot {
	if r.performanceEntries == nil {
		return nil
	}
	snapshots := r.performanceEntries()
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
	_, _, trackedURL := r.capture.Extension().GetTrackingStatus()
	if trackedURL != "" {
		return trackedURL
	}
	if snap := r.GetPerformance(); snap != nil && snap.URL != "" {
		return snap.URL
	}
	bodies := r.capture.Telemetry().GetNetworkBodies()
	if len(bodies) > 0 {
		return bodies[len(bodies)-1].URL
	}
	return ""
}
