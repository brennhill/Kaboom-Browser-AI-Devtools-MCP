// catalog.go — Declares the MCP resource catalog and URI templates.

package resources

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"

func Resources() []mcp.MCPResource {
	return []mcp.MCPResource{
		{
			URI:         "kaboom://capabilities",
			Name:        "Kaboom Capability Index",
			Description: "Compact capability index with task-to-playbook routing hints",
			MimeType:    "text/markdown",
		},
		{
			URI:         "kaboom://guide",
			Name:        "Kaboom Usage Guide",
			Description: "How to use Kaboom MCP tools for browser debugging",
			MimeType:    "text/markdown",
		},
		{
			URI:         "kaboom://quickstart",
			Name:        "Kaboom MCP Quickstart",
			Description: "Short, canonical MCP call examples and workflows",
			MimeType:    "text/markdown",
		},
	}
}

func ResourceTemplates() []any {
	return []any{
		map[string]any{
			"uriTemplate": "kaboom://playbook/{capability}/{level}",
			"name":        "Kaboom Capability Playbook",
			"description": "On-demand, token-efficient playbooks. Start with quick; use full for deep workflows.",
			"mimeType":    "text/markdown",
		},
		map[string]any{
			"uriTemplate": "kaboom://demo/{name}",
			"name":        "Kaboom Demo Script",
			"description": "Demo scripts for websockets, annotations, recording, and dependency vetting",
			"mimeType":    "text/markdown",
		},
	}
}
