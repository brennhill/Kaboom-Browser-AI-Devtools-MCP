// Purpose: Package configure — MCP tool definition and property groups for the configure tool.
// Why: The configure parameter surface is large enough that its property groups need their own package.
// Docs: docs/features/feature/config-profiles/index.md

/*
Package configure defines the configure tool's MCP schema.

Layout:
  - tool.go: the tool definition (name, description, input schema)
  - properties.go: merges the core and runtime property groups
  - properties_core.go: dispatch properties (what, action, mode, tool, ...)
  - properties_runtime.go: runtime properties (buffer, streaming, sequences, ...)
*/
package configure

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"

// ToolSchema returns the MCP tool definition for the configure tool.
func ToolSchema() mcp.MCPTool {
	return mcp.MCPTool{
		Name:        "configure",
		Description: "Session settings and utilities.\n\nSession: store, load, clear, telemetry, security_mode.\nDiagnostics: health, doctor, restart, audit_log, describe_capabilities, report_issue.\nRecording: event_recording_start/stop, playback, log_diff, network_recording.\nSequences: save/get/list/delete/replay_sequence.\nNoise & streaming: noise_rule, streaming, action_jitter.\nTesting: test_boundary_start/end.\nQuality: setup_quality_gates.\nHelp: tutorial, diff_sessions.\n\nDiscovery: describe_capabilities — list available modes and per-mode parameters for any tool. Filter with tool and mode params, e.g. configure(what:'describe_capabilities', tool:'observe', mode:'errors') returns only the params relevant to that mode.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": toolProperties(),
			"required":   []string{"what"},
		},
	}
}
