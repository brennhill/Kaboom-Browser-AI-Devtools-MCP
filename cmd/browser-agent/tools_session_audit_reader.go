// Purpose: Adapts ToolHandler runtime state into session snapshot reader interfaces.
// Why: Isolates snapshot extraction logic from audit trail persistence utilities.
// Docs: docs/features/feature/enterprise-audit/index.md

package main

import (
	"sort"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session"
)

// toolCaptureStateReader adapts ToolHandler state to session.CaptureStateReader.
type toolCaptureStateReader struct {
	h *ToolHandler
}

func newToolCaptureStateReader(h *ToolHandler) session.CaptureStateReader {
	return &toolCaptureStateReader{h: h}
}

func (r *toolCaptureStateReader) GetConsoleErrors() []session.SnapshotError {
	return r.collectConsoleByLevel(map[string]bool{"error": true})
}

func (r *toolCaptureStateReader) GetConsoleWarnings() []session.SnapshotError {
	return r.collectConsoleByLevel(map[string]bool{"warn": true, "warning": true})
}

func (r *toolCaptureStateReader) collectConsoleByLevel(levels map[string]bool) []session.SnapshotError {
	if r.h == nil || r.h.server == nil {
		return []session.SnapshotError{}
	}

	entries := r.h.server.logs.Entries()

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

	out := make([]session.SnapshotError, 0, len(keys))
	for _, k := range keys {
		out = append(out, session.SnapshotError{
			Type:    k.level,
			Message: k.msg,
			Count:   counts[k],
		})
	}
	return out
}

func (r *toolCaptureStateReader) GetNetworkRequests() []session.SnapshotNetworkRequest {
	if r.h == nil || r.h.capture == nil {
		return []session.SnapshotNetworkRequest{}
	}
	bodies := r.h.capture.GetNetworkBodies()
	out := make([]session.SnapshotNetworkRequest, 0, len(bodies))
	for _, body := range bodies {
		out = append(out, session.SnapshotNetworkRequest{
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

func (r *toolCaptureStateReader) GetWSConnections() []session.SnapshotWSConnection {
	if r.h == nil || r.h.capture == nil {
		return []session.SnapshotWSConnection{}
	}
	status := r.h.capture.GetWebSocketStatus(capture.WebSocketStatusFilter{})
	out := make([]session.SnapshotWSConnection, 0, len(status.Connections))
	for _, conn := range status.Connections {
		out = append(out, session.SnapshotWSConnection{
			URL:         conn.URL,
			State:       conn.State,
			MessageRate: conn.MessageRate.Incoming.PerSecond + conn.MessageRate.Outgoing.PerSecond,
		})
	}
	return out
}

func (r *toolCaptureStateReader) GetPerformance() *performance.Snapshot {
	if r.h == nil || r.h.capture == nil {
		return nil
	}
	snapshots := r.h.capture.GetPerformanceSnapshots()
	if len(snapshots) == 0 {
		return nil
	}

	var best *performance.Snapshot
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

func (r *toolCaptureStateReader) GetCurrentPageURL() string {
	if r.h == nil || r.h.capture == nil {
		return ""
	}
	_, _, trackedURL := r.h.capture.GetTrackingStatus()
	if trackedURL != "" {
		return trackedURL
	}
	if snap := r.GetPerformance(); snap != nil && snap.URL != "" {
		return snap.URL
	}
	bodies := r.h.capture.GetNetworkBodies()
	if len(bodies) > 0 {
		return bodies[len(bodies)-1].URL
	}
	return ""
}
