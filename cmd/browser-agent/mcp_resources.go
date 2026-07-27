// Purpose: Declares MCP resource URIs (capabilities, guide, quickstart) and URI templates (playbooks, demos) for client discovery.
// Why: Exposes token-efficient documentation resources that MCP clients can read on demand.

package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/playbooks"

func resolveResourceContent(uri string) (string, string, bool) {
	return playbooks.ResolveResourceContent(uri)
}

func tutorialFailureRecoveryPlaybooks() map[string]any {
	return playbooks.TutorialFailureRecoveryPlaybooks()
}

func mcpResources() []MCPResource {
	return []MCPResource{
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

func mcpResourceTemplates() []any {
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
