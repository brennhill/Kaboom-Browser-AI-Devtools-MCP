// timeline.go — Session timeline: merged, time-sorted view of all captured events.
// Purpose: Provides a unified, chronologically-sorted timeline across all capture buffers.
// Why: Extracted from analysis.go to keep files under 800 LOC and isolate timeline logic.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/hints"
)

type timelineEntry struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Summary   string `json:"summary"`
	Data      any    `json:"data,omitempty"`
}

type timelineIncludes struct {
	actions bool
	errors  bool
	network bool
	ws      bool
}

func parseTimelineIncludes(include []string) timelineIncludes {
	if len(include) == 0 {
		return timelineIncludes{actions: true, errors: true, network: true, ws: true}
	}
	var inc timelineIncludes
	for _, v := range include {
		switch v {
		case "actions":
			inc.actions = true
		case "errors":
			inc.errors = true
		case "network":
			inc.network = true
		case "websocket":
			inc.ws = true
		}
	}
	return inc
}

// GetSessionTimeline returns a merged, time-sorted timeline of all captured events.
func GetSessionTimeline(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit   int      `json:"limit"`
		Include []string `json:"include"`
		Summary bool     `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > MaxObserveLimit {
		params.Limit = MaxObserveLimit
	}

	inc := parseTimelineIncludes(params.Include)
	entries := collectTimelineEntries(deps, inc)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	if params.Summary {
		summary := buildTimelineSummary(entries)
		summary["metadata"] = BuildResponseMetadata(deps.GetCapture(), time.Now())
		return mcp.Succeed(req, "Timeline", summary)
	}

	if len(entries) > params.Limit {
		entries = entries[:params.Limit]
	}

	response := map[string]any{
		"entries":  entries,
		"count":    len(entries),
		"metadata": BuildResponseMetadata(deps.GetCapture(), time.Now()),
	}
	if len(entries) == 0 {
		response["hint"] = hints.Timeline()
	}
	return mcp.Succeed(req, "Timeline", response)
}

func collectTimelineEntries(deps Deps, inc timelineIncludes) []timelineEntry {
	cap := deps.GetCapture()
	entries := make([]timelineEntry, 0)
	if inc.actions {
		entries = append(entries, collectTimelineActions(cap)...)
	}
	if inc.errors {
		entries = append(entries, collectTimelineErrors(deps)...)
	}
	if inc.network {
		entries = append(entries, collectTimelineNetwork(cap.GetNetworkWaterfallEntries())...)
	}
	if inc.ws {
		entries = append(entries, collectTimelineWebSocket(cap.GetAllWebSocketEvents())...)
	}
	return entries
}

func collectTimelineActions(cap *capture.Store) []timelineEntry {
	actions := cap.GetAllEnhancedActions()
	entries := make([]timelineEntry, 0, len(actions))
	for _, a := range actions {
		ts := time.UnixMilli(a.Timestamp).Format(time.RFC3339Nano)
		selector := ""
		if css, ok := a.Selectors["css"].(string); ok {
			selector = css
		}
		entries = append(entries, timelineEntry{
			Timestamp: ts,
			Type:      "action",
			Summary:   a.Type + " on " + selector,
		})
	}
	return entries
}

func collectTimelineErrors(deps Deps) []timelineEntry {
	logEntries, _ := deps.GetLogEntries()
	entries := make([]timelineEntry, 0)
	for _, entry := range logEntries {
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}
		ts := logEntryTimestamp(entry)
		msg, _ := entry["message"].(string)
		if len(msg) > 80 {
			msg = msg[:80] + "..."
		}
		entries = append(entries, timelineEntry{
			Timestamp: ts,
			Type:      "error",
			Summary:   msg,
		})
	}
	return entries
}

func collectTimelineNetwork(networkEntries []capture.NetworkWaterfallEntry) []timelineEntry {
	entries := make([]timelineEntry, 0, len(networkEntries))
	for _, n := range networkEntries {
		var ts string
		if !n.Timestamp.IsZero() {
			ts = n.Timestamp.Format(time.RFC3339Nano)
		} else {
			ts = time.Now().Add(-time.Duration(n.StartTime) * time.Millisecond).Format(time.RFC3339Nano)
		}
		entries = append(entries, timelineEntry{
			Timestamp: ts,
			Type:      "network",
			Summary:   n.InitiatorType + " " + n.URL,
		})
	}
	return entries
}

func collectTimelineWebSocket(wsEvents []capture.WebSocketEvent) []timelineEntry {
	entries := make([]timelineEntry, 0, len(wsEvents))
	for _, ws := range wsEvents {
		summary := ws.Event
		if ws.Direction != "" {
			summary += " (" + ws.Direction + ")"
		}
		entries = append(entries, timelineEntry{
			Timestamp: ws.Timestamp,
			Type:      "websocket",
			Summary:   summary,
		})
	}
	return entries
}

func buildTimelineSummary(entries []timelineEntry) map[string]any {
	counts := make(map[string]int)
	var first, last string
	for _, e := range entries {
		counts[e.Type]++
		if first == "" || e.Timestamp < first {
			first = e.Timestamp
		}
		if last == "" || e.Timestamp > last {
			last = e.Timestamp
		}
	}
	result := map[string]any{
		"counts_by_type": counts,
		"total":          len(entries),
	}
	if first != "" {
		result["time_range"] = map[string]string{"first": first, "last": last}
	}
	return result
}
