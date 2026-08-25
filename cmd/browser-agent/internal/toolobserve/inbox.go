// inbox.go — observe(what:"inbox") handler and push piggyback.

package toolobserve

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

// HandleInbox drains the push inbox and returns pending events.
func HandleInbox(d Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	inbox := d.Inbox
	if inbox == nil {
		return mcp.Succeed(req, "Push inbox empty", map[string]any{
			"events": []any{},
			"count":  0,
		})
	}

	events := inbox.DrainAll()
	if events == nil {
		return mcp.Succeed(req, "Push inbox empty", map[string]any{
			"events": []any{},
			"count":  0,
		})
	}

	return mcp.Succeed(req, "Push inbox drained", map[string]any{
		"events": events,
		"count":  len(events),
	})
}

// AppendPushPiggyback drains the push inbox and inlines events into any tool response.
// Screenshots are delivered as image content blocks so the LLM sees them immediately.
func AppendPushPiggyback(d Deps, resp mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	inbox := d.Inbox
	if inbox == nil {
		return resp
	}

	events := inbox.DrainAll()
	if len(events) == 0 {
		return resp
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}

	latestScreenshot, screenshotCount, otherEvents := partitionPushEvents(events)

	for _, ev := range otherEvents {
		appendPushEventContent(&result, ev)
	}

	appendScreenshotContent(&result, latestScreenshot, screenshotCount)

	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

// partitionPushEvents separates screenshots from other events; only the most
// recent screenshot is retained.
func partitionPushEvents(events []push.PushEvent) (*push.PushEvent, int, []push.PushEvent) {
	var latestScreenshot *push.PushEvent
	screenshotCount := 0
	var otherEvents []push.PushEvent
	for i := range events {
		if events[i].Type == "screenshot" {
			screenshotCount++
			latestScreenshot = &events[i]
		} else {
			otherEvents = append(otherEvents, events[i])
		}
	}
	return latestScreenshot, screenshotCount, otherEvents
}

func appendPushEventContent(result *mcp.MCPToolResult, ev push.PushEvent) {
	switch ev.Type {
	case "annotations":
		label := fmt.Sprintf("\n\n_push_annotations: from %s", ev.PageURL)
		if ev.AnnotSession != "" {
			label += fmt.Sprintf(" (session: %s)", ev.AnnotSession)
		}
		if len(ev.Annotations) > 0 {
			label += "\n" + string(ev.Annotations)
		}
		result.Content = append(result.Content, mcp.MCPContentBlock{Type: "text", Text: label})
	case "chat":
		result.Content = append(result.Content, mcp.MCPContentBlock{
			Type: "text",
			Text: fmt.Sprintf("\n\n_push_chat: %s\n[from: %s]", ev.Message, ev.PageURL),
		})
	default:
		result.Content = append(result.Content, mcp.MCPContentBlock{
			Type: "text",
			Text: fmt.Sprintf("\n\n_push_%s: event from %s", ev.Type, ev.PageURL),
		})
	}
}

func appendScreenshotContent(result *mcp.MCPToolResult, latestScreenshot *push.PushEvent, screenshotCount int) {
	if latestScreenshot == nil {
		return
	}
	if screenshotCount > 1 {
		result.Content = append(result.Content, mcp.MCPContentBlock{
			Type: "text",
			Text: fmt.Sprintf("\n\n_push_screenshot: %d earlier screenshots skipped (showing most recent only)", screenshotCount-1),
		})
	}
	label := fmt.Sprintf("\n\n_push_screenshot: captured from %s", latestScreenshot.PageURL)
	if latestScreenshot.Note != "" {
		label += " — " + latestScreenshot.Note
	}
	result.Content = append(result.Content, mcp.MCPContentBlock{Type: "text", Text: label})
	if latestScreenshot.ScreenshotB64 != "" {
		result.Content = append(result.Content, mcp.MCPContentBlock{
			Type:     "image",
			Data:     latestScreenshot.ScreenshotB64,
			MimeType: "image/jpeg",
		})
	}
}
