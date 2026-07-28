// Purpose: Encodes annotation session payloads for async command completion responses.
// Why: Keeps result-shape logic centralized so waiter completion stays consistent across call sites.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotation

import (
	"encoding/json"
	"net/url"
	"strings"
)

// BuildSessionResult serializes an annotation session for the CommandTracker.
func BuildSessionResult(session *Session, urlFilter string) json.RawMessage {
	annotations := session.Annotations
	if !URLMatches(urlFilter, session.PageURL) {
		annotations = []Annotation{}
	}
	result := map[string]any{
		"status":          "complete",
		"annotations":     annotations,
		"count":           len(annotations),
		"page_url":        session.PageURL,
		"terminal_reason": "completed",
		"filter_applied":  FilterAppliedValue(urlFilter),
	}
	if session.ScreenshotPath != "" && len(annotations) > 0 {
		result["screenshot"] = session.ScreenshotPath
	}
	// Error impossible: map of primitive types.
	data, _ := json.Marshal(result)
	return data
}

// NamedSessionPageView is the canonical filtered projection shared by async
// completion and MCP analysis responses.
type NamedSessionPageView struct {
	FilteredPages []*Session
	Pages         []map[string]any
	TotalCount    int
}

// BuildNamedSessionPageView filters and projects the pages in a named session.
func BuildNamedSessionPageView(ns *NamedSession, urlFilter string) NamedSessionPageView {
	filteredPages := filterPagesByURL(ns.Pages, urlFilter)
	view := NamedSessionPageView{
		FilteredPages: filteredPages,
		Pages:         make([]map[string]any, 0, len(filteredPages)),
	}
	for _, page := range filteredPages {
		view.TotalCount += len(page.Annotations)
		projected := map[string]any{
			"page_url":    page.PageURL,
			"annotations": page.Annotations,
			"count":       len(page.Annotations),
			"tab_id":      page.TabID,
		}
		if page.ScreenshotPath != "" {
			projected["screenshot"] = page.ScreenshotPath
		}
		view.Pages = append(view.Pages, projected)
	}
	return view
}

// BuildNamedSessionResult serializes a named session for the CommandTracker.
func BuildNamedSessionResult(ns *NamedSession, urlFilter string) json.RawMessage {
	view := BuildNamedSessionPageView(ns, urlFilter)
	result := map[string]any{
		"status":             "complete",
		"annot_session_name": ns.Name,
		"pages":              view.Pages,
		"page_count":         len(view.FilteredPages),
		"total_count":        view.TotalCount,
		"terminal_reason":    "completed",
		"filter_applied":     FilterAppliedValue(urlFilter),
	}
	// Error impossible: map of primitive types.
	data, _ := json.Marshal(result)
	return data
}

// FilterAppliedValue returns the effective filter label for annotation responses.
// Returns "none" when no filter is applied (empty/whitespace input).
func FilterAppliedValue(urlFilter string) string {
	if strings.TrimSpace(urlFilter) == "" {
		return "none"
	}
	return urlFilter
}

func filterPagesByURL(pages []*Session, urlFilter string) []*Session {
	if strings.TrimSpace(urlFilter) == "" {
		return pages
	}
	filtered := make([]*Session, 0, len(pages))
	for _, page := range pages {
		if URLMatches(urlFilter, page.PageURL) {
			filtered = append(filtered, page)
		}
	}
	return filtered
}

// URLMatches checks whether pageURL matches the given URL filter.
// Supports exact match, wildcard, path-prefix, and origin-only patterns.
func URLMatches(urlFilter, pageURL string) bool {
	urlFilter = strings.TrimSpace(urlFilter)
	if urlFilter == "" {
		return true
	}
	pageURL = strings.TrimSpace(pageURL)
	if pageURL == "" {
		return false
	}

	// Support wildcard suffix filters like http://localhost:3000/*.
	if strings.HasSuffix(urlFilter, "/*") {
		return URLMatches(strings.TrimSuffix(urlFilter, "*"), pageURL)
	}
	// T10 fix: proper glob matching. Split at the first '*' and check that
	// the URL starts with the prefix and (if present) ends with the suffix.
	// This prevents "http://localhost:*" from matching "http://localhost-evil.com".
	if strings.Contains(urlFilter, "*") {
		parts := strings.SplitN(urlFilter, "*", 2)
		if !strings.HasPrefix(pageURL, parts[0]) {
			return false
		}
		if len(parts) > 1 && parts[1] != "" && !strings.HasSuffix(pageURL, parts[1]) {
			return false
		}
		return true
	}

	filterURL, filterErr := url.Parse(urlFilter)
	page, pageErr := url.Parse(pageURL)
	if filterErr == nil && pageErr == nil &&
		filterURL.Scheme != "" && filterURL.Host != "" &&
		page.Scheme != "" && page.Host != "" {
		if !strings.EqualFold(filterURL.Scheme, page.Scheme) || !strings.EqualFold(filterURL.Host, page.Host) {
			return false
		}

		filterPath := strings.TrimSpace(filterURL.Path)
		switch {
		case filterPath == "", filterPath == "/":
			// Base URL filter: match any path on the same origin.
			return true
		case strings.HasSuffix(filterPath, "/"):
			// Path prefix filter.
			return strings.HasPrefix(page.Path, filterPath)
		default:
			// Exact path filter. Query/fragment are optional constraints when provided.
			if page.Path != filterPath {
				return false
			}
			if filterURL.RawQuery != "" && page.RawQuery != filterURL.RawQuery {
				return false
			}
			if filterURL.Fragment != "" && page.Fragment != filterURL.Fragment {
				return false
			}
			return true
		}
	}

	if strings.HasSuffix(urlFilter, "/") {
		return strings.HasPrefix(pageURL, urlFilter)
	}

	return pageURL == urlFilter
}
