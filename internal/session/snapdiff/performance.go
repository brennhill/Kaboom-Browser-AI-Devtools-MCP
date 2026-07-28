// Purpose: Computes before/after metric changes for load time, request count, and transfer size with regression detection.
// Docs: docs/features/feature/request-session-correlation/index.md

// performance.go — Performance diff computation.
package snapdiff

import (
	"fmt"
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
	if a.Performance == nil || b.Performance == nil {
		return PerformanceDiff{}
	}
	return PerformanceDiff{
		LoadTime:     computeMetricIfNonZero(a.Performance.Timing.Load, b.Performance.Timing.Load),
		RequestCount: computeMetricIfNonZero(float64(a.Performance.Network.RequestCount), float64(b.Performance.Network.RequestCount)),
		TransferSize: computeMetricIfNonZero(float64(a.Performance.Network.TransferSize), float64(b.Performance.Network.TransferSize)),
	}
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
