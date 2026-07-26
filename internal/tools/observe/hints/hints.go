// Purpose: Package hints — the "why is this empty?" copy observe attaches when a mode returns 0 entries.
// Why: These strings are the tool's user-facing guidance and the main thing an agent reads when a call comes
// back empty; keeping them in one package keeps the wording consistent and independently testable, and lets
// call sites read as hints.Logs(scope, minLevel) instead of logsEmptyHint(scope, minLevel).
// Originally fixes #278 and #287.
// Docs: docs/features/feature/observe/index.md

/*
Package hints builds the diagnostic "hint" field observe attaches to an
otherwise-empty response. Each function corresponds to one observe mode and
explains the most likely reason that mode has nothing to report — capture
started after the events happened, a filter excluded everything, or the
extension is not tracking — followed by the call that would fix it.
*/
package hints

import (
	"fmt"
	"strings"
)

// NetworkBodiesFilters captures active network_bodies filters for hint generation.
type NetworkBodiesFilters struct {
	URL       string
	Method    string
	StatusMin int
	StatusMax int
	BodyPath  string
}

// NetworkBodies returns a diagnostic hint when GetNetworkBodies returns
// 0 filtered entries. It cross-references the waterfall count to explain
// whether the issue is prospective-only capture, a URL filter mismatch, or no data at all.
// waterfallCount is the number of entries in the network waterfall buffer.
func NetworkBodies(waterfallCount int, unfilteredCount int, filters NetworkBodiesFilters) string {
	activeFilterSummary := formatNetworkBodiesFilters(filters)

	// Case 1: Filter reduced non-empty results to zero
	if unfilteredCount > 0 && activeFilterSummary != "" {
		return fmt.Sprintf(
			"No bodies matched filters (%s), but %d bodies exist in the buffer. "+
				"Try broadening the filters or calling observe({what: \"network_bodies\"}) without filter params.",
			activeFilterSummary, unfilteredCount,
		)
	}

	// Case 2: No bodies at all — check waterfall for context
	if waterfallCount > 0 {
		return fmt.Sprintf(
			"No network bodies captured, but the network waterfall shows %d requests. "+
				"Bodies are only captured for requests made after the extension started tracking. "+
				"Requests that completed before tracking began appear in the waterfall (via PerformanceResourceTiming) "+
				"but their bodies were not intercepted. Navigate to a new page or trigger new requests to capture bodies.",
			waterfallCount,
		)
	}

	// Case 3: No waterfall, no bodies — nothing captured yet
	return "No network bodies captured. Ensure the Kaboom extension is connected and tracking a tab. " +
		"Bodies are captured for requests made after tracking starts. Check observe({what: \"pilot\"}) for extension status."
}

func formatNetworkBodiesFilters(filters NetworkBodiesFilters) string {
	parts := make([]string, 0, 5)
	if filters.URL != "" {
		parts = append(parts, fmt.Sprintf("url~%q", filters.URL))
	}
	if filters.Method != "" {
		parts = append(parts, fmt.Sprintf("method=%s", strings.ToUpper(filters.Method)))
	}
	if filters.StatusMin > 0 && filters.StatusMax > 0 {
		parts = append(parts, fmt.Sprintf("status=%d..%d", filters.StatusMin, filters.StatusMax))
	} else if filters.StatusMin > 0 {
		parts = append(parts, fmt.Sprintf("status>=%d", filters.StatusMin))
	} else if filters.StatusMax > 0 {
		parts = append(parts, fmt.Sprintf("status<=%d", filters.StatusMax))
	}
	if filters.BodyPath != "" {
		parts = append(parts, fmt.Sprintf("body_path=%s", filters.BodyPath))
	}
	return strings.Join(parts, ", ")
}

// WSEvents returns a diagnostic hint when GetWSEvents returns 0 filtered entries.
func WSEvents(unfilteredCount int, urlFilter string) string {
	// Case 1: Filter reduced non-empty results to zero
	if unfilteredCount > 0 && urlFilter != "" {
		return fmt.Sprintf(
			"No WebSocket events matched the url filter %q, but %d events exist unfiltered. "+
				"Try broadening the filter or calling observe({what: \"websocket_events\"}) without a url param.",
			urlFilter, unfilteredCount,
		)
	}

	// Case 2: No events at all
	return "No WebSocket events captured. WebSocket interception must be active before connections open. " +
		"If the page was loaded before tracking started, existing WebSocket connections are not captured. " +
		"Navigate to the page again (or refresh) while tracking is active to capture WebSocket traffic."
}

