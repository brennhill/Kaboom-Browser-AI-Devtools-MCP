// Purpose: Builds project-level summaries and scope warnings from annotation sessions.
// Why: Isolates project detection logic from annotation handler flow control.
// Docs: docs/features/feature/annotated-screenshots/index.md

package toolanalyze

import (
	"net/url"
	"sort"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
)

// BuildProjectSummaries builds project-level summaries from annotation sessions.
func BuildProjectSummaries(pages []*annotation.Session) []map[string]any {
	type projectAggregate struct {
		pageSet         map[string]struct{}
		annotationCount int
	}
	projects := make(map[string]*projectAggregate)
	for _, page := range pages {
		baseURL := AnnotationProjectBaseURL(page.PageURL)
		if strings.TrimSpace(baseURL) == "" {
			continue
		}
		agg, ok := projects[baseURL]
		if !ok {
			agg = &projectAggregate{
				pageSet: make(map[string]struct{}),
			}
			projects[baseURL] = agg
		}
		agg.annotationCount += len(page.Annotations)
		if strings.TrimSpace(page.PageURL) != "" {
			agg.pageSet[page.PageURL] = struct{}{}
		}
	}

	if len(projects) == 0 {
		return nil
	}
	baseURLs := make([]string, 0, len(projects))
	for baseURL := range projects {
		baseURLs = append(baseURLs, baseURL)
	}
	sort.Strings(baseURLs)

	summaries := make([]map[string]any, 0, len(baseURLs))
	for _, baseURL := range baseURLs {
		agg := projects[baseURL]
		pageURLs := make([]string, 0, len(agg.pageSet))
		for pageURL := range agg.pageSet {
			pageURLs = append(pageURLs, pageURL)
		}
		sort.Strings(pageURLs)
		summary := map[string]any{
			"base_url":           baseURL,
			"annotation_count":   agg.annotationCount,
			"page_count":         len(pageURLs),
			"recommended_filter": baseURL + "/*",
		}
		if len(pageURLs) > 0 {
			summary["page_urls"] = pageURLs
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

// BuildScopeWarning creates a multi-project warning message.
func BuildScopeWarning(projects []map[string]any) map[string]any {
	suggestedFilters := make([]string, 0, len(projects))
	projectBaseURLs := make([]string, 0, len(projects))
	for _, project := range projects {
		if filter, ok := project["recommended_filter"].(string); ok && filter != "" {
			suggestedFilters = append(suggestedFilters, filter)
		}
		if baseURL, ok := project["base_url"].(string); ok && baseURL != "" {
			projectBaseURLs = append(projectBaseURLs, baseURL)
		}
	}
	return map[string]any{
		"warning":           "MULTI-PROJECT ANNOTATION SESSION DETECTED: annotations span multiple projects.",
		"recommendation":    "Re-run analyze({what:'annotations'}) with 'url' scoped to the active project before implementing changes.",
		"suggested_filters": suggestedFilters,
		"projects_detected": projectBaseURLs,
	}
}

// AnnotationProjectBaseURL extracts the base URL (scheme + host) from a page URL.
func AnnotationProjectBaseURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return rawURL
	}
	return parsed.Scheme + "://" + parsed.Host
}
