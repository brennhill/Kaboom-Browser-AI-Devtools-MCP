// site_menus.go — observe(what:"site_menus") handler.
// Why: Gives AI agents structured menu discovery without requiring landmark markup.
// Dispatches list_interactive to the extension, then runs the 3-layer menu heuristic in Go.

package observehandler

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/menus"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// HandleSiteMenus handles observe(what="site_menus").
func HandleSiteMenus(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Summary bool `json:"summary"`
	}
	if len(args) > 0 {
		mcp.LenientUnmarshal(args, &params)
	}

	// Dispatch list_interactive to the extension to get all interactables with bboxes.
	queryArgs := mcp.SafeMarshal(map[string]any{
		"what":         "list_interactive",
		"visible_only": true,
	}, "{}")

	correlationID := newCorrelationID("site_menus")
	query := queries.PendingQuery{
		Type:          "dom_action",
		Params:        queryArgs,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := d.EnqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	// Wait synchronously for the extension result.
	resp := d.MaybeWaitForCommand(req, correlationID, args, "site_menus queued")

	// Check if we got the result or it's still pending.
	var toolResult mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &toolResult); err != nil || toolResult.IsError {
		return resp
	}

	// Parse the list_interactive result to extract elements.
	rawElements := parseListInteractiveResult(toolResult)
	if rawElements == nil {
		return succeed(req, "Site menus", menus.Result{
			Main: []menus.MenuGroup{}, Sidebar: []menus.MenuGroup{},
			Footer: []menus.MenuGroup{}, Other: []menus.MenuGroup{},
			Ungrouped: []menus.MenuItem{},
		})
	}

	cfg := menus.DefaultConfig()
	result := menus.Discover(rawElements, cfg)

	if params.Summary {
		return succeed(req, "Site menus summary", buildSiteMenusSummary(result))
	}
	return succeed(req, "Site menus", result)
}

// parseListInteractiveResult extracts raw elements from a list_interactive tool response.
func parseListInteractiveResult(result mcp.MCPToolResult) []menus.RawElement {
	if len(result.Content) == 0 {
		return nil
	}

	// The result text is: summary line\n{json}
	text := result.Content[0].Text
	jsonStart := -1
	for i := 0; i < len(text); i++ {
		if text[i] == '{' {
			jsonStart = i
			break
		}
	}
	if jsonStart < 0 {
		return nil
	}

	var data struct {
		Elements []struct {
			Index        int        `json:"index"`
			Tag          string     `json:"tag"`
			Type         string     `json:"type"`
			ElementType  string     `json:"element_type"`
			Label        string     `json:"label"`
			Role         string     `json:"role"`
			Placeholder  string     `json:"placeholder"`
			Visible      bool       `json:"visible"`
			BBox         menus.BBox `json:"bbox"`
			LandmarkTag  string     `json:"landmark_tag"`
			LandmarkRole string     `json:"landmark_role"`
			Href         string     `json:"href"`
			InOverlay    bool       `json:"in_overlay"`
		} `json:"elements"`
	}
	if err := json.Unmarshal([]byte(text[jsonStart:]), &data); err != nil {
		return nil
	}

	out := make([]menus.RawElement, len(data.Elements))
	for i, el := range data.Elements {
		out[i] = menus.RawElement{
			Text:       el.Label,
			Href:       el.Href,
			Tag:        el.Tag,
			Type:       el.Type,
			Role:       el.Role,
			BBox:       el.BBox,
			ParentTag:  el.LandmarkTag,
			ParentRole: el.LandmarkRole,
			Visible:    el.Visible,
			Index:      el.Index,
		}
	}
	return out
}

func buildSiteMenusSummary(result menus.Result) map[string]any {
	countItems := func(groups []menus.MenuGroup) int {
		n := 0
		for _, g := range groups {
			n += len(g.Items)
		}
		return n
	}
	return map[string]any{
		"main_count":      countItems(result.Main),
		"sidebar_count":   countItems(result.Sidebar),
		"footer_count":    countItems(result.Footer),
		"other_count":     countItems(result.Other),
		"ungrouped_count": len(result.Ungrouped),
		"main_groups":     len(result.Main),
		"sidebar_groups":  len(result.Sidebar),
		"footer_groups":   len(result.Footer),
		"other_groups":    len(result.Other),
	}
}
