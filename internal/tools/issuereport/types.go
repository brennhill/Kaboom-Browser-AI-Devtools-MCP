// types.go — Core types for issue reporting: templates, reports, diagnostics, and submission results.

package issuereport

// IssueTemplate defines a category of issue that can be reported.
type IssueTemplate struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Labels      []string `json:"labels"`
	Sections    []string `json:"sections"`
}

// IssueReport is the sanitized payload ready for submission.
type IssueReport struct {
	Template    string         `json:"template"`
	Title       string         `json:"title"`
	UserContext string         `json:"user_context,omitempty"`
	Diagnostics DiagnosticData `json:"diagnostics"`
}

// DiagnosticData collects runtime context for an issue report.
type DiagnosticData struct {
	Server    ServerDiag    `json:"server"`
	Extension ExtensionDiag `json:"extension"`
	Platform  PlatformDiag  `json:"platform"`
	Buffers   BufferStats   `json:"buffers"`
}

// ServerDiag contains server-side diagnostic info.
type ServerDiag struct {
	Version       string  `json:"version"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	TotalCalls    int64   `json:"total_calls"`
	TotalErrors   int64   `json:"total_errors"`
	ErrorRatePct  float64 `json:"error_rate_pct"`
}

// ExtensionDiag contains extension connectivity diagnostics.
type ExtensionDiag struct {
	Connected bool   `json:"connected"`
	Source    string `json:"source,omitempty"`
}

// PlatformDiag contains platform information.
type PlatformDiag struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	GoVersion string `json:"go_version"`
}

// BufferStats contains buffer utilization snapshot.
type BufferStats struct {
	ConsoleEntries int `json:"console_entries"`
	NetworkEntries int `json:"network_entries"`
	ActionEntries  int `json:"action_entries"`
}

// SubmitResult is the response from a submission attempt.
type SubmitResult struct {
	Status        string `json:"status"`
	Method        string `json:"method,omitempty"`
	IssueURL      string `json:"issue_url,omitempty"`
	FormattedBody string `json:"formatted_body,omitempty"`
	RepoURL       string `json:"repo_url,omitempty"`
	Hint          string `json:"hint,omitempty"`
	Error         string `json:"error,omitempty"`
}
