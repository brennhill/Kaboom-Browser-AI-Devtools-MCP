// capabilities.go — Implements configure capabilities introspection response shaping.
// Why: Keeps capability filtering/reporting isolated from configure routing logic.

package configurehandler

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
)

// HandleDescribeCapabilities returns machine-readable tool metadata derived from ToolsList().
// Supports filtering by tool name and mode to reduce payload size.
// When summary=true, returns only tool name -> { description, dispatch_param, modes }.
func HandleDescribeCapabilities(d Deps, req mcp.JSONRPCRequest, args json.RawMessage, version string) mcp.JSONRPCResponse {
	var params struct {
		Summary bool   `json:"summary"`
		Tool    string `json:"tool"`
		Mode    string `json:"mode"`
	}
	lenientUnmarshal(args, &params)

	tools := d.ToolsList()

	// mode without tool is an error.
	if params.Mode != "" && params.Tool == "" {
		return fail(req, mcp.ErrInvalidParam,
			"'mode' requires 'tool' to be set",
			"Add the 'tool' parameter to filter by mode",
			mcp.WithParam("tool"))
	}

	// Filter by tool + mode.
	if params.Tool != "" {
		toolCap, ok := cfg.BuildCapabilitiesForTool(tools, params.Tool)
		if !ok {
			validNames := make([]string, 0, len(tools))
			for _, t := range tools {
				validNames = append(validNames, t.Name)
			}
			sort.Strings(validNames)
			return fail(req, mcp.ErrInvalidParam,
				"Unknown tool: "+params.Tool,
				"Use a valid tool name",
				mcp.WithParam("tool"),
				mcp.WithHint("Valid tools: "+strings.Join(validNames, ", ")))
		}

		if params.Mode != "" {
			modeCap, ok := cfg.FilterToolByMode(toolCap, params.Tool, params.Mode)
			if !ok {
				modes, _ := toolCap["modes"].([]string)
				if modes == nil {
					if modesAny, ok := toolCap["modes"].([]any); ok {
						for _, m := range modesAny {
							if s, ok := m.(string); ok {
								modes = append(modes, s)
							}
						}
					}
				}
				return fail(req, mcp.ErrInvalidParam,
					"Unknown mode '"+params.Mode+"' for tool '"+params.Tool+"'",
					"Use a valid mode for this tool",
					mcp.WithParam("mode"),
					mcp.WithHint("Valid modes: "+strings.Join(modes, ", ")))
			}
			return succeed(req, "Capabilities", modeCap)
		}

		result := map[string]any{
			"version":          version,
			"protocol_version": "2024-11-05",
			"tools":            map[string]any{params.Tool: toolCap},
		}
		if examples := d.GetToolModuleExamples(params.Tool); examples != nil {
			result["examples"] = examples
		}
		return succeed(req, "Capabilities", result)
	}

	// Full or summary response (no filters).
	var toolsMap map[string]any
	if params.Summary {
		toolsMap = cfg.BuildCapabilitiesSummary(tools)
	} else {
		toolsMap = cfg.BuildCapabilitiesMap(tools)
	}

	return succeed(req, "Capabilities", map[string]any{
		"version":          version,
		"protocol_version": "2024-11-05",
		"tools":            toolsMap,
		"deprecated":       []string{},
	})
}
