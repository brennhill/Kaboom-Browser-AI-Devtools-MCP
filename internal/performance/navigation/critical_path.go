// critical_path.go — Models ordered browser navigation evidence without inventing missing timings.
// Docs: docs/features/feature/navigation-critical-path/index.md

package navigation

import (
	"sort"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type Phase struct {
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	StartMs    *float64 `json:"start_ms,omitempty"`
	EndMs      *float64 `json:"end_ms,omitempty"`
	DurationMs *float64 `json:"duration_ms,omitempty"`
	Evidence   string   `json:"evidence,omitempty"`
}

type Gap struct {
	Before     string   `json:"before"`
	After      string   `json:"after"`
	Status     string   `json:"status"`
	DurationMs *float64 `json:"duration_ms,omitempty"`
}

type CriticalPath struct {
	Status          string  `json:"status"`
	URL             string  `json:"url,omitempty"`
	Phases          []Phase `json:"phases"`
	Gaps            []Gap   `json:"gaps"`
	DominantSegment string  `json:"dominant_segment,omitempty"`
	DominantMs      float64 `json:"dominant_ms,omitempty"`
}

func BuildCriticalPath(snapshot performance.PerformanceSnapshot, waterfall []types.NetworkWaterfallEntry) CriticalPath {
	ordered := append([]types.NetworkWaterfallEntry(nil), waterfall...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].StartTime < ordered[j].StartTime })
	phases := []Phase{
		metricPhase("navigation_ttfb", 0, snapshot.Timing.TimeToFirstByte, "navigation timing"),
		requestPhase("authentication", selectRequest(ordered, isAuthenticationRequest)),
		requestPhase("backend_response", selectRequest(ordered, isApplicationRequest)),
		timingPhase("state_update", selectUserTiming(snapshot.UserTiming, []string{"state", "store", "dispatch", "update"})),
		timingPhase("react_commit", selectUserTiming(snapshot.UserTiming, []string{"react", "commit", "render"})),
		milestonePhase("first_contentful_paint", snapshot.Timing.FirstContentfulPaint, nil),
		milestonePhase("largest_contentful_paint", snapshot.Timing.LargestContentfulPaint, snapshot.Timing.FirstContentfulPaint),
	}
	result := CriticalPath{Status: "complete", URL: snapshot.URL, Phases: phases, Gaps: buildGaps(phases)}
	for _, phase := range phases {
		if phase.Status == "unavailable" {
			result.Status = "partial"
		}
		if phase.DurationMs != nil && *phase.DurationMs > result.DominantMs {
			result.DominantMs = *phase.DurationMs
			result.DominantSegment = phase.Name
		}
	}
	return result
}

func buildGaps(phases []Phase) []Gap {
	gaps := make([]Gap, 0, len(phases)-1)
	for index := 1; index < len(phases); index++ {
		before, after := phases[index-1], phases[index]
		gap := Gap{Before: before.Name, After: after.Name, Status: "unavailable"}
		if before.EndMs != nil && after.StartMs != nil {
			duration := *after.StartMs - *before.EndMs
			if duration < 0 {
				duration = 0
			}
			gap.Status = "available"
			gap.DurationMs = ptr(duration)
		}
		gaps = append(gaps, gap)
	}
	return gaps
}

func metricPhase(name string, start, duration float64, evidence string) Phase {
	if duration <= 0 {
		return unavailablePhase(name)
	}
	end := start + duration
	return Phase{Name: name, Status: "available", StartMs: ptr(start), EndMs: ptr(end), DurationMs: ptr(duration), Evidence: evidence}
}

func requestPhase(name string, entry *types.NetworkWaterfallEntry) Phase {
	if entry == nil {
		return unavailablePhase(name)
	}
	end := entry.ResponseEnd
	if end <= 0 {
		end = entry.StartTime + entry.Duration
	}
	return Phase{
		Name: name, Status: "available", StartMs: ptr(entry.StartTime), EndMs: ptr(end), DurationMs: ptr(entry.Duration),
		Evidence: requestEvidence(*entry),
	}
}

func timingPhase(name string, timing *performance.UserTimingEntry) Phase {
	if timing == nil || timing.Duration <= 0 {
		return unavailablePhase(name)
	}
	return Phase{
		Name: name, Status: "available", StartMs: ptr(timing.StartTime), EndMs: ptr(timing.StartTime + timing.Duration),
		DurationMs: ptr(timing.Duration), Evidence: "user_timing:" + timing.Name,
	}
}

func milestonePhase(name string, value, prior *float64) Phase {
	if value == nil {
		return unavailablePhase(name)
	}
	start := 0.0
	if prior != nil && *prior <= *value {
		start = *prior
	}
	duration := *value - start
	return Phase{Name: name, Status: "available", StartMs: ptr(start), EndMs: ptr(*value), DurationMs: ptr(duration), Evidence: "performance_observer"}
}

func unavailablePhase(name string) Phase {
	return Phase{Name: name, Status: "unavailable", Evidence: "browser_or_application_evidence_unavailable"}
}

func selectRequest(entries []types.NetworkWaterfallEntry, match func(types.NetworkWaterfallEntry) bool) *types.NetworkWaterfallEntry {
	var selected *types.NetworkWaterfallEntry
	for index := range entries {
		entry := &entries[index]
		if !match(*entry) || (selected != nil && entry.Duration <= selected.Duration) {
			continue
		}
		selected = entry
	}
	return selected
}

func isAuthenticationRequest(entry types.NetworkWaterfallEntry) bool {
	value := strings.ToLower(entry.URL + " " + entry.RouteLoader + " " + entry.StoreAction)
	return containsAny(value, []string{"auth", "token", "session", "clerk", "login", "oauth"})
}

func isApplicationRequest(entry types.NetworkWaterfallEntry) bool {
	return !isAuthenticationRequest(entry) && (entry.InitiatorType == "fetch" || entry.InitiatorType == "xmlhttprequest" || strings.Contains(entry.URL, "/api/"))
}

func selectUserTiming(data *performance.UserTimingData, names []string) *performance.UserTimingEntry {
	if data == nil {
		return nil
	}
	var selected *performance.UserTimingEntry
	for index := range data.Measures {
		entry := &data.Measures[index]
		if containsAny(strings.ToLower(entry.Name), names) && (selected == nil || entry.Duration > selected.Duration) {
			selected = entry
		}
	}
	return selected
}

func requestEvidence(entry types.NetworkWaterfallEntry) string {
	if entry.RequestID != "" {
		return "request_id:" + entry.RequestID
	}
	if entry.Traceparent != "" {
		return "traceparent"
	}
	return "resource_timing"
}

func containsAny(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func ptr(value float64) *float64 { return &value }
