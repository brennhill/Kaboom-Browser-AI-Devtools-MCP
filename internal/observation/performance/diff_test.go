// diff_test.go — Tests performance metric comparison, ratings, and verdicts.
// Docs: docs/features/feature/performance-audit/index.md

package performance

import (
	"math"
	"strings"
	"testing"
)

// ============================================
// PerfDiff: Before/After Metric Comparison
// ============================================

func TestPerfDiff_BasicImprovement(t *testing.T) {
	t.Parallel()
	fcp900 := 900.0
	fcp800 := 800.0
	lcp2800 := 2800.0
	lcp1200 := 1200.0
	cls02 := 0.02
	cls01 := 0.01

	before := PageLoadMetrics{
		URL:       "https://example.com",
		Timestamp: 1000,
		Timing: MetricsTiming{
			TTFB: 120, FCP: &fcp900, LCP: &lcp2800,
			DomContentLoaded: 800, Load: 1500,
		},
		CLS:          &cls02,
		TransferSize: 768 * 1024,
		RequestCount: 58,
	}
	after := PageLoadMetrics{
		URL:       "https://example.com",
		Timestamp: 2000,
		Timing: MetricsTiming{
			TTFB: 80, FCP: &fcp800, LCP: &lcp1200,
			DomContentLoaded: 700, Load: 1100,
		},
		CLS:          &cls01,
		TransferSize: 512 * 1024,
		RequestCount: 42,
	}

	diff := ComputePerfDiff(before, after)

	// LCP improved 57%
	lcp := diff.Metrics["lcp"]
	if lcp.Before != 2800 {
		t.Errorf("lcp.Before = %v, want 2800", lcp.Before)
	}
	if lcp.After != 1200 {
		t.Errorf("lcp.After = %v, want 1200", lcp.After)
	}
	if lcp.Delta != -1600 {
		t.Errorf("lcp.Delta = %v, want -1600", lcp.Delta)
	}
	if !lcp.Improved {
		t.Error("lcp.Improved should be true (lower is better)")
	}

	// Transfer size decreased
	transfer := diff.Metrics["transfer_kb"]
	if !transfer.Improved {
		t.Error("transfer_kb.Improved should be true")
	}

	// Request count decreased
	requests := diff.Metrics["requests"]
	if requests.Before != 58 || requests.After != 42 {
		t.Errorf("requests = %v→%v, want 58→42", requests.Before, requests.After)
	}

	// Summary must exist and be non-empty
	if diff.Summary == "" {
		t.Error("Summary must not be empty")
	}
}

func TestPerfDiff_Regression(t *testing.T) {
	t.Parallel()
	lcp1200 := 1200.0
	lcp2800 := 2800.0

	before := PageLoadMetrics{
		Timing: MetricsTiming{LCP: &lcp1200, TTFB: 80, Load: 1100},
	}
	after := PageLoadMetrics{
		Timing: MetricsTiming{LCP: &lcp2800, TTFB: 200, Load: 2500},
	}

	diff := ComputePerfDiff(before, after)

	lcp := diff.Metrics["lcp"]
	if lcp.Improved {
		t.Error("lcp.Improved should be false (LCP got worse)")
	}
	if lcp.Delta <= 0 {
		t.Errorf("lcp.Delta = %v, want positive (regression)", lcp.Delta)
	}

	// Summary should flag the regression
	if !strings.Contains(strings.ToLower(diff.Summary), "regress") &&
		!strings.Contains(strings.ToLower(diff.Summary), "worse") &&
		!strings.Contains(strings.ToLower(diff.Summary), "increased") {
		t.Errorf("Summary should flag regression. Got: %q", diff.Summary)
	}
}

func TestPerfDiff_NilLCP(t *testing.T) {
	t.Parallel()
	lcp := 1200.0

	before := PageLoadMetrics{
		Timing: MetricsTiming{LCP: &lcp, TTFB: 80, Load: 1100},
	}
	after := PageLoadMetrics{
		Timing: MetricsTiming{LCP: nil, TTFB: 80, Load: 1100}, // LCP didn't fire
	}

	diff := ComputePerfDiff(before, after)

	// LCP should be absent (not zero, not crash)
	if _, exists := diff.Metrics["lcp"]; exists {
		t.Error("lcp should be omitted when after.LCP is nil")
	}
}

