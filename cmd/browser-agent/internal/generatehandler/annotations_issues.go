// annotations_issues.go — Converts annotation sessions into structured issue payloads.
// Why: Isolates issue-shape construction so tool handlers stay focused on response orchestration.
// Docs: docs/features/feature/annotated-screenshots/index.md

package generatehandler

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"

// BuildIssueList converts annotation pages into a flat issue list with detail enrichment.
func BuildIssueList(pages []*annotation.Session, store *annotation.Store) []map[string]any {
	issues := make([]map[string]any, 0)

	for _, pg := range pages {
		for _, ann := range pg.Annotations {
			issue := map[string]any{
				"annotation_id":  ann.ID,
				"text":           ann.Text,
				"element":        ann.ElementSummary,
				"page_url":       pg.PageURL,
				"rect":           ann.Rect,
				"correlation_id": ann.CorrelationID,
			}

			detail, found := store.GetDetail(ann.CorrelationID)
			if found {
				issue["selector"] = detail.Selector
				issue["tag"] = detail.Tag
				issue["computed_styles"] = detail.ComputedStyles
				if len(detail.A11yFlags) > 0 {
					issue["a11y_flags"] = detail.A11yFlags
				}
			}

			issues = append(issues, issue)
		}
	}

	return issues
}
