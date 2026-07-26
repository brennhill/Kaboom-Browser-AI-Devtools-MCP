// Purpose: Captures verification snapshots and normalizes console/network data for comparison.
// Why: Isolates snapshot transformation logic from session action orchestration.
// Docs: docs/features/feature/request-session-correlation/index.md

package verify

import (
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session"
)

// ============================================
// Snapshot Capture
// ============================================

// convertConsoleErrors converts snapshot errors to verification errors.
func convertConsoleErrors(errors []session.SnapshotError) []VerifyError {
	result := make([]VerifyError, 0, len(errors))
	for _, e := range errors {
		result = append(result, VerifyError{
			Message:    e.Message,
			Normalized: normalizeVerifyErrorMessage(e.Message),
			Count:      e.Count,
		})
	}
	return truncateSlice(result, maxBaselineErrors)
}

// convertNetworkRequests converts and filters network requests, returning all requests and error-only requests.
func convertNetworkRequests(network []session.SnapshotNetworkRequest, urlFilter string) ([]VerifyNetworkEntry, []VerifyNetworkEntry) {
	allNetwork := make([]VerifyNetworkEntry, 0, len(network))
	networkErrors := make([]VerifyNetworkEntry, 0)
	for _, req := range network {
		if urlFilter != "" && !strings.Contains(req.URL, urlFilter) {
			continue
		}
		entry := VerifyNetworkEntry{
			Method: req.Method, URL: req.URL,
			Path: capture.ExtractURLPath(req.URL), Status: req.Status, Duration: req.Duration,
		}
		allNetwork = append(allNetwork, entry)
		if req.Status >= 400 {
			networkErrors = append(networkErrors, entry)
		}
	}
	return truncateSlice(allNetwork, maxBaselineNetworkEntries),
		truncateSlice(networkErrors, maxBaselineNetworkEntries)
}

// truncateSlice returns the first maxLen elements of a slice, or the full slice if shorter.
func truncateSlice[T any](s []T, maxLen int) []T {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// captureSnapshot captures current state from the reader.
func (vm *VerificationManager) captureSnapshot(urlFilter string) *VerifSnapshot {
	perf := vm.reader.GetPerformance()
	allNetwork, networkErrors := convertNetworkRequests(vm.reader.GetNetworkRequests(), urlFilter)

	var perfCopy *performance.Snapshot
	if perf != nil {
		p := *perf
		perfCopy = &p
	}

	return &VerifSnapshot{
		CapturedAt:         time.Now(),
		ConsoleErrors:      convertConsoleErrors(vm.reader.GetConsoleErrors()),
		NetworkErrors:      networkErrors,
		AllNetworkRequests: allNetwork,
		PageURL:            vm.reader.GetCurrentPageURL(),
		Performance:        perfCopy,
	}
}

// buildBaselineSummary creates a summary of the baseline snapshot.
func (vm *VerificationManager) buildBaselineSummary(baseline *VerifSnapshot) BaselineSummary {
	// Count total console errors.
	consoleErrorCount := 0
	for _, e := range baseline.ConsoleErrors {
		consoleErrorCount += e.Count
	}

	// Build error details.
	details := make([]ErrorDetail, 0, len(baseline.ConsoleErrors)+len(baseline.NetworkErrors))

	for _, e := range baseline.ConsoleErrors {
		details = append(details, ErrorDetail{
			Type:    "console",
			Message: e.Message,
			Count:   e.Count,
		})
	}

	for _, n := range baseline.NetworkErrors {
		details = append(details, ErrorDetail{
			Type:   "network",
			Method: n.Method,
			URL:    n.URL,
			Status: n.Status,
		})
	}

	return BaselineSummary{
		CapturedAt:    baseline.CapturedAt,
		ConsoleErrors: consoleErrorCount,
		NetworkErrors: len(baseline.NetworkErrors),
		ErrorDetails:  details,
	}
}

// ============================================
// Helper Functions
// ============================================

// normalizeVerifyErrorMessage normalizes dynamic values in error messages for matching.
// Uses similar patterns to clustering.go but returns placeholders in a test-friendly format.
func normalizeVerifyErrorMessage(msg string) string {
	// Order matters: apply more specific patterns first.
	// File:line must be matched before numeric IDs, or the line number gets replaced first.
	result := clusterUUIDRegex.ReplaceAllString(msg, "[uuid]")
	result = clusterTimestampRegex.ReplaceAllString(result, "[timestamp]")
	result = verifyFileLineRegex.ReplaceAllString(result, "[file]")
	result = clusterNumericIDRegex.ReplaceAllString(result, "[id]")
	return result
}
