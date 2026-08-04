// Purpose: Computes before/after metric changes for load time, request count, and transfer size with regression detection.
// Docs: docs/features/feature/request-session-correlation/index.md

// performance.go — Performance diff computation.
package snapdiff

import (
	"fmt"
	"math"
	"sort"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// computeMetricIfNonZero returns a MetricChange if at least one value is non-zero, else nil.
func computeMetricIfNonZero(before, after float64) *MetricChange {
	if before == 0 && after == 0 {
		return nil
	}
	return computeMetricChange(before, after)
}

// Performance compares performance metrics between two snapshots.
func Performance(a, b *types.NamedSnapshot) PerformanceDiff {
	return PerformanceWithBudgets(a, b, nil)
}

func PerformanceWithBudgets(a, b *types.NamedSnapshot, budgets map[string]float64) PerformanceDiff {
	if a.Performance == nil || b.Performance == nil {
		return PerformanceDiff{}
	}
	diff := PerformanceDiff{
		LoadTime:      computeMetricIfNonZero(a.Performance.Timing.Load, b.Performance.Timing.Load),
		FCP:           optionalMetric(a.Performance.Timing.FirstContentfulPaint, b.Performance.Timing.FirstContentfulPaint),
		LCP:           optionalMetric(a.Performance.Timing.LargestContentfulPaint, b.Performance.Timing.LargestContentfulPaint),
		INP:           optionalMetric(a.Performance.Timing.InteractionToNextPaint, b.Performance.Timing.InteractionToNextPaint),
		CLS:           optionalMetric(a.Performance.CLS, b.Performance.CLS),
		RequestCount:  computeMetricIfNonZero(float64(a.Performance.Network.RequestCount), float64(b.Performance.Network.RequestCount)),
		TransferSize:  computeMetricIfNonZero(float64(a.Performance.Network.TransferSize), float64(b.Performance.Network.TransferSize)),
		ExecutionCost: computeMetricIfNonZero(a.Performance.LongTasks.TotalBlockingTime, b.Performance.LongTasks.TotalBlockingTime),
		Statistics:    buildStatistics(samplesFor(a), samplesFor(b)),
	}
	beforeSegment, beforeMs := dominantSegment(*a.Performance)
	afterSegment, afterMs := dominantSegment(*b.Performance)
	if beforeSegment != "" || afterSegment != "" {
		diff.CriticalPath = &CriticalPathChange{BeforeSegment: beforeSegment, BeforeMs: beforeMs, AfterSegment: afterSegment, AfterMs: afterMs}
	}
	diff.Budgets = evaluateBudgets(diff, budgets)
	return diff
}

func optionalMetric(before, after *float64) *MetricChange {
	if before == nil || after == nil {
		return nil
	}
	return computeMetricIfNonZero(*before, *after)
}

func samplesFor(snapshot *types.NamedSnapshot) []performanceSample {
	samples := snapshot.PerformanceSamples
	if len(samples) == 0 && snapshot.Performance != nil {
		samples = []performance.PerformanceSnapshot{*snapshot.Performance}
	}
	result := make([]performanceSample, 0, len(samples))
	for _, sample := range samples {
		result = append(result, performanceSample{
			load: sample.Timing.Load, fcp: sample.Timing.FirstContentfulPaint, lcp: sample.Timing.LargestContentfulPaint,
			inp: sample.Timing.InteractionToNextPaint, cls: sample.CLS, transfer: float64(sample.Network.TransferSize),
			execution: sample.LongTasks.TotalBlockingTime,
		})
	}
	return result
}

type performanceSample struct {
	load, transfer, execution float64
	fcp, lcp, inp, cls        *float64
}

func buildStatistics(before, after []performanceSample) *PerformanceStatistics {
	result := &PerformanceStatistics{
		Status: "sufficient_samples", Before: snapshotStatistics(before), After: snapshotStatistics(after),
	}
	if len(before) < 3 || len(after) < 3 {
		result.Status = "insufficient_samples"
	}
	return result
}

func snapshotStatistics(samples []performanceSample) SnapshotStatistics {
	return SnapshotStatistics{
		Load:          distribution(samples, func(sample performanceSample) (float64, bool) { return sample.load, true }),
		FCP:           distribution(samples, func(sample performanceSample) (float64, bool) { return optionalValue(sample.fcp) }),
		LCP:           distribution(samples, func(sample performanceSample) (float64, bool) { return optionalValue(sample.lcp) }),
		INP:           distribution(samples, func(sample performanceSample) (float64, bool) { return optionalValue(sample.inp) }),
		CLS:           distribution(samples, func(sample performanceSample) (float64, bool) { return optionalValue(sample.cls) }),
		TransferSize:  distribution(samples, func(sample performanceSample) (float64, bool) { return sample.transfer, true }),
		ExecutionCost: distribution(samples, func(sample performanceSample) (float64, bool) { return sample.execution, true }),
	}
}

func distribution(samples []performanceSample, project func(performanceSample) (float64, bool)) Distribution {
	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		if candidate, available := project(sample); available {
			values = append(values, candidate)
		}
	}
	if len(values) == 0 {
		return Distribution{}
	}
	sort.Float64s(values)
	return Distribution{SampleCount: len(values), Median: percentile(values, 0.5), P75: percentile(values, 0.75)}
}

