// Purpose: Filters network waterfall and body entries by URL, method, and status before HAR export.
// Why: Isolates filter matching from conversion and file writing to keep each concern testable.
package har

import (
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// matchesWaterfallFilter checks if a waterfall entry passes the filter criteria.
// Status filters are skipped since waterfall entries have no status code.
func matchesWaterfallFilter(wf types.NetworkWaterfallEntry, filter types.NetworkBodyFilter) bool {
	if filter.URLFilter != "" && !strings.Contains(strings.ToLower(wf.URL), strings.ToLower(filter.URLFilter)) {
		return false
	}
	if filter.Method != "" && !strings.EqualFold("GET", filter.Method) {
		return false
	}
	// Status filters don't apply — waterfall has no status code.
	return true
}

// matchesHARFilter checks if a NetworkBody passes the filter criteria.
func matchesHARFilter(body types.NetworkBody, filter types.NetworkBodyFilter) bool {
	if filter.URLFilter != "" && !strings.Contains(strings.ToLower(body.URL), strings.ToLower(filter.URLFilter)) {
		return false
	}
	if filter.Method != "" && !strings.EqualFold(body.Method, filter.Method) {
		return false
	}
	if filter.StatusMin > 0 && body.Status < filter.StatusMin {
		return false
	}
	if filter.StatusMax > 0 && body.Status > filter.StatusMax {
		return false
	}
	return true
}