func TestPerfDiff_FirstLoad_NoPrevious(t *testing.T) {
	t.Parallel()
	lcp := 1200.0

	// Empty before (first page load, no baseline)
	before := PageLoadMetrics{}
	after := PageLoadMetrics{
		Timing: MetricsTiming{LCP: &lcp, TTFB: 80, Load: 1100},
	}

	diff := ComputePerfDiff(before, after)

	// Should have metrics with "n/a" pct (no baseline to compare, but after values exist)
	if len(diff.Metrics) != 2 {
		t.Errorf("Expected 2 metrics (ttfb, load), got %d: %v", len(diff.Metrics), diff.Metrics)
	}
	if ttfb, ok := diff.Metrics["ttfb"]; ok {
		if ttfb.Pct != "n/a" {
			t.Errorf("TTFB pct should be 'n/a' when before=0, got %q", ttfb.Pct)
		}
	}
}

func TestComputePerfDiff_TTFBZeroNotSkipped(t *testing.T) {
	t.Parallel()
	before := PageLoadMetrics{
		Timing: MetricsTiming{TTFB: 0, Load: 400},
	}
	after := PageLoadMetrics{
		Timing: MetricsTiming{TTFB: 10, Load: 500},
	}

	diff := ComputePerfDiff(before, after)

	if _, ok := diff.Metrics["ttfb"]; !ok {
		t.Fatal("TTFB metric missing — TTFB=0 should not be skipped")
	}
	if diff.Metrics["ttfb"].Pct != "n/a" {
		t.Errorf("TTFB pct should be 'n/a' when before=0, got %q", diff.Metrics["ttfb"].Pct)
	}
}

func TestComputePerfDiff_BothZeroSkipped(t *testing.T) {
	t.Parallel()
	before := PageLoadMetrics{
		Timing: MetricsTiming{TTFB: 0, Load: 400},
	}
	after := PageLoadMetrics{
		Timing: MetricsTiming{TTFB: 0, Load: 500},
	}

	diff := ComputePerfDiff(before, after)

	if _, ok := diff.Metrics["ttfb"]; ok {
		t.Error("Both-zero TTFB should be skipped")
	}
	if _, ok := diff.Metrics["load"]; !ok {
		t.Error("Load metric should still be present")
	}
}

func TestPerfDiff_PercentageCalculation(t *testing.T) {
	t.Parallel()
	lcp100 := 100.0
	lcp50 := 50.0

	before := PageLoadMetrics{
		Timing: MetricsTiming{LCP: &lcp100, TTFB: 200, Load: 1000},
	}
	after := PageLoadMetrics{
		Timing: MetricsTiming{LCP: &lcp50, TTFB: 100, Load: 500},
	}

	diff := ComputePerfDiff(before, after)

	lcp := diff.Metrics["lcp"]
	// 50→100 is -50%, should show as "-50%"
	if !strings.Contains(lcp.Pct, "-50") {
		t.Errorf("lcp.Pct = %q, want contains '-50'", lcp.Pct)
	}

	ttfb := diff.Metrics["ttfb"]
	if !strings.Contains(ttfb.Pct, "-50") {
		t.Errorf("ttfb.Pct = %q, want contains '-50'", ttfb.Pct)
	}
}

func TestPerfDiff_Verdict_Improved(t *testing.T) {
	t.Parallel()
	lcp2800 := 2800.0
	lcp1200 := 1200.0

	before := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp2800, TTFB: 120, Load: 1500}}
	after := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp1200, TTFB: 80, Load: 1100}}

	diff := ComputePerfDiff(before, after)
	if diff.Verdict != "improved" {
		t.Errorf("Verdict = %q, want 'improved' when all metrics improve", diff.Verdict)
	}
}

func TestPerfDiff_Verdict_Regressed(t *testing.T) {
	t.Parallel()
	lcp1200 := 1200.0
	lcp2800 := 2800.0

	before := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp1200, TTFB: 80, Load: 1100}}
	after := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp2800, TTFB: 200, Load: 2500}}

	diff := ComputePerfDiff(before, after)
	if diff.Verdict != "regressed" {
		t.Errorf("Verdict = %q, want 'regressed' when all metrics get worse", diff.Verdict)
	}
}

