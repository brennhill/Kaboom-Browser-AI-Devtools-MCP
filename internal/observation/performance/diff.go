// Purpose: Computes before/after performance metric diffs and Web Vitals ratings.
// Docs: docs/features/feature/performance-audit/index.md

// diff.go — Rich Action Results: performance diff computation.
// Computes before/after metric diffs and emits AI-consumable summaries.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
package performance

import (
	"fmt"
	"math"
)

// ============================================
// Types for diff computation
// ============================================

// PageLoadMetrics is a simplified snapshot used for before/after comparison.
type PageLoadMetrics struct {
	URL          string        `json:"url"`
	Timestamp    int64         `json:"timestamp"`
	Timing       MetricsTiming `json:"timing"`
	CLS          *float64      `json:"cls,omitempty"`
	TransferSize int64         `json:"transfer_size"`
	RequestCount int           `json:"request_count"`
}

// MetricsTiming holds the core Web Vitals timing values.
type MetricsTiming struct {
	TTFB             float64  `json:"ttfb"`
	FCP              *float64 `json:"fcp,omitempty"`
	LCP              *float64 `json:"lcp,omitempty"`
	DomContentLoaded float64  `json:"dom_content_loaded"`
	Load             float64  `json:"load"`
}

// MetricDiff holds the before/after comparison for a single metric.
type MetricDiff struct {
	Before   float64 `json:"before"`
	After    float64 `json:"after"`
	Delta    float64 `json:"delta"`
	Pct      string  `json:"pct"`
	Unit     string  `json:"unit,omitempty"` // "ms", "KB", "count" — empty for unitless (CLS)
	Improved bool    `json:"improved"`
	Rating   string  `json:"rating,omitempty"` // "good", "needs_improvement", "poor" per Web Vitals thresholds
}

// PerfDiff holds the complete diff result with all metrics and resources.
type PerfDiff struct {
	Verdict   string                `json:"verdict"` // "improved", "regressed", "mixed", "unchanged"
	Metrics   map[string]MetricDiff `json:"metrics"`
	Resources ResourceDiff          `json:"resources"`
	Summary   string                `json:"summary"`
}

// ============================================
// SnapshotToPageLoadMetrics: type mapping
// ============================================

// SnapshotToPageLoadMetrics maps a PerformanceSnapshot (from the extension)
// to a PageLoadMetrics (used by ComputePerfDiff).
func SnapshotToPageLoadMetrics(snapshot PerformanceSnapshot) PageLoadMetrics {
	metrics := PageLoadMetrics{
		URL:          snapshot.URL,
		TransferSize: snapshot.Network.TransferSize,
		RequestCount: snapshot.Network.RequestCount,
		Timing: MetricsTiming{
			TTFB:             snapshot.Timing.TimeToFirstByte,
			DomContentLoaded: snapshot.Timing.DomContentLoaded,
			Load:             snapshot.Timing.Load,
		},
	}
	if snapshot.Timing.FirstContentfulPaint != nil {
		v := *snapshot.Timing.FirstContentfulPaint
		metrics.Timing.FCP = &v
	}
	if snapshot.Timing.LargestContentfulPaint != nil {
		v := *snapshot.Timing.LargestContentfulPaint
		metrics.Timing.LCP = &v
	}
	if snapshot.CLS != nil {
		v := *snapshot.CLS
		metrics.CLS = &v
	}
	return metrics
}

// ============================================
// ComputePerfDiff: before/after metric comparison
// ============================================

// buildMetricDiff computes a single metric diff if at least one value is non-zero.
// Returns the diff and true if the metric is meaningful, false otherwise.
func buildMetricDiff(name string, beforeVal, afterVal float64) (MetricDiff, bool) {
	if beforeVal == 0 && afterVal == 0 {
		return MetricDiff{}, false
	}
	var delta float64
	var pctStr string
	if beforeVal == 0 {
		delta = round1(afterVal)
		pctStr = "n/a"
	} else {
		delta = round1(afterVal - beforeVal)
		pctStr = formatPct((afterVal - beforeVal) / beforeVal * 100)
	}
	return MetricDiff{
		Before:   round1(beforeVal),
		After:    round1(afterVal),
		Delta:    delta,
		Pct:      pctStr,
		Unit:     unitForMetric(name),
		Improved: delta < 0,
		Rating:   rateMetric(name, round1(afterVal)),
	}, true
}

