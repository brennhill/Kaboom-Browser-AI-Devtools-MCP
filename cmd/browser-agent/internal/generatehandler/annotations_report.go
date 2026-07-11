// annotations_report.go — Renders annotation sessions into a human-readable Markdown report.
// Why: Separates report formatting concerns from artifact dispatch and storage traversal.
// Docs: docs/features/feature/annotated-screenshots/index.md

package generatehandler

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
)

// GenerateMarkdownReport builds a Markdown report from annotation sessions.
func GenerateMarkdownReport(pages []*annotation.Session, store *annotation.Store) string {
	var b builder
	b.line("# Annotation Report")
	b.linef("Generated: %s", time.Now().Format("2006-01-02 15:04"))
	b.line("")

	totalCount := 0
	for _, pg := range pages {
		totalCount += len(pg.Annotations)
	}
	b.linef("**Total annotations:** %d across %d page(s)", totalCount, len(pages))
	b.line("")

	for i, pg := range pages {
		writePageSection(&b, i, pg, store)
	}

	return b.string()
}

func writePageSection(b *builder, pageIdx int, pg *annotation.Session, store *annotation.Store) {
	b.linef("## Page %d: %s", pageIdx+1, pg.PageURL)
	b.line("")

	if pg.ScreenshotPath != "" {
		b.linef("Screenshot: `%s`", pg.ScreenshotPath)
		b.line("")
	}

	for j, ann := range pg.Annotations {
		writeAnnotationSection(b, j, ann, store)
	}
}

func writeAnnotationSection(b *builder, idx int, ann annotation.Annotation, store *annotation.Store) {
	b.linef("### %d. %s", idx+1, ann.Text)
	b.linef("- **Element:** %s", ann.ElementSummary)
	b.linef("- **Region:** (%.0f, %.0f) %0.fx%.0f", ann.Rect.X, ann.Rect.Y, ann.Rect.Width, ann.Rect.Height)

	detail, found := store.GetDetail(ann.CorrelationID)
	if found {
		writeAnnotationDetail(b, detail)
	}
	b.line("")
}

func writeAnnotationDetail(b *builder, detail *annotation.Detail) {
	b.linef("- **Selector:** `%s`", detail.Selector)
	if len(detail.ComputedStyles) > 0 {
		b.line("- **Styles:**")
		for prop, val := range detail.ComputedStyles {
			b.linef("  - `%s`: `%s`", prop, val)
		}
	}
	if len(detail.A11yFlags) > 0 {
		b.line("- **Accessibility issues:**")
		for _, flag := range detail.A11yFlags {
			b.linef("  - %s", flag)
		}
	}
}