func TestPerfDiff_Verdict_Mixed(t *testing.T) {
	t.Parallel()
	lcp2800 := 2800.0
	lcp1200 := 1200.0

	// LCP improves, TTFB regresses
	before := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp2800, TTFB: 80, Load: 1100}}
	after := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp1200, TTFB: 200, Load: 1100}}

	diff := ComputePerfDiff(before, after)
	if diff.Verdict != "mixed" {
		t.Errorf("Verdict = %q, want 'mixed' when some improve and some regress", diff.Verdict)
	}
}

func TestPerfDiff_Verdict_Unchanged(t *testing.T) {
	t.Parallel()
	before := PageLoadMetrics{}
	after := PageLoadMetrics{}

	diff := ComputePerfDiff(before, after)
	if diff.Verdict != "unchanged" {
		t.Errorf("Verdict = %q, want 'unchanged' when no metrics to compare", diff.Verdict)
	}
}

// ============================================
// Rating: Web Vitals thresholds for LLM context
// ============================================

func TestPerfDiff_LCP_Rating_Good(t *testing.T) {
	t.Parallel()
	lcp4000 := 4000.0
	lcp1200 := 1200.0

	before := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp4000, TTFB: 120, Load: 1500}}
	after := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp1200, TTFB: 80, Load: 1100}}

	diff := ComputePerfDiff(before, after)
	lcp := diff.Metrics["lcp"]
	if lcp.Rating != "good" {
		t.Errorf("LCP 1200ms rating = %q, want 'good' (<2500ms)", lcp.Rating)
	}
}

func TestPerfDiff_LCP_Rating_Poor(t *testing.T) {
	t.Parallel()
	lcp1200 := 1200.0
	lcp5000 := 5000.0

	before := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp1200, TTFB: 80, Load: 1100}}
	after := PageLoadMetrics{Timing: MetricsTiming{LCP: &lcp5000, TTFB: 80, Load: 1100}}

	diff := ComputePerfDiff(before, after)
	lcp := diff.Metrics["lcp"]
	if lcp.Rating != "poor" {
		t.Errorf("LCP 5000ms rating = %q, want 'poor' (>4000ms)", lcp.Rating)
	}
}

func TestPerfDiff_CLS_Rating_NeedsImprovement(t *testing.T) {
	t.Parallel()
	cls01 := 0.01
	cls015 := 0.15

	before := PageLoadMetrics{
		CLS:    &cls01,
		Timing: MetricsTiming{TTFB: 80, Load: 1100},
	}
	after := PageLoadMetrics{
		CLS:    &cls015,
		Timing: MetricsTiming{TTFB: 80, Load: 1100},
	}

	diff := ComputePerfDiff(before, after)
	cls := diff.Metrics["cls"]
	if cls.Rating != "needs_improvement" {
		t.Errorf("CLS 0.15 rating = %q, want 'needs_improvement' (0.1-0.25)", cls.Rating)
	}
}

func TestPerfDiff_MetricUnit(t *testing.T) {
	t.Parallel()
	lcp2800 := 2800.0
	lcp1200 := 1200.0
	cls02 := 0.02
	cls01 := 0.01

	before := PageLoadMetrics{
		Timing:       MetricsTiming{LCP: &lcp2800, TTFB: 120, DomContentLoaded: 800, Load: 1500},
		CLS:          &cls02,
		TransferSize: 768 * 1024,
		RequestCount: 58,
	}
	after := PageLoadMetrics{
		Timing:       MetricsTiming{LCP: &lcp1200, TTFB: 80, DomContentLoaded: 700, Load: 1100},
		CLS:          &cls01,
		TransferSize: 512 * 1024,
		RequestCount: 42,
	}

	diff := ComputePerfDiff(before, after)

	checks := map[string]string{
		"lcp":                "ms",
		"ttfb":               "ms",
		"load":               "ms",
		"dom_content_loaded": "ms",
		"transfer_kb":        "KB",
		"requests":           "count",
	}
	for name, wantUnit := range checks {
		md, ok := diff.Metrics[name]
		if !ok {
			t.Errorf("metric %q missing", name)
			continue
		}
		if md.Unit != wantUnit {
			t.Errorf("%s.Unit = %q, want %q", name, md.Unit, wantUnit)
		}
	}
	// CLS is unitless — no unit string
	if diff.Metrics["cls"].Unit != "" {
		t.Errorf("cls.Unit = %q, want empty (unitless)", diff.Metrics["cls"].Unit)
	}
}

