// tools_configure_report_issue.go — Handler for configure(what="report_issue").
// Provides list_templates, preview, and submit operations for filing sanitized bug reports.
// Docs: docs/features/feature/issue-reporting/index.md

package main

import (
	"encoding/json"
	"runtime"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/issuereport"
)

// toolConfigureReportIssue handles configure(what="report_issue") calls.
// Operations: list_templates, preview (default), submit.
func (h *ToolHandler) toolConfigureReportIssue(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Operation   string `json:"operation"`
		Template    string `json:"template"`
		Title       string `json:"title"`
		UserContext string `json:"user_context"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	switch params.Operation {
	case "list_templates":
		return h.reportIssueListTemplates(req)
	case "submit":
		return h.reportIssueSubmit(req, params.Template, params.Title, params.UserContext)
	case "preview", "":
		return h.reportIssuePreview(req, params.Template, params.UserContext)
	default:
		return fail(req, ErrInvalidParam,
			"Unknown operation: "+params.Operation,
			"Use list_templates, preview, or submit",
			withParam("operation"))
	}
}

// reportIssueListTemplates returns the available issue templates.
func (h *ToolHandler) reportIssueListTemplates(req JSONRPCRequest) JSONRPCResponse {
	names := issuereport.TemplateNames()
	templates := make([]map[string]any, 0, len(names))
	for _, name := range names {
		tmpl := issuereport.GetTemplate(name)
		templates = append(templates, map[string]any{
			"name":        tmpl.Name,
			"title":       tmpl.Title,
			"description": tmpl.Description,
		})
	}
	return succeed(req, "Available issue templates",
		map[string]any{"templates": templates, "count": len(templates)})
}

// reportIssuePreview collects diagnostics, sanitizes, and returns the payload without submitting.
func (h *ToolHandler) reportIssuePreview(req JSONRPCRequest, template, userContext string) JSONRPCResponse {
	if template == "" {
		template = "bug"
	}
	if issuereport.GetTemplate(template) == nil {
		return fail(req, ErrInvalidParam,
			"Unknown template: "+template,
			"Use list_templates to see available templates",
			withParam("template"))
	}

	report := h.collectIssueReport(template, "Preview: "+template, userContext)
	sanitized := h.sanitizeIssueReport(report)
	body := issuereport.FormatIssueBody(sanitized)

	return succeed(req, "Issue preview (nothing sent)",
		map[string]any{
			"operation":      "preview",
			"template":       template,
			"report":         sanitized,
			"formatted_body": body,
			"hint":           "Review the payload above. Call with operation=\"submit\" and a title to file the issue.",
		})
}

// reportIssueSubmit validates, sanitizes, and submits an issue.
func (h *ToolHandler) reportIssueSubmit(req JSONRPCRequest, template, title, userContext string) JSONRPCResponse {
	if title == "" {
		return fail(req, ErrMissingParam,
			"title is required for submit",
			"Provide a title describing the issue",
			withParam("title"))
	}
	if template == "" {
		template = "bug"
	}
	if issuereport.GetTemplate(template) == nil {
		return fail(req, ErrInvalidParam,
			"Unknown template: "+template,
			"Use list_templates to see available templates",
			withParam("template"))
	}

	report := h.collectIssueReport(template, title, userContext)
	sanitized := h.sanitizeIssueReport(report)

	// Use shutdownCtx so the gh subprocess is cancelled on daemon shutdown.
	// issueCommandRunner is nil in production (uses real exec); tests inject a fake.
	result := issuereport.SubmitViaGH(h.shutdownCtx, sanitized, h.issueCommandRunner)

	return succeed(req, "Issue submission result",
		result)
}

// collectIssueReport gathers diagnostics from the handler's runtime state.
func (h *ToolHandler) collectIssueReport(template, title, userContext string) issuereport.IssueReport {
	report := issuereport.IssueReport{
		Template:    template,
		Title:       title,
		UserContext: userContext,
	}

	report.Diagnostics.Server.Version = version
	report.Diagnostics.Platform.OS = runtime.GOOS
	report.Diagnostics.Platform.Arch = runtime.GOARCH
	report.Diagnostics.Platform.GoVersion = runtime.Version()

	if h.healthMetrics != nil {
		report.Diagnostics.Server.UptimeSeconds = h.healthMetrics.GetUptime().Seconds()
		audit := h.healthMetrics.BuildAuditInfo()
		report.Diagnostics.Server.TotalCalls = audit.TotalCalls
		report.Diagnostics.Server.TotalErrors = audit.TotalErrors
		report.Diagnostics.Server.ErrorRatePct = audit.ErrorRatePct
	}

	if h.capture != nil {
		health := h.capture.GetHealthSnapshot()
		report.Diagnostics.Extension.Connected = health.ConnectionCount > 0
		report.Diagnostics.Extension.Source = health.ExtSessionID
		report.Diagnostics.Buffers.NetworkEntries = health.NetworkBodyCount
		report.Diagnostics.Buffers.ActionEntries = health.ActionCount
	}

	if h.server != nil {
		report.Diagnostics.Buffers.ConsoleEntries = h.server.logs.EntryCount()
	}

	return report
}

// sanitizeIssueReport applies redaction to the report using the handler's redaction engine.
func (h *ToolHandler) sanitizeIssueReport(report issuereport.IssueReport) issuereport.IssueReport {
	if h.redactionEngine == nil {
		return report
	}
	sanitizer := issuereport.NewSanitizer(h.redactionEngine)
	return sanitizer.SanitizeReport(report)
}
