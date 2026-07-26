// Purpose: Tests for session performance diff computation.
// Docs: docs/features/feature/request-session-correlation/index.md

// performance_test.go — Tests for performance.go.
// Covers: computeMetricIfNonZero, computeMetricChange, formatPctChange, Performance.
package snapdiff

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

// ============================================
// computeMetricIfNonZero
// ============================================

func TestComputeMetricIfNonZero_BothZero(t *testing.T) {
	t.Parallel()
	result := computeMetricIfNonZero(0, 0)
	if result != nil {
		t.Errorf("Expected nil when both zero, got %+v", result)
	}
}

func TestComputeMetricIfNonZero_BeforeZero(t *testing.T) {
	t.Parallel()
	result := computeMetricIfNonZero(0, 100)
	if result == nil {
		t.Fatal("Expected non-nil when after is non-zero")
	}
	if result.Before != 0 {
		t.Errorf("Expected before=0, got %v", result.Before)
	}
	if result.After != 100 {
		t.Errorf("Expected after=100, got %v", result.After)
	}
	if result.Change != "+inf" {
		t.Errorf("Expected change='+inf', got %q", result.Change)
	}
	if !result.Regression {
		t.Error("Expected regression=true for 0->100")
	}
}

func TestComputeMetricIfNonZero_AfterZero(t *testing.T) {
	t.Parallel()
	result := computeMetricIfNonZero(100, 0)
	if result == nil {
		t.Fatal("Expected non-nil when before is non-zero")
	}
	if result.Before != 100 {
		t.Errorf("Expected before=100, got %v", result.Before)
	}
	if result.After != 0 {
		t.Errorf("Expected after=0, got %v", result.After)
	}
}

func TestComputeMetricIfNonZero_BothNonZero(t *testing.T) {
	t.Parallel()
	result := computeMetricIfNonZero(100, 200)
	if result == nil {
		t.Fatal("Expected non-nil when both non-zero")
	}
	if result.Before != 100 || result.After != 200 {
		t.Errorf("Expected before=100, after=200, got %v, %v", result.Before, result.After)
	}
}

// ============================================
// computeMetricChange
// ============================================

func TestComputeMetricChange_Increase(t *testing.T) {
	t.Parallel()
	mc := computeMetricChange(100, 200)
	if mc.Before != 100 {
		t.Errorf("Expected before=100, got %v", mc.Before)
	}
	if mc.After != 200 {
		t.Errorf("Expected after=200, got %v", mc.After)
	}
	if mc.Change != "+100%" {
		t.Errorf("Expected change='+100%%', got %q", mc.Change)
	}
	// 200 > 100*1.5 = 150, so regression
	if !mc.Regression {
		t.Error("Expected regression=true for 100->200 (>50% increase)")
	}
}

func TestComputeMetricChange_Decrease(t *testing.T) {
	t.Parallel()
	mc := computeMetricChange(200, 100)
	if mc.Change != "-50%" {
		t.Errorf("Expected change='-50%%', got %q", mc.Change)
	}
	// 100 is not > 200*1.5 = 300, so no regression
	if mc.Regression {
		t.Error("Expected regression=false for decrease")
	}
}

func TestComputeMetricChange_NoChange(t *testing.T) {
	t.Parallel()
	mc := computeMetricChange(100, 100)
	if mc.Change != "+0%" {
		t.Errorf("Expected change='+0%%', got %q", mc.Change)
	}
	if mc.Regression {
		t.Error("Expected regression=false for no change")
	}
}

func TestComputeMetricChange_SmallIncrease_NoRegression(t *testing.T) {
	t.Parallel()
	// 100 -> 140 = 40% increase, but 140 <= 100*1.5, so not regression
	mc := computeMetricChange(100, 140)
	if mc.Change != "+40%" {
		t.Errorf("Expected change='+40%%', got %q", mc.Change)
	}
	if mc.Regression {
		t.Error("Expected regression=false for <50% increase")
	}
}