// ============================================
func TestPerfDiff_DeltaZeroVerdict(t *testing.T) {
	t.Parallel()
	before := PageLoadMetrics{
		Timing: MetricsTiming{TTFB: 80, Load: 200},
	}
	after := PageLoadMetrics{
		Timing: MetricsTiming{TTFB: 80, Load: 200},
	}

	diff := ComputePerfDiff(before, after)
	if diff.Verdict != "unchanged" {
		t.Errorf("Verdict = %q, want 'unchanged' when all deltas are 0", diff.Verdict)
	}
	// Summary should not claim regression
	if strings.Contains(strings.ToLower(diff.Summary), "regress") {
		t.Errorf("Summary claims regression for identical metrics. Got: %q", diff.Summary)
	}
}

// ============================================
// Types: PageLoadMetrics and PerfDiff structs
// ============================================

// ============================================
// SnapshotToPageLoadMetrics: type mapping
// ============================================

func TestSnapshotToPageLoadMetrics(t *testing.T) {
	t.Parallel()
	fcp := 900.0
	lcp := 2800.0
	cls := 0.15

	snap := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: "2024-01-01T00:00:00Z",
		Timing: PerformanceTiming{
			TimeToFirstByte:        120,
			FirstContentfulPaint:   &fcp,
			LargestContentfulPaint: &lcp,
			DomContentLoaded:       800,
			Load:                   1500,
		},
		Network: NetworkSummary{
			TransferSize: 768 * 1024,
			RequestCount: 58,
		},
		CLS: &cls,
	}

	m := SnapshotToPageLoadMetrics(snap)

	if m.URL != "/dashboard" {
		t.Errorf("URL = %q, want /dashboard", m.URL)
	}
	if m.Timing.TTFB != 120 {
		t.Errorf("TTFB = %v, want 120", m.Timing.TTFB)
	}
	if m.Timing.FCP == nil || *m.Timing.FCP != 900 {
		t.Errorf("FCP = %v, want 900", m.Timing.FCP)
	}
	if m.Timing.LCP == nil || *m.Timing.LCP != 2800 {
		t.Errorf("LCP = %v, want 2800", m.Timing.LCP)
	}
	if m.Timing.DomContentLoaded != 800 {
		t.Errorf("DCL = %v, want 800", m.Timing.DomContentLoaded)
	}
	if m.Timing.Load != 1500 {
		t.Errorf("Load = %v, want 1500", m.Timing.Load)
	}
	if m.CLS == nil || *m.CLS != 0.15 {
		t.Errorf("CLS = %v, want 0.15", m.CLS)
	}
	if m.TransferSize != 768*1024 {
		t.Errorf("TransferSize = %d, want %d", m.TransferSize, 768*1024)
	}
	if m.RequestCount != 58 {
		t.Errorf("RequestCount = %d, want 58", m.RequestCount)
	}
}

func TestSnapshotToPageLoadMetrics_NilOptionals(t *testing.T) {
	t.Parallel()
	snap := PerformanceSnapshot{
		URL: "/page",
		Timing: PerformanceTiming{
			TimeToFirstByte:        100,
			DomContentLoaded:       500,
			Load:                   1000,
			FirstContentfulPaint:   nil,
			LargestContentfulPaint: nil,
		},
		// CLS is nil
	}

	m := SnapshotToPageLoadMetrics(snap)

	if m.Timing.FCP != nil {
		t.Errorf("FCP should be nil, got %v", m.Timing.FCP)
	}
	if m.Timing.LCP != nil {
		t.Errorf("LCP should be nil, got %v", m.Timing.LCP)
	}
	if m.CLS != nil {
		t.Errorf("CLS should be nil, got %v", m.CLS)
	}
}

func TestMetricDiff_Round(t *testing.T) {
	t.Parallel()
	// MetricDiff values should be rounded to avoid floating point noise
	fcp := 123.456789
	before := PageLoadMetrics{
		Timing: MetricsTiming{FCP: &fcp, TTFB: 80.123456, Load: 1000},
	}

	fcp2 := 100.654321
	after := PageLoadMetrics{
		Timing: MetricsTiming{FCP: &fcp2, TTFB: 70.987654, Load: 900},
	}

	diff := ComputePerfDiff(before, after)

	fcp_diff := diff.Metrics["fcp"]
	// Values should be rounded (no more than 1 decimal place for ms values)
	if fcp_diff.Before != math.Round(fcp_diff.Before*10)/10 {
		t.Errorf("fcp.Before not rounded: %v", fcp_diff.Before)
	}
}

