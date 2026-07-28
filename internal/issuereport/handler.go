// handler.go — Configure issue-report operation routing and response shaping.
// Docs: docs/features/feature/issue-reporting/index.md

package issuereport

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type HandlerDeps struct {
	Collect  func(template, title, userContext string) IssueReport
	Sanitize func(report IssueReport) IssueReport
	Submit   func(report IssueReport) SubmitResult
}

func Handle(d HandlerDeps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Operation   string `json:"operation"`
		Template    string `json:"template"`
		Title       string `json:"title"`
		UserContext string `json:"user_context"`
	}
	if len(args) > 0 {
		if response, stop := mcp.ParseArgs(req, args, &params); stop {
			return response
		}
	}
	switch params.Operation {
	case "list_templates":
		return listTemplates(req)
	case "submit":
		return submit(d, req, params.Template, params.Title, params.UserContext)
	case "preview", "":
		return preview(d, req, params.Template, params.UserContext)
	default:
		return mcp.Fail(req, mcp.ErrInvalidParam, "Unknown operation: "+params.Operation,
			"Use list_templates, preview, or submit", mcp.WithParam("operation"))
	}
}

func listTemplates(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	names := TemplateNames()
	templates := make([]map[string]any, 0, len(names))
	for _, name := range names {
		template := GetTemplate(name)
		templates = append(templates, map[string]any{
			"name": template.Name, "title": template.Title, "description": template.Description,
		})
	}
	return mcp.Succeed(req, "Available issue templates", map[string]any{
		"templates": templates, "count": len(templates),
	})
}

func preview(d HandlerDeps, req mcp.JSONRPCRequest, template, userContext string) mcp.JSONRPCResponse {
	if template == "" {
		template = "bug"
	}
	if GetTemplate(template) == nil {
		return unknownTemplate(req, template)
	}
	report := d.Sanitize(d.Collect(template, "Preview: "+template, userContext))
	return mcp.Succeed(req, "Issue preview (nothing sent)", map[string]any{
		"operation":      "preview",
		"template":       template,
		"report":         report,
		"formatted_body": FormatIssueBody(report),
		"hint":           "Review the payload above. Call with operation=\"submit\" and a title to file the issue.",
	})
}

func submit(d HandlerDeps, req mcp.JSONRPCRequest, template, title, userContext string) mcp.JSONRPCResponse {
	if title == "" {
		return mcp.Fail(req, mcp.ErrMissingParam, "title is required for submit",
			"Provide a title describing the issue", mcp.WithParam("title"))
	}
	if template == "" {
		template = "bug"
	}
	if GetTemplate(template) == nil {
		return unknownTemplate(req, template)
	}
	report := d.Sanitize(d.Collect(template, title, userContext))
	return mcp.Succeed(req, "Issue submission result", d.Submit(report))
}

func unknownTemplate(req mcp.JSONRPCRequest, template string) mcp.JSONRPCResponse {
	return mcp.Fail(req, mcp.ErrInvalidParam, "Unknown template: "+template,
		"Use list_templates to see available templates", mcp.WithParam("template"))
}
