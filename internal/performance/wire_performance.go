// Purpose: Defines wire types for performance data received from the extension over HTTP (must mirror wire-performance-snapshot.ts).
// Docs: docs/features/feature/performance-audit/index.md

// wire_performance.go — Wire types for performance snapshots over HTTP.
// Defines the JSON fields sent by the extension for performance data.
// Changes here MUST be mirrored in src/types/wire-performance-snapshot.ts.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
package performance

// WirePerformanceTiming holds navigation timing metrics from the extension.
type WirePerformanceTiming struct {
	DomContentLoaded       float64  `json:"dom_content_loaded"`
	Load                   float64  `json:"load"`
	FirstContentfulPaint   *float64 `json:"first_contentful_paint"`
	LargestContentfulPaint *float64 `json:"largest_contentful_paint"`
	InteractionToNextPaint *float64 `json:"interaction_to_next_paint,omitempty"`
	TimeToFirstByte        float64  `json:"time_to_first_byte"`
	DomInteractive         float64  `json:"dom_interactive"`
}

// WireTypeSummary holds per-type resource metrics.
type WireTypeSummary struct {
	Count int   `json:"count"`
	Size  int64 `json:"size"`
}

// WireSlowRequest represents one of the slowest network requests.
type WireSlowRequest struct {
	URL      string  `json:"url"`
	Duration float64 `json:"duration"`
	Size     int64   `json:"size"`
}

// WireNetworkSummary holds aggregated network resource metrics from the extension.
type WireNetworkSummary struct {
	RequestCount    int                        `json:"request_count"`
	TransferSize    int64                      `json:"transfer_size"`
	DecodedSize     int64                      `json:"decoded_size"`
	ByType          map[string]WireTypeSummary `json:"by_type"`
	SlowestRequests []WireSlowRequest          `json:"slowest_requests"`
}

// WireLongTaskMetrics holds accumulated long task data from the extension.
type WireLongTaskMetrics struct {
	Count             int     `json:"count"`
	TotalBlockingTime float64 `json:"total_blocking_time"`
	Longest           float64 `json:"longest"`
}

// WireElementDescriptor identifies an element without capturing its text or attributes.
type WireElementDescriptor struct {
	Tag     string   `json:"tag"`
	ID      string   `json:"id,omitempty"`
	Classes []string `json:"classes,omitempty"`
	Role    string   `json:"role,omitempty"`
}

type WireLCPAttribution struct {
	Element                WireElementDescriptor `json:"element,omitempty"`
	TimeToFirstByteMs      float64               `json:"time_to_first_byte_ms"`
	ResourceLoadDelayMs    *float64              `json:"resource_load_delay_ms,omitempty"`
	ResourceLoadDurationMs *float64              `json:"resource_load_duration_ms,omitempty"`
	ElementRenderDelayMs   float64               `json:"element_render_delay_ms"`
	AttributionStatus      string                `json:"attribution_status"`
	ResourceTimingStatus   string                `json:"resource_timing_status"`
}

type WireINPAttribution struct {
	EventType           string                `json:"event_type"`
	Target              WireElementDescriptor `json:"target,omitempty"`
	InputDelayMs        float64               `json:"input_delay_ms"`
	ProcessingMs        float64               `json:"processing_ms"`
	PresentationDelayMs float64               `json:"presentation_delay_ms"`
	InteractionID       float64               `json:"interaction_id"`
}

type WireLayoutShiftAttribution struct {
	Value     float64                 `json:"value"`
	StartTime float64                 `json:"start_time"`
	Nodes     []WireElementDescriptor `json:"nodes"`
}

type WireCLSAttribution struct {
	Shifts            []WireLayoutShiftAttribution `json:"shifts"`
	AttributionStatus string                       `json:"attribution_status"`
}

type WireLongTaskAttribution struct {
	Name              string   `json:"name"`
	StartTime         float64  `json:"start_time"`
	Duration          float64  `json:"duration"`
	SourceStack       []string `json:"source_stack,omitempty"`
	SourceStackStatus string   `json:"source_stack_status"`
}

type WireVitalsAttribution struct {
	LCP       *WireLCPAttribution       `json:"lcp,omitempty"`
	INP       *WireINPAttribution       `json:"inp,omitempty"`
	CLS       WireCLSAttribution        `json:"cls"`
	LongTasks []WireLongTaskAttribution `json:"long_tasks"`
}

// WireUserTimingEntry represents a single performance mark or measure.
type WireUserTimingEntry struct {
	Name      string  `json:"name"`
	StartTime float64 `json:"start_time"`
	Duration  float64 `json:"duration,omitempty"`
}

// WireUserTimingData holds captured performance.mark() and performance.measure() entries.
type WireUserTimingData struct {
	Marks    []WireUserTimingEntry `json:"marks"`
	Measures []WireUserTimingEntry `json:"measures"`
}

// WirePerformanceSnapshot is the canonical wire format for performance data.
type WirePerformanceSnapshot struct {
	URL               string                 `json:"url"`
	Timestamp         string                 `json:"timestamp"`
	Timing            WirePerformanceTiming  `json:"timing"`
	Network           WireNetworkSummary     `json:"network"`
	LongTasks         WireLongTaskMetrics    `json:"long_tasks"`
	CLS               *float64               `json:"cumulative_layout_shift,omitempty"`
	VitalsAttribution *WireVitalsAttribution `json:"vitals_attribution,omitempty"`
	UserTiming        *WireUserTimingData    `json:"user_timing,omitempty"`
}
