// Purpose: Assembles API contract analysis/report responses from tracked endpoint state.
// Why: Keeps aggregation and response formatting separate from violation detection logic.
// Docs: docs/features/feature/api-schema/index.md

package apicontract

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ============================================
// MCP Tool Interface
// ============================================

// analyzeAccum holds intermediate totals during endpoint analysis.
type analyzeAccum struct {
	violations       []APIContractViolation
	totalRequests    int
	cleanEndpoints   int
	trackedEndpoints int
	earliestCall     time.Time
}

// Analyze processes tracked endpoints and returns all violations.
func (v *APIContractValidator) Analyze(filter APIContractFilter) APIContractAnalyzeResult {
	v.mu.RLock()
	defer v.mu.RUnlock()

	acc := v.accumulateViolations(filter)

	sort.Slice(acc.violations, func(i, j int) bool {
		return acc.violations[i].Endpoint < acc.violations[j].Endpoint
	})

	return v.buildAnalyzeResult(acc, filter)
}

// accumulateViolations collects violations and statistics from all tracked endpoints.
func (v *APIContractValidator) accumulateViolations(filter APIContractFilter) analyzeAccum {
	var acc analyzeAccum
	for endpoint, tracker := range v.trackers {
		if !v.matchesFilter(endpoint, filter) {
			continue
		}
		acc.trackedEndpoints++
		acc.totalRequests += tracker.CallCount
		if !tracker.FirstCalled.IsZero() && (acc.earliestCall.IsZero() || tracker.FirstCalled.Before(acc.earliestCall)) {
			acc.earliestCall = tracker.FirstCalled
		}
		if len(tracker.Violations) > 0 {
			acc.violations = append(acc.violations, tracker.Violations...)
		} else {
			acc.cleanEndpoints++
		}
	}
	return acc
}

// buildAnalyzeResult constructs the final analysis result from accumulated data.
func (v *APIContractValidator) buildAnalyzeResult(acc analyzeAccum, filter APIContractFilter) APIContractAnalyzeResult {
	result := APIContractAnalyzeResult{
		Action:                "analyzed",
		AnalyzedAt:            time.Now().Format(time.RFC3339),
		Violations:            acc.violations,
		TrackedEndpoints:      acc.trackedEndpoints,
		TotalRequestsAnalyzed: acc.totalRequests,
		CleanEndpoints:        acc.cleanEndpoints,
		Summary: &AnalyzeSummary{
			Violations: len(acc.violations), Endpoints: acc.trackedEndpoints,
			TotalRequests: acc.totalRequests, CleanEndpoints: acc.cleanEndpoints,
		},
		PossibleViolationTypes: []string{"shape_change", "type_change", "error_spike", "new_field", "null_field"},
	}
	if !acc.earliestCall.IsZero() {
		result.DataWindowStartedAt = acc.earliestCall.Format(time.RFC3339)
	}
	if filter.URLFilter != "" || len(filter.IgnoreEndpoints) > 0 {
		result.AppliedFilter = &AppliedFilterEcho{URL: filter.URLFilter, IgnoreEndpoints: filter.IgnoreEndpoints}
	}
	if len(acc.violations) == 0 {
		result.Hint = analyzeHint(acc.trackedEndpoints)
	}
	return result
}

// analyzeHint returns a helpful hint when no violations are found.
func analyzeHint(trackedEndpoints int) string {
	if trackedEndpoints > 0 {
		return fmt.Sprintf("No violations detected. All %d tracked endpoint(s) have consistent response shapes.", trackedEndpoints)
	}
	return "No violations detected. No endpoints tracked yet — browse your application to capture API traffic."
}

// Report returns the current state of all tracked endpoint schemas.
func (v *APIContractValidator) Report(filter APIContractFilter) APIContractReportResult {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var endpoints []EndpointContractReport
	for endpoint, tracker := range v.trackers {
		if !v.matchesFilter(endpoint, filter) {
			continue
		}
		endpoints = append(endpoints, buildEndpointReport(endpoint, tracker))
	}

	// Sort by call count (most used first).
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].CallCount > endpoints[j].CallCount
	})

	result := APIContractReportResult{
		Action:     "report",
		AnalyzedAt: time.Now().Format(time.RFC3339),
		Endpoints:  endpoints,
		ConsistencyLevels: map[string]string{
			"1.0":       "Perfect — all responses match established schema",
			"0.9-0.99":  "Good — occasional minor deviations",
			"0.7-0.89":  "Degraded — frequent schema mismatches, investigate",
			"below 0.7": "Poor — endpoint contract is unstable",
		},
	}

	if filter.URLFilter != "" || len(filter.IgnoreEndpoints) > 0 {
		result.AppliedFilter = &AppliedFilterEcho{URL: filter.URLFilter, IgnoreEndpoints: filter.IgnoreEndpoints}
	}

	return result
}

// buildEndpointReport creates a contract report for a single endpoint tracker.
func buildEndpointReport(endpoint string, tracker *EndpointTracker) EndpointContractReport {
	statusCodes := make(map[string]int)
	for _, status := range tracker.StatusHistory {
		statusCodes[fmt.Sprintf("%d", status)]++
	}

	consistency, score := computeConsistency(tracker.ConsistentCount, tracker.CallCount)
	parts := strings.SplitN(endpoint, " ", 2)

	report := EndpointContractReport{
		Endpoint:         endpoint,
		Method:           parts[0],
		CallCount:        tracker.CallCount,
		StatusCodes:      statusCodes,
		Consistency:      consistency,
		ConsistencyScore: score,
	}
	if !tracker.LastCalled.IsZero() {
		report.LastCalledAt = tracker.LastCalled.Format(time.RFC3339)
	}
	if !tracker.FirstCalled.IsZero() {
		report.FirstCalledAt = tracker.FirstCalled.Format(time.RFC3339)
	}
	if tracker.EstablishedShape != nil {
		if shapeMap, ok := tracker.EstablishedShape.(map[string]any); ok {
			report.EstablishedShape = toStringMap(shapeMap)
		}
	}
	return report
}

// computeConsistency returns the human-readable consistency string and numeric score.
func computeConsistency(consistentCount, callCount int) (string, float64) {
	if callCount == 0 {
		return "100%", 1.0
	}
	score := float64(consistentCount) / float64(callCount)
	return fmt.Sprintf("%.0f%%", score*100), score
}

// Clear resets all tracked endpoint data.
func (v *APIContractValidator) Clear() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.trackers = make(map[string]*EndpointTracker)
}

// GetTrackers returns a copy of trackers for testing.
func (v *APIContractValidator) GetTrackers() map[string]*EndpointTracker {
	v.mu.RLock()
	defer v.mu.RUnlock()

	copyTrackers := make(map[string]*EndpointTracker)
	for key, tracker := range v.trackers {
		copyTrackers[key] = tracker
	}
	return copyTrackers
}

// ============================================
// Helpers
// ============================================

func (v *APIContractValidator) matchesFilter(endpoint string, filter APIContractFilter) bool {
	if filter.URLFilter != "" && !strings.Contains(endpoint, filter.URLFilter) {
		return false
	}
	for _, ignore := range filter.IgnoreEndpoints {
		if strings.Contains(endpoint, ignore) {
			return false
		}
	}
	return true
}