func TestComputeMetricChange_ExactThreshold_NoRegression(t *testing.T) {
	t.Parallel()
	// 100 -> 150 = exactly 50%, but 150 is NOT > 150, so no regression
	mc := computeMetricChange(100, 150)
	if mc.Change != "+50%" {
		t.Errorf("Expected change='+50%%', got %q", mc.Change)
	}
	if mc.Regression {
		t.Error("Expected regression=false at exact threshold (not strictly greater)")
	}
}

func TestComputeMetricChange_JustOverThreshold_Regression(t *testing.T) {
	t.Parallel()
	// 100 -> 151: 151 > 100*1.5 = 150
	mc := computeMetricChange(100, 151)
	if !mc.Regression {
		t.Error("Expected regression=true for 100->151")
	}
}

func TestComputeMetricChange_BeforeZero(t *testing.T) {
	t.Parallel()
	mc := computeMetricChange(0, 100)
	if mc.Change != "+inf" {
		t.Errorf("Expected change='+inf', got %q", mc.Change)
	}
	if !mc.Regression {
		t.Error("Expected regression=true for 0->positive")
	}
}

func TestComputeMetricChange_BeforeZeroAfterZero(t *testing.T) {
	t.Parallel()
	mc := computeMetricChange(0, 0)
	if mc.Change != "0%" {
		t.Errorf("Expected change='0%%', got %q", mc.Change)
	}
	if mc.Regression {
		t.Error("Expected regression=false for 0->0")
	}
}

// ============================================
// formatPctChange
// ============================================

func TestFormatPctChange_Positive(t *testing.T) {
	t.Parallel()
	result := formatPctChange(42.3)
	if result != "+42%" {
		t.Errorf("Expected '+42%%', got %q", result)
	}
}

func TestFormatPctChange_Negative(t *testing.T) {
	t.Parallel()
	result := formatPctChange(-25.7)
	if result != "-26%" {
		t.Errorf("Expected '-26%%', got %q", result)
	}
}

func TestFormatPctChange_Zero(t *testing.T) {
	t.Parallel()
	result := formatPctChange(0)
	if result != "+0%" {
		t.Errorf("Expected '+0%%', got %q", result)
	}
}

func TestFormatPctChange_LargePositive(t *testing.T) {
	t.Parallel()
	result := formatPctChange(500.0)
	if result != "+500%" {
		t.Errorf("Expected '+500%%', got %q", result)
	}
}

// ============================================
// diffPerformance
// ============================================

func TestDiffPerformance_BothNilPerformance(t *testing.T) {
	t.Parallel()
	snapA := &NamedSnapshot{Performance: nil}
	snapB := &NamedSnapshot{Performance: nil}

	diff := Performance(snapA, snapB)
	if diff.LoadTime != nil {
		t.Error("Expected nil LoadTime when both nil")
	}
	if diff.RequestCount != nil {
		t.Error("Expected nil RequestCount when both nil")
	}
	if diff.TransferSize != nil {
		t.Error("Expected nil TransferSize when both nil")
	}
}

func TestDiffPerformance_AIsNil(t *testing.T) {
	t.Parallel()
	snapA := &NamedSnapshot{Performance: nil}
	snapB := &NamedSnapshot{
		Performance: &performance.Snapshot{
			Timing:  performance.Timing{Load: 1000},
			Network: performance.NetworkSummary{RequestCount: 10, TransferSize: 50000},
		},
	}

	diff := Performance(snapA, snapB)
	if diff.LoadTime != nil {
		t.Error("Expected nil LoadTime when A is nil")
	}
}

func TestDiffPerformance_BIsNil(t *testing.T) {
	t.Parallel()
	snapA := &NamedSnapshot{
		Performance: &performance.Snapshot{
			Timing:  performance.Timing{Load: 1000},
			Network: performance.NetworkSummary{RequestCount: 10, TransferSize: 50000},
		},
	}
	snapB := &NamedSnapshot{Performance: nil}

	diff := Performance(snapA, snapB)
	if diff.LoadTime != nil {
		t.Error("Expected nil LoadTime when B is nil")
	}
}