func TestPerfDiff_FullWebVitals_AllRatings(t *testing.T) {
	t.Parallel()

	// Construct PageLoadMetrics via SnapshotToPageLoadMetrics to test the real path
	fcp900 := 900.0
	fcp3500 := 3500.0
	lcp2400 := 2400.0
	lcp1200 := 1200.0
	cls02 := 0.02
	cls03 := 0.3

	beforeSnap := PerformanceSnapshot{
		URL: "/page",
		Timing: PerformanceTiming{
			TimeToFirstByte:        900,
			FirstContentfulPaint:   &fcp3500,
			LargestContentfulPaint: &lcp2400,
			DomContentLoaded:       1000,
			Load:                   2000,
		},
		CLS:     &cls03,
		Network: NetworkSummary{TransferSize: 500000, RequestCount: 40},
	}
	afterSnap := PerformanceSnapshot{
		URL: "/page",
		Timing: PerformanceTiming{
			TimeToFirstByte:        200,
			FirstContentfulPaint:   &fcp900,
			LargestContentfulPaint: &lcp1200,
			DomContentLoaded:       600,
			Load:                   1100,
		},
		CLS:     &cls02,
		Network: NetworkSummary{TransferSize: 300000, RequestCount: 30},
	}

	before := SnapshotToPageLoadMetrics(beforeSnap)
	after := SnapshotToPageLoadMetrics(afterSnap)
	diff := ComputePerfDiff(before, after)

	// FCP 900ms → "good" (<1800ms)
	fcp := diff.Metrics["fcp"]
	if fcp.Rating != "good" {
		t.Errorf("FCP 900ms rating = %q, want 'good' (<1800ms)", fcp.Rating)
	}
	if !fcp.Improved {
		t.Error("FCP should be improved (3500→900)")
	}

	// TTFB 200ms → "good" (<800ms)
	ttfb := diff.Metrics["ttfb"]
	if ttfb.Rating != "good" {
		t.Errorf("TTFB 200ms rating = %q, want 'good' (<800ms)", ttfb.Rating)
	}

	// LCP 1200ms → "good" (<2500ms)
	lcp := diff.Metrics["lcp"]
	if lcp.Rating != "good" {
		t.Errorf("LCP 1200ms rating = %q, want 'good' (<2500ms)", lcp.Rating)
	}

	// CLS 0.02 → "good" (<0.1)
	cls := diff.Metrics["cls"]
	if cls.Rating != "good" {
		t.Errorf("CLS 0.02 rating = %q, want 'good' (<0.1)", cls.Rating)
	}

	// Verdict should be "improved" — all metrics got better
	if diff.Verdict != "improved" {
		t.Errorf("Verdict = %q, want 'improved'", diff.Verdict)
	}

	// Summary must mention rating
	if !strings.Contains(diff.Summary, "good") {
		t.Errorf("Summary should mention 'good' rating. Got: %q", diff.Summary)
	}
}

func TestPerfDiff_FCP_NeedsImprovement_Rating(t *testing.T) {
	t.Parallel()
	fcp1000 := 1000.0
	fcp2500 := 2500.0

	before := PageLoadMetrics{Timing: MetricsTiming{FCP: &fcp1000, TTFB: 80, Load: 1000}}
	after := PageLoadMetrics{Timing: MetricsTiming{FCP: &fcp2500, TTFB: 80, Load: 1000}}

	diff := ComputePerfDiff(before, after)

	fcp := diff.Metrics["fcp"]
	if fcp.Rating != "needs_improvement" {
		t.Errorf("FCP 2500ms rating = %q, want 'needs_improvement' (1800-3000ms)", fcp.Rating)
	}
}

func TestPerfDiff_TTFB_Poor_Rating(t *testing.T) {
	t.Parallel()
	before := PageLoadMetrics{Timing: MetricsTiming{TTFB: 100, Load: 1000}}
	after := PageLoadMetrics{Timing: MetricsTiming{TTFB: 2000, Load: 3000}}

	diff := ComputePerfDiff(before, after)

	ttfb := diff.Metrics["ttfb"]
	if ttfb.Rating != "poor" {
		t.Errorf("TTFB 2000ms rating = %q, want 'poor' (>1800ms)", ttfb.Rating)
	}
}
