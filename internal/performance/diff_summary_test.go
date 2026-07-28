// diff_summary_test.go — Tests performance diff summary wording and ordering.
// Docs: docs/features/feature/performance-audit/index.md

package performance

import (
	"strings"
	"testing"
)

// ============================================
// Summary Generation
// ============================================

func TestSummary_LeadsWithBiggestImprovement(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"lcp":  {Before: 2800, After: 1200, Delta: -1600, Pct: "-57%", Improved: true},
			"ttfb": {Before: 120, After: 110, Delta: -10, Pct: "-8%", Improved: true},
		},
	}

	summary := GeneratePerfSummary(diff)

	// Should lead with LCP (biggest improvement)
	if !strings.HasPrefix(strings.ToUpper(summary), "LCP") {
		t.Errorf("Summary should lead with biggest improvement (LCP). Got: %q", summary)
	}
}

func TestSummary_MentionsResourceChanges(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"transfer_kb": {Before: 768, After: 512, Delta: -256, Pct: "-33%", Improved: true},
		},
		Resources: ResourceDiff{
			Removed: []RemovedResource{
				{URL: "/old-bundle.js", Type: "script", SizeBytes: 262144},
			},
		},
	}

	summary := GeneratePerfSummary(diff)

	if !strings.Contains(summary, "old-bundle.js") {
		t.Errorf("Summary should mention removed resource. Got: %q", summary)
	}
}

func TestSummary_FlagsRegression(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"cls": {Before: 0.01, After: 0.03, Delta: 0.02, Pct: "+200%", Improved: false},
		},
	}

	summary := GeneratePerfSummary(diff)

	lower := strings.ToLower(summary)
	if !strings.Contains(lower, "regress") && !strings.Contains(lower, "warning") && !strings.Contains(lower, "worse") {
		t.Errorf("Summary should flag CLS regression. Got: %q", summary)
	}
}

func TestSummary_Under200Chars(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"lcp":         {Before: 2800, After: 1200, Delta: -1600, Pct: "-57%", Improved: true},
			"fcp":         {Before: 900, After: 800, Delta: -100, Pct: "-11%", Improved: true},
			"cls":         {Before: 0.02, After: 0.01, Delta: -0.01, Pct: "-50%", Improved: true},
			"ttfb":        {Before: 120, After: 80, Delta: -40, Pct: "-33%", Improved: true},
			"load":        {Before: 1500, After: 1100, Delta: -400, Pct: "-27%", Improved: true},
			"transfer_kb": {Before: 768, After: 512, Delta: -256, Pct: "-33%", Improved: true},
			"requests":    {Before: 58, After: 42, Delta: -16, Pct: "-28%", Improved: true},
		},
		Resources: ResourceDiff{
			Removed: []RemovedResource{
				{URL: "/old-bundle.js", SizeBytes: 262144},
				{URL: "/legacy-polyfill.js", SizeBytes: 131072},
			},
		},
	}

	summary := GeneratePerfSummary(diff)

	if len(summary) > 200 {
		t.Errorf("Summary is %d chars, max 200. Got: %q", len(summary), summary)
	}
}

// ============================================
// Verdict: top-level signal for LLM decision-making
// ============================================

func TestSummary_SortsByPercentageNotAbsoluteDelta(t *testing.T) {
	t.Parallel()
	// CLS has tiny absolute delta (0.2) but huge percentage (+200%)
	// TTFB has large absolute delta (100) but small percentage (+50%)
	// Summary should lead with CLS because percentage is bigger
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"cls":  {Before: 0.1, After: 0.3, Delta: 0.2, Pct: "+200%", Improved: false},
			"ttfb": {Before: 200, After: 300, Delta: 100, Pct: "+50%", Improved: false},
		},
	}

	summary := GeneratePerfSummary(diff)
	if !strings.HasPrefix(strings.ToUpper(summary), "CLS") {
		t.Errorf("Summary should lead with highest percentage (CLS +200%%), not highest delta (TTFB +100ms). Got: %q", summary)
	}
}

// ============================================
// Unit: metric values must carry units for LLM clarity
// ============================================

// Summary: no redundant sign, includes rating
// ============================================

func TestSummary_NoRedundantSign(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"lcp": {Before: 2800, After: 1200, Delta: -1600, Pct: "-57%", Improved: true, Rating: "good"},
		},
	}
	summary := GeneratePerfSummary(diff)
	// "improved" already conveys direction — sign is redundant noise
	if strings.Contains(summary, "improved -") || strings.Contains(summary, "improved +") {
		t.Errorf("Summary has redundant sign after direction word. Got: %q", summary)
	}
}

func TestSummary_IncludesRating(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"lcp": {Before: 4000, After: 1200, Delta: -2800, Pct: "-70%", Improved: true, Rating: "good"},
		},
	}
	summary := GeneratePerfSummary(diff)
	if !strings.Contains(summary, "good") {
		t.Errorf("Summary should include Web Vitals rating. Got: %q", summary)
	}
}

func TestSummary_RegressionShowsAbsolutePercentage(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"lcp": {Before: 1200, After: 4000, Delta: 2800, Pct: "+233%", Improved: false, Rating: "poor"},
		},
	}
	summary := GeneratePerfSummary(diff)
	// Should say "regressed 233%" not "regressed +233%"
	if strings.Contains(summary, "regressed +") {
		t.Errorf("Summary has redundant + sign after 'regressed'. Got: %q", summary)
	}
	if !strings.Contains(summary, "233%") {
		t.Errorf("Summary should include percentage. Got: %q", summary)
	}
}

func TestSummary_DeltaZeroSaysUnchanged(t *testing.T) {
	t.Parallel()
	diff := PerfDiff{
		Metrics: map[string]MetricDiff{
			"load": {Before: 200, After: 200, Delta: 0, Pct: "+0%", Improved: false},
		},
	}
	summary := GeneratePerfSummary(diff)
	// delta=0 should NOT say "regressed" — it's unchanged
	if strings.Contains(strings.ToLower(summary), "regress") {
		t.Errorf("Summary says 'regressed' for delta=0, should say 'unchanged'. Got: %q", summary)
	}
}
