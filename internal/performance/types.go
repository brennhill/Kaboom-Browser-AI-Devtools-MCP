// Purpose: Declares PerformanceSnapshot, budget threshold, and baseline types for performance monitoring.
// Docs: docs/features/feature/performance-audit/index.md

// types.go — Performance monitoring types
// Handles performance snapshots, baselines, regression detection, and
// causal diffing for identifying performance bottlenecks.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
package performance

import "sort"

var budgetMetrics = map[string]struct{}{
	"load": {}, "fcp": {}, "lcp": {}, "inp": {}, "cls": {},
	"request_count": {}, "transfer_size": {}, "execution_cost": {},
}

func IsBudgetMetric(name string) bool {
	_, ok := budgetMetrics[name]
	return ok
}

func BudgetMetricNames() []string {
	names := make([]string, 0, len(budgetMetrics))
	for name := range budgetMetrics {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ============================================
// Performance Budget Types
// ============================================

// PerformanceSnapshot represents a captured performance snapshot from a page load.
// Wire fields: see WirePerformanceSnapshot in wire_performance.go
type PerformanceSnapshot struct {
	URL               string                 `json:"url"`
	Timestamp         string                 `json:"timestamp"`
	Timing            PerformanceTiming      `json:"timing"`
	Network           NetworkSummary         `json:"network"`
	LongTasks         LongTaskMetrics        `json:"long_tasks"`
	CLS               *float64               `json:"cumulative_layout_shift,omitempty"` // snake_case (from browser LayoutShift)
	VitalsAttribution *WireVitalsAttribution `json:"vitals_attribution,omitempty"`
	Resources         []ResourceEntry        `json:"resources,omitempty"`
	UserTiming        *UserTimingData        `json:"user_timing,omitempty"`
}

// CloneSnapshot returns a deep copy safe for storage and caller mutation.
func CloneSnapshot(snapshot PerformanceSnapshot) PerformanceSnapshot {
	snapshot.Resources = append([]ResourceEntry(nil), snapshot.Resources...)
	snapshot.Network.SlowestRequests = append([]SlowRequest(nil), snapshot.Network.SlowestRequests...)
	if snapshot.Network.ByType != nil {
		byType := make(map[string]TypeSummary, len(snapshot.Network.ByType))
		for key, item := range snapshot.Network.ByType {
			byType[key] = item
		}
		snapshot.Network.ByType = byType
	}
	snapshot.CLS = cloneFloat(snapshot.CLS)
	snapshot.Timing.FirstContentfulPaint = cloneFloat(snapshot.Timing.FirstContentfulPaint)
	snapshot.Timing.LargestContentfulPaint = cloneFloat(snapshot.Timing.LargestContentfulPaint)
	snapshot.Timing.InteractionToNextPaint = cloneFloat(snapshot.Timing.InteractionToNextPaint)
	if snapshot.UserTiming != nil {
		userTiming := *snapshot.UserTiming
		userTiming.Marks = append([]UserTimingEntry(nil), snapshot.UserTiming.Marks...)
		userTiming.Measures = append([]UserTimingEntry(nil), snapshot.UserTiming.Measures...)
		snapshot.UserTiming = &userTiming
	}
	snapshot.VitalsAttribution = cloneVitalsAttribution(snapshot.VitalsAttribution)
	return snapshot
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneVitalsAttribution(source *WireVitalsAttribution) *WireVitalsAttribution {
	if source == nil {
		return nil
	}
	clone := *source
	if source.LCP != nil {
		lcp := *source.LCP
		lcp.Element = cloneElement(source.LCP.Element)
		clone.LCP = &lcp
	}
	if source.INP != nil {
		inp := *source.INP
		inp.Target = cloneElement(source.INP.Target)
		clone.INP = &inp
	}
	clone.CLS.Shifts = append([]WireLayoutShiftAttribution(nil), source.CLS.Shifts...)
	for index := range clone.CLS.Shifts {
		clone.CLS.Shifts[index].Nodes = append([]WireElementDescriptor(nil), source.CLS.Shifts[index].Nodes...)
		for nodeIndex := range clone.CLS.Shifts[index].Nodes {
			clone.CLS.Shifts[index].Nodes[nodeIndex] = cloneElement(source.CLS.Shifts[index].Nodes[nodeIndex])
		}
	}
	clone.LongTasks = append([]WireLongTaskAttribution(nil), source.LongTasks...)
	for index := range clone.LongTasks {
		clone.LongTasks[index].SourceStack = append([]string(nil), source.LongTasks[index].SourceStack...)
	}
	return &clone
}

func cloneElement(source WireElementDescriptor) WireElementDescriptor {
	source.Classes = append([]string(nil), source.Classes...)
	return source
}

// UserTimingData holds captured performance.mark() and performance.measure() entries.
type UserTimingData struct {
	Marks    []UserTimingEntry `json:"marks"`
	Measures []UserTimingEntry `json:"measures"`
}

// UserTimingEntry represents a single performance mark or measure.
type UserTimingEntry struct {
	Name      string  `json:"name"`
	StartTime float64 `json:"start_time"`
	Duration  float64 `json:"duration,omitempty"`
}

// PerformanceTiming holds navigation timing metrics
type PerformanceTiming struct {
	DomContentLoaded       float64  `json:"dom_content_loaded"`
	Load                   float64  `json:"load"`
	FirstContentfulPaint   *float64 `json:"first_contentful_paint"`
	LargestContentfulPaint *float64 `json:"largest_contentful_paint"`
	InteractionToNextPaint *float64 `json:"interaction_to_next_paint,omitempty"`
	TimeToFirstByte        float64  `json:"time_to_first_byte"`
	DomInteractive         float64  `json:"dom_interactive"`
}

// NetworkSummary holds aggregated network resource metrics
type NetworkSummary struct {
	RequestCount    int                    `json:"request_count"`
	TransferSize    int64                  `json:"transfer_size"`
	DecodedSize     int64                  `json:"decoded_size"`
	ByType          map[string]TypeSummary `json:"by_type"`
	SlowestRequests []SlowRequest          `json:"slowest_requests"`
}

// TypeSummary holds per-type resource metrics
type TypeSummary struct {
	Count int   `json:"count"`
	Size  int64 `json:"size"`
}

// SlowRequest represents one of the slowest network requests
type SlowRequest struct {
	URL      string  `json:"url"`
	Duration float64 `json:"duration"`
	Size     int64   `json:"size"`
}

// LongTaskMetrics holds accumulated long task data
type LongTaskMetrics struct {
	Count             int     `json:"count"`
	TotalBlockingTime float64 `json:"total_blocking_time"`
	Longest           float64 `json:"longest"`
}

// PerformanceBaseline holds averaged performance data for a URL path
type PerformanceBaseline struct {
	URL         string          `json:"url"`
	SampleCount int             `json:"sample_count"`
	LastUpdated string          `json:"last_updated"`
	Timing      BaselineTiming  `json:"timing"`
	Network     BaselineNetwork `json:"network"`
	LongTasks   LongTaskMetrics `json:"long_tasks"`
	CLS         *float64        `json:"cumulative_layout_shift,omitempty"` // snake_case (from browser LayoutShift)
	Resources   []ResourceEntry `json:"resources,omitempty"`
}

// BaselineTiming holds averaged timing metrics
type BaselineTiming struct {
	DomContentLoaded       float64  `json:"dom_content_loaded"`
	Load                   float64  `json:"load"`
	FirstContentfulPaint   *float64 `json:"first_contentful_paint"`
	LargestContentfulPaint *float64 `json:"largest_contentful_paint"`
	TimeToFirstByte        float64  `json:"time_to_first_byte"`
	DomInteractive         float64  `json:"dom_interactive"`
}

// BaselineNetwork holds averaged network metrics
type BaselineNetwork struct {
	RequestCount int   `json:"request_count"`
	TransferSize int64 `json:"transfer_size"`
}

// PerformanceRegression describes a detected performance regression
type PerformanceRegression struct {
	Metric         string  `json:"metric"`
	Current        float64 `json:"current"`
	Baseline       float64 `json:"baseline"`
	ChangePercent  float64 `json:"change_percent"`
	AbsoluteChange float64 `json:"absolute_change"`
}

// ============================================
// Causal Diffing Types
// ============================================

// ResourceEntry represents a single resource in a performance snapshot fingerprint
type ResourceEntry struct {
	URL            string  `json:"url"`
	Type           string  `json:"type"`
	TransferSize   int64   `json:"transfer_size"` // snake_case (from browser PerformanceResourceTiming)
	Duration       float64 `json:"duration"`      // snake_case (from browser PerformanceResourceTiming)
	RenderBlocking bool    `json:"render_blocking,omitempty"`
}

// ResourceDiff holds the categorized differences between baseline and current resources
type ResourceDiff struct {
	Added   []AddedResource   `json:"added"`
	Removed []RemovedResource `json:"removed"`
	Resized []ResizedResource `json:"resized"`
	Retimed []RetimedResource `json:"retimed"`
}

// AddedResource is a resource present in current but not in baseline
type AddedResource struct {
	URL            string  `json:"url"`
	Type           string  `json:"type"`
	SizeBytes      int64   `json:"size_bytes"`
	DurationMs     float64 `json:"duration_ms"`
	RenderBlocking bool    `json:"render_blocking"`
}

// RemovedResource is a resource present in baseline but not in current
type RemovedResource struct {
	URL       string `json:"url"`
	Type      string `json:"type"`
	SizeBytes int64  `json:"size_bytes"`
}

// ResizedResource is a resource present in both with significant size change
type ResizedResource struct {
	URL           string `json:"url"`
	BaselineBytes int64  `json:"baseline_bytes"`
	CurrentBytes  int64  `json:"current_bytes"`
	DeltaBytes    int64  `json:"delta_bytes"`
}

// RetimedResource is a resource present in both with significant duration change
type RetimedResource struct {
	URL        string  `json:"url"`
	BaselineMs float64 `json:"baseline_ms"`
	CurrentMs  float64 `json:"current_ms"`
	DeltaMs    float64 `json:"delta_ms"`
}

// TimingDelta holds the timing differences between baseline and current
type TimingDelta struct {
	LoadMs float64 `json:"load_ms"`
	FCPMs  float64 `json:"fcp_ms"`
	LCPMs  float64 `json:"lcp_ms"`
}

// CausalDiffResult is the full response from the get_causal_diff tool
type CausalDiffResult struct {
	URL             string       `json:"url"`
	TimingDelta     TimingDelta  `json:"timing_delta"`
	ResourceChanges ResourceDiff `json:"resource_changes"`
	ProbableCause   string       `json:"probable_cause"`
	Recommendations []string     `json:"recommendations"`
}

// ============================================
// Constants
// ============================================

const (
	// MaxPerfSnapshots is the maximum number of performance snapshots to retain
	MaxPerfSnapshots = 20

	// MaxPerfBaselines is the maximum number of performance baselines to retain
	MaxPerfBaselines = 20
)