func optionalValue(pointer *float64) (float64, bool) {
	if pointer == nil {
		return 0, false
	}
	return *pointer, true
}

func percentile(values []float64, quantile float64) float64 {
	if len(values) == 1 {
		return values[0]
	}
	position := quantile * float64(len(values)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return values[lower]
	}
	return values[lower] + (values[upper]-values[lower])*(position-float64(lower))
}

func dominantSegment(snapshot performance.PerformanceSnapshot) (string, float64) {
	type segment struct {
		name     string
		duration float64
	}
	segments := []segment{
		{name: "navigation_ttfb", duration: snapshot.Timing.TimeToFirstByte},
		{name: "first_contentful_paint", duration: value(snapshot.Timing.FirstContentfulPaint)},
		{name: "execution_cost", duration: snapshot.LongTasks.TotalBlockingTime},
	}
	if snapshot.Timing.LargestContentfulPaint != nil {
		duration := *snapshot.Timing.LargestContentfulPaint - value(snapshot.Timing.FirstContentfulPaint)
		if duration < 0 {
			duration = 0
		}
		segments = append(segments, segment{name: "largest_contentful_paint", duration: duration})
	}
	backendDuration := 0.0
	for _, request := range snapshot.Network.SlowestRequests {
		if request.Duration > backendDuration {
			backendDuration = request.Duration
		}
	}
	segments = append(segments, segment{name: "backend_response", duration: backendDuration})
	name, duration := "", 0.0
	for _, candidate := range segments {
		if candidate.duration > duration {
			name, duration = candidate.name, candidate.duration
		}
	}
	return name, duration
}

func evaluateBudgets(diff PerformanceDiff, budgets map[string]float64) map[string]BudgetVerdict {
	if len(budgets) == 0 {
		return nil
	}
	metrics := map[string]*MetricChange{
		"load": diff.LoadTime, "fcp": diff.FCP, "lcp": diff.LCP, "inp": diff.INP, "cls": diff.CLS,
		"request_count": diff.RequestCount, "transfer_size": diff.TransferSize, "execution_cost": diff.ExecutionCost,
	}
	verdicts := make(map[string]BudgetVerdict, len(budgets))
	for name, allowed := range budgets {
		metric := metrics[name]
		verdict := BudgetVerdict{AllowedRegression: allowed, Status: "unavailable"}
		if metric != nil {
			verdict.ActualRegression = metric.After - metric.Before
			verdict.Status = "pass"
			if verdict.ActualRegression > allowed {
				verdict.Status = "fail"
			}
		}
		verdicts[name] = verdict
	}
	return verdicts
}

func value(pointer *float64) float64 {
	if pointer == nil {
		return 0
	}
	return *pointer
}

// formatPctChange formats a percentage change as a signed string.
func formatPctChange(pctChange float64) string {
	if pctChange >= 0 {
		return fmt.Sprintf("+%.0f%%", pctChange)
	}
	return fmt.Sprintf("%.0f%%", pctChange)
}

// computeMetricChange creates a MetricChange comparing two values.
func computeMetricChange(before, after float64) *MetricChange {
	mc := &MetricChange{Before: before, After: after}

	if before == 0 {
		mc.Change = "0%"
		if after > 0 {
			mc.Change = "+inf"
			mc.Regression = true
		}
		return mc
	}

	mc.Change = formatPctChange(((after - before) / before) * 100)
	mc.Regression = after > before*perfRegressionRatio
	return mc
}