// addMetricIfValid adds a metric diff to the map if both values are meaningful.
func addMetricIfValid(metrics map[string]MetricDiff, name string, beforeVal, afterVal float64) {
	if md, ok := buildMetricDiff(name, beforeVal, afterVal); ok {
		metrics[name] = md
	}
}

// ptrPairValues returns the dereferenced values of two optional float64 pointers.
// Returns 0, 0, false if either is nil.
func ptrPairValues(a, b *float64) (float64, float64, bool) {
	if a == nil || b == nil {
		return 0, 0, false
	}
	return *a, *b, true
}

// computeVerdict derives the overall verdict from metric deltas.
func computeVerdict(metrics map[string]MetricDiff) string {
	if len(metrics) == 0 {
		return "unchanged"
	}
	improved, regressed := 0, 0
	for _, md := range metrics {
		if md.Delta < 0 {
			improved++
		} else if md.Delta > 0 {
			regressed++
		}
	}
	switch {
	case improved > 0 && regressed == 0:
		return "improved"
	case regressed > 0 && improved == 0:
		return "regressed"
	case improved > 0 && regressed > 0:
		return "mixed"
	default:
		return "unchanged"
	}
}

// ComputePerfDiff compares two page load snapshots and returns per-metric diffs.
// Metrics with no baseline (before is zero/nil) are omitted.
// Metrics where after is nil (didn't fire) are omitted.
func ComputePerfDiff(before, after PageLoadMetrics) PerfDiff {
	diff := PerfDiff{Metrics: make(map[string]MetricDiff)}

	addMetricIfValid(diff.Metrics, "ttfb", before.Timing.TTFB, after.Timing.TTFB)
	if bv, av, ok := ptrPairValues(before.Timing.FCP, after.Timing.FCP); ok {
		addMetricIfValid(diff.Metrics, "fcp", bv, av)
	}
	if bv, av, ok := ptrPairValues(before.Timing.LCP, after.Timing.LCP); ok {
		addMetricIfValid(diff.Metrics, "lcp", bv, av)
	}
	addMetricIfValid(diff.Metrics, "dom_content_loaded", before.Timing.DomContentLoaded, after.Timing.DomContentLoaded)
	addMetricIfValid(diff.Metrics, "load", before.Timing.Load, after.Timing.Load)
	if bv, av, ok := ptrPairValues(before.CLS, after.CLS); ok {
		addMetricIfValid(diff.Metrics, "cls", bv, av)
	}
	if before.TransferSize > 0 {
		addMetricIfValid(diff.Metrics, "transfer_kb", float64(before.TransferSize)/1024, float64(after.TransferSize)/1024)
	}
	if before.RequestCount > 0 {
		addMetricIfValid(diff.Metrics, "requests", float64(before.RequestCount), float64(after.RequestCount))
	}

	diff.Verdict = computeVerdict(diff.Metrics)
	diff.Summary = GeneratePerfSummary(diff)
	return diff
}

// ============================================
// Helpers
// ============================================

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func formatPct(pct float64) string {
	rounded := math.Round(pct)
	if rounded >= 0 {
		return fmt.Sprintf("+%.0f%%", rounded)
	}
	return fmt.Sprintf("%.0f%%", rounded)
}

// unitForMetric returns the unit string for a given metric name.
func unitForMetric(name string) string {
	switch name {
	case "ttfb", "fcp", "lcp", "dom_content_loaded", "load":
		return "ms"
	case "transfer_kb":
		return "KB"
	case "requests":
		return "count"
	default:
		return "" // CLS is unitless
	}
}

// metricThreshold defines Web Vitals thresholds for a metric.
type metricThreshold struct {
	good float64 // values below this are "good"
	ni   float64 // values at or below this are "needs_improvement"; above is "poor"
}

// webVitalsThresholds maps metric names to their Web Vitals thresholds.
var webVitalsThresholds = map[string]metricThreshold{
	"lcp":  {good: 2500, ni: 4000},
	"fcp":  {good: 1800, ni: 3000},
	"ttfb": {good: 800, ni: 1800},
	"cls":  {good: 0.1, ni: 0.25},
}

// rateMetric returns a Web Vitals rating for the given metric's current value.
// Returns "" for metrics without standard thresholds.
func rateMetric(name string, value float64) string {
	threshold, ok := webVitalsThresholds[name]
	if !ok {
		return ""
	}
	if value < threshold.good {
		return "good"
	}
	if value <= threshold.ni {
		return "needs_improvement"
	}
	return "poor"
}