// WSStatus returns a diagnostic hint when GetWSStatus returns 0 connections.
func WSStatus() string {
	return "No WebSocket connections found. WebSocket interception must be active before connections open. " +
		"If the page was loaded before tracking started, existing connections are not visible. " +
		"Navigate to the page again (or refresh) while tracking is active to detect WebSocket connections."
}

// Errors returns a diagnostic hint when GetBrowserErrors returns 0 entries.
func Errors(scope string) string {
	if scope == "current_page" {
		return "No errors on the current page. If you expected errors, try observe({what: \"errors\", scope: \"all\"}) " +
			"to check all tabs, or observe({what: \"logs\", min_level: \"warn\"}) for warnings. " +
			"Errors are captured in real-time — trigger the action that causes the error and re-check."
	}
	return "No errors captured in any tab. Errors are captured in real-time as they occur. " +
		"Trigger the action that causes the error, then call observe({what: \"errors\"}) again. " +
		"Check observe({what: \"pilot\"}) to verify the extension is connected and tracking."
}

// Logs returns a diagnostic hint when GetBrowserLogs returns 0 entries.
func Logs(scope, minLevel string) string {
	if minLevel != "" {
		return fmt.Sprintf("No logs at level '%s' or above. Try a lower threshold: "+
			"observe({what: \"logs\", min_level: \"debug\"}) to see all logs, "+
			"or remove min_level to get all levels.", minLevel)
	}
	if scope == "current_page" {
		return "No console logs on the current page. Try observe({what: \"logs\", scope: \"all\"}) " +
			"to check all tabs. Logs are captured in real-time — interact with the page to generate output."
	}
	return "No console logs captured. Logs are captured in real-time as console.log/warn/error are called. " +
		"Interact with the page to generate logs. Check observe({what: \"pilot\"}) for extension status."
}

// Actions returns a diagnostic hint when GetEnhancedActions returns 0 entries.
func Actions() string {
	return "No user actions captured yet. Actions (clicks, inputs, navigations, form submissions) are recorded " +
		"in real-time as the user interacts with the page. Interact with the page, then re-check. " +
		"Check observe({what: \"pilot\"}) to verify the extension is connected and tracking."
}

// Timeline returns a diagnostic hint when GetSessionTimeline returns 0 entries.
func Timeline() string {
	return "No timeline events captured. The timeline merges errors, actions, network requests, and WebSocket events. " +
		"Interact with the page to generate activity, then call observe({what: \"timeline\"}) again. " +
		"Check observe({what: \"pilot\"}) for extension status."
}

// ErrorBundles returns a diagnostic hint when GetErrorBundles returns 0 bundles.
func ErrorBundles() string {
	return "No error bundles — no errors captured in the current window. Error bundles are assembled around each " +
		"console error with surrounding network/action context. Trigger the error scenario, then re-check. " +
		"Try observe({what: \"errors\"}) to see if individual errors exist without bundle context."
}

// Transients returns a diagnostic hint when GetTransients returns 0 entries.
func Transients(classification string) string {
	if classification != "" {
		return fmt.Sprintf("No transient elements with classification '%s'. "+
			"Try observe({what: \"transients\"}) without a classification filter to see all types, "+
			"or interact with the page to trigger toast/alert/snackbar elements.", classification)
	}
	return "No transient UI elements captured. Transients (toasts, alerts, snackbars, notifications) are detected " +
		"in real-time as they appear in the DOM. Interact with the page to trigger transient UI, then re-check."
}

// NetworkWaterfall returns a diagnostic hint when GetNetworkWaterfall returns 0 entries.
func NetworkWaterfall(urlFilter string) string {
	if urlFilter != "" {
		return fmt.Sprintf("No network requests matched URL filter %q. "+
			"Try observe({what: \"network_waterfall\"}) without a url filter to see all requests.", urlFilter)
	}
	return "No network requests captured. The waterfall is populated from the browser's Performance API " +
		"and live request interception. Navigate to a page or trigger requests, then re-check. " +
		"Check observe({what: \"pilot\"}) for extension status."
}
