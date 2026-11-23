// sanitize.go — Sanitization layer that wraps a Redactor to scrub issue reports.

package issuereport

// Redactor defines the minimal interface for string-level redaction.
// The concrete implementation lives in internal/platform/redaction.
type Redactor interface {
	Redact(input string) string
}

// Sanitizer wraps a Redactor to sanitize issue reports before submission.
type Sanitizer struct {
	redactor Redactor
}

// NewSanitizer creates a Sanitizer backed by the given Redactor.
func NewSanitizer(r Redactor) *Sanitizer {
	return &Sanitizer{redactor: r}
}

// SanitizeReport applies redaction to all user-supplied and diagnostic strings
// in the report. Returns a new report; the original is not modified.
//
// Redacted fields: Title, UserContext, Extension.Source.
// Not redacted: Server.Version, Platform.* (safe runtime constants).
func (s *Sanitizer) SanitizeReport(report IssueReport) IssueReport {
	out := report
	out.Title = s.redactor.Redact(report.Title)
	out.UserContext = s.redactor.Redact(report.UserContext)
	out.Diagnostics.Extension.Source = s.redactor.Redact(report.Diagnostics.Extension.Source)
	return out
}