func TestDiffPerformance_AllMetrics(t *testing.T) {
	t.Parallel()
	snapA := &NamedSnapshot{
		Performance: &performance.Snapshot{
			Timing:  performance.Timing{Load: 1000},
			Network: performance.NetworkSummary{RequestCount: 10, TransferSize: 50000},
		},
	}
	snapB := &NamedSnapshot{
		Performance: &performance.Snapshot{
			Timing:  performance.Timing{Load: 2000},
			Network: performance.NetworkSummary{RequestCount: 20, TransferSize: 100000},
		},
	}

	diff := Performance(snapA, snapB)

	// LoadTime: 1000 -> 2000
	if diff.LoadTime == nil {
		t.Fatal("Expected non-nil LoadTime")
	}
	if diff.LoadTime.Before != 1000 {
		t.Errorf("Expected LoadTime.Before=1000, got %v", diff.LoadTime.Before)
	}
	if diff.LoadTime.After != 2000 {
		t.Errorf("Expected LoadTime.After=2000, got %v", diff.LoadTime.After)
	}
	if diff.LoadTime.Change != "+100%" {
		t.Errorf("Expected LoadTime.Change='+100%%', got %q", diff.LoadTime.Change)
	}
	if !diff.LoadTime.Regression {
		t.Error("Expected LoadTime.Regression=true")
	}

	// RequestCount: 10 -> 20
	if diff.RequestCount == nil {
		t.Fatal("Expected non-nil RequestCount")
	}
	if diff.RequestCount.Before != 10 {
		t.Errorf("Expected RequestCount.Before=10, got %v", diff.RequestCount.Before)
	}
	if diff.RequestCount.After != 20 {
		t.Errorf("Expected RequestCount.After=20, got %v", diff.RequestCount.After)
	}
	if !diff.RequestCount.Regression {
		t.Error("Expected RequestCount.Regression=true")
	}

	// TransferSize: 50000 -> 100000
	if diff.TransferSize == nil {
		t.Fatal("Expected non-nil TransferSize")
	}
	if diff.TransferSize.Before != 50000 {
		t.Errorf("Expected TransferSize.Before=50000, got %v", diff.TransferSize.Before)
	}
	if diff.TransferSize.After != 100000 {
		t.Errorf("Expected TransferSize.After=100000, got %v", diff.TransferSize.After)
	}
	if !diff.TransferSize.Regression {
		t.Error("Expected TransferSize.Regression=true")
	}
}

func TestDiffPerformance_ZeroMetrics(t *testing.T) {
	t.Parallel()
	snapA := &NamedSnapshot{
		Performance: &performance.Snapshot{
			Timing:  performance.Timing{Load: 0},
			Network: performance.NetworkSummary{RequestCount: 0, TransferSize: 0},
		},
	}
	snapB := &NamedSnapshot{
		Performance: &performance.Snapshot{
			Timing:  performance.Timing{Load: 0},
			Network: performance.NetworkSummary{RequestCount: 0, TransferSize: 0},
		},
	}

	diff := Performance(snapA, snapB)

	// Both zero => computeMetricIfNonZero returns nil
	if diff.LoadTime != nil {
		t.Error("Expected nil LoadTime when both zero")
	}
	if diff.RequestCount != nil {
		t.Error("Expected nil RequestCount when both zero")
	}
	if diff.TransferSize != nil {
		t.Error("Expected nil TransferSize when both zero")
	}
}

func TestDiffPerformance_Improvement(t *testing.T) {
	t.Parallel()
	snapA := &NamedSnapshot{
		Performance: &performance.Snapshot{
			Timing:  performance.Timing{Load: 3000},
			Network: performance.NetworkSummary{RequestCount: 50, TransferSize: 500000},
		},
	}
	snapB := &NamedSnapshot{
		Performance: &performance.Snapshot{
			Timing:  performance.Timing{Load: 800},
			Network: performance.NetworkSummary{RequestCount: 10, TransferSize: 50000},
		},
	}

	diff := Performance(snapA, snapB)

	if diff.LoadTime == nil {
		t.Fatal("Expected non-nil LoadTime")
	}
	if diff.LoadTime.Regression {
		t.Error("Expected no regression when load time decreased")
	}
	if diff.RequestCount.Regression {
		t.Error("Expected no regression when request count decreased")
	}
	if diff.TransferSize.Regression {
		t.Error("Expected no regression when transfer size decreased")
	}
}
