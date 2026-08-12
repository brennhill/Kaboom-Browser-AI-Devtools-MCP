// element_projection.go — Trims element listings to what a caller acts on.
//
// Shared by interact (list_interactive, explore_page) and observe
// (page_inventory), which all return the same element shape and were all
// paying full price for it.

package toolresp

import (
	"encoding/json"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// leanElementFields are the fields an agent needs to decide what to act on and
// then to address it. Everything else is diagnostic detail that costs tokens on
// every element of every listing.
//
// Measured before this projection: 279 bytes per element, against roughly 22
// for the equivalent line from chrome-devtools-mcp. The bounding box alone was
// a fifth of each element, and no caller choosing what to click reads it.
//
// index stays because interact accepts it as a targeting parameter, so removing
// it would break a workflow rather than just slim a payload. selector stays
// because it is portable — it works in Playwright, in the console and in
// generated tests, which an opaque handle does not.
func isLeanElementField(field string) bool {
	switch field {
	case "element_id", "element_type", "label", "selector", "index", "value", "placeholder":
		return true
	default:
		return false
	}
}

// elementCollectionKeys are the response fields that hold element listings.
// list_interactive and explore_page name theirs differently. A function rather
// than a package variable so the package keeps no mutable global state.
func elementCollectionKeys() []string { return []string{"elements", "interactive_elements"} }

// projectElementCollections trims element listings to the fields a caller acts
// on, unless verbose was requested.
//
// This runs AFTER enrichment, never before: enrichExploreWithMenus reads bbox,
// landmark_tag and index to group menu items, so projecting first would silently
// disable menu discovery rather than merely shrink the response.
func projectElementCollections(data map[string]any, verbose bool) map[string]any {
	if verbose {
		return data
	}
	// Browser-mediated modes answer with a lifecycle envelope and put the payload
	// under result, so the collection is nested for every real list_interactive
	// call. Projecting only the top level silently did nothing in production
	// while passing against a flat test fixture.
	projectElementCollectionsIn(data)
	if inner, ok := data["result"].(map[string]any); ok {
		projectElementCollectionsIn(inner)
	}
	return data
}

func projectElementCollectionsIn(data map[string]any) {
	for _, key := range elementCollectionKeys() {
		raw, ok := data[key].([]any)
		if !ok {
			continue
		}
		for i, entry := range raw {
			element, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			lean := make(map[string]any, 7)
			for field, value := range element {
				if isLeanElementField(field) {
					lean[field] = value
				}
			}
			// Only the exception earns its bytes: visible=true is the common
			// case and tells the caller nothing it did not already assume.
			if visible, ok := element["visible"].(bool); ok && !visible {
				lean["visible"] = false
			}
			raw[i] = lean
		}
	}
}

// ProjectElementsInResponse applies the lean element projection to a tool
// response. Non-JSON and unparseable bodies pass through untouched: a response
// this cannot read is one it must not corrupt.
func ProjectElementsInResponse(resp mcp.JSONRPCResponse, verbose bool) mcp.JSONRPCResponse {
	if verbose {
		return resp
	}
	return mcp.MutateToolResult(resp, func(r *mcp.MCPToolResult) {
		if len(r.Content) == 0 || r.Content[0].Type != "text" {
			return
		}
		text := r.Content[0].Text
		jsonStart := strings.Index(text, "{")
		if jsonStart < 0 {
			return
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(text[jsonStart:]), &data); err != nil {
			return
		}
		projected, err := json.Marshal(projectElementCollections(data, false))
		if err != nil {
			return
		}
		r.Content[0].Text = text[:jsonStart] + string(projected)
	})
}
