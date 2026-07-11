// Purpose: Handles explore_page — a single compound query returning screenshot, interactive elements, metadata, text, and links.
// Why: Reduces agent round-trips by combining page discovery signals into one call instead of multiple observe/interact calls.
// Docs: docs/features/feature/interact-explore/index.md

package interacthandler

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/menus"
)

// handleExplorePage handles interact(what="explore_page").
// Creates a pending query for the extension to return combined page metadata,
// interactive elements, readable text, and navigation links in one response.
// If url is provided, the extension navigates first before collecting data.
// Screenshot is appended server-side after the extension returns.
// Post-processes the result to separate menus from ungrouped page elements.
func (h *InteractActionHandler) HandleExplorePage(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL         string `json:"url,omitempty"`
		TabID       int    `json:"tab_id,omitempty"`
		VisibleOnly bool   `json:"visible_only,omitempty"`
		Limit       int    `json:"limit,omitempty"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	// Validate URL scheme — only http/https allowed (#341 security review)
	if params.URL != "" {
		parsed, err := url.Parse(params.URL)
		if err != nil || parsed.Scheme == "" {
			return fail(req, ErrInvalidParam, "Invalid URL: "+params.URL, "Provide a valid http or https URL", withParam("url"))
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fail(req, ErrInvalidParam, "Only http and https URLs are allowed, got: "+parsed.Scheme, "Use an http or https URL", withParam("url"))
		}
	}

	resp := h.newCommand("explore_page").
		correlationPrefix("explore_page").
		reason("explore_page").
		queryType("explore_page").
		queryParams(args).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		recordAction("explore_page", params.URL, nil).
		queuedMessage("Explore page queued").
		execute(req, args)

	// Post-process: enrich with structured site_menus if the command completed
	if !isErrorResponse(resp) && !isResponseQueued(resp) {
		resp = enrichExploreWithMenus(resp)
		resp = h.AppendScreenshotToResponse(resp, req)
	}

	return resp
}

// enrichExploreWithMenus post-processes an explore_page response to add a
// site_menus section. Elements claimed by menus are removed from the
// interactive_elements list so there is no overlap.
func enrichExploreWithMenus(resp JSONRPCResponse) JSONRPCResponse {
	return mutateToolResult(resp, func(r *MCPToolResult) {
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

		elementsRaw, ok := data["interactive_elements"].([]any)
		if !ok || len(elementsRaw) == 0 {
			return
		}

		// Parse elements into RawElement for the heuristic
		rawElements := make([]menus.RawElement, 0, len(elementsRaw))
		for _, eRaw := range elementsRaw {
			eMap, ok := eRaw.(map[string]any)
			if !ok {
				continue
			}
			bbox := menus.BBox{}
			if bboxMap, ok := eMap["bbox"].(map[string]any); ok {
				bbox.X, _ = bboxMap["x"].(float64)
				bbox.Y, _ = bboxMap["y"].(float64)
				bbox.Width, _ = bboxMap["width"].(float64)
				bbox.Height, _ = bboxMap["height"].(float64)
			}
			idx, _ := eMap["index"].(float64)
			label, _ := eMap["label"].(string)
			tag, _ := eMap["tag"].(string)
			role, _ := eMap["role"].(string)
			href, _ := eMap["href"].(string)
			visible := true
			if v, ok := eMap["visible"].(bool); ok {
				visible = v
			}
			landmarkTag, _ := eMap["landmark_tag"].(string)
			landmarkRole, _ := eMap["landmark_role"].(string)
			rawElements = append(rawElements, menus.RawElement{
				Text:       label,
				Href:       href,
				Tag:        tag,
				Role:       role,
				BBox:       bbox,
				ParentTag:  landmarkTag,
				ParentRole: landmarkRole,
				Visible:    visible,
				Index:      int(idx),
			})
		}

		cfg := menus.DefaultConfig()
		menuResult := menus.Discover(rawElements, cfg)

		claimedIndices := menuResult.ClaimedIndices()

		// Filter interactive_elements to remove menu items
		if len(claimedIndices) > 0 {
			filtered := make([]any, 0, len(elementsRaw))
			for _, eRaw := range elementsRaw {
				eMap, ok := eRaw.(map[string]any)
				if !ok {
					filtered = append(filtered, eRaw)
					continue
				}
				idx, _ := eMap["index"].(float64)
				if !claimedIndices[int(idx)] {
					filtered = append(filtered, eRaw)
				}
			}
			data["interactive_elements"] = filtered
			if count, ok := data["interactive_count"].(float64); ok {
				data["interactive_count"] = count - float64(len(claimedIndices))
			}
		}

		data["site_menus"] = menuResult

		dataJSON, err := json.Marshal(data)
		if err != nil {
			return
		}
		r.Content[0].Text = text[:jsonStart] + string(dataJSON)
	})
}
