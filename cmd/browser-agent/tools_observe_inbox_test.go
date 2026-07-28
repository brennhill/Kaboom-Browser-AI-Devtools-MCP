// tools_observe_inbox_test.go — Tests for observe inbox handler and push piggyback.
package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolobserve"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

func newPushTestToolHandler(inbox *push.PushInbox) *ToolHandler {
	s := &Server{pushInbox: inbox}
	mcp := &MCPHandler{server: s}
	return &ToolHandler{MCPHandler: mcp}
}

// piggybackTestResponse creates a base tool response, runs appendPushPiggyback, and returns the parsed result.
func piggybackTestResponse(t *testing.T, h *ToolHandler, baseText string) mcp.MCPToolResult {
	t.Helper()
	result := mcp.MCPToolResult{Content: []mcp.MCPContentBlock{{Type: "text", Text: baseText}}}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: json.RawMessage(resultJSON)}
	out := toolobserve.AppendPushPiggyback(buildObserveLocalDeps(h), resp)
	var outResult mcp.MCPToolResult
	if err := json.Unmarshal(out.Result, &outResult); err != nil {
		t.Fatal(err)
	}
	return outResult
}

func TestToolObserveInbox_Empty(t *testing.T) {
	h := newPushTestToolHandler(push.NewPushInbox(10))

	resp := toolobserve.HandleInbox(buildObserveLocalDeps(h), mcp.JSONRPCRequest{ID: json.RawMessage(`1`)}, nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
}

func TestToolObserveInbox_WithEvents(t *testing.T) {
	inbox := push.NewPushInbox(10)
	inbox.Enqueue(push.PushEvent{
		ID:        "test-1",
		Type:      "chat",
		Message:   "hello",
		Timestamp: time.Now(),
	})
	inbox.Enqueue(push.PushEvent{
		ID:        "test-2",
		Type:      "screenshot",
		Timestamp: time.Now(),
	})

	h := newPushTestToolHandler(inbox)
	resp := toolobserve.HandleInbox(buildObserveLocalDeps(h), mcp.JSONRPCRequest{ID: json.RawMessage(`2`)}, nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	// After drain, inbox should be empty
	if inbox.Len() != 0 {
		t.Fatal("inbox should be empty after drain")
	}
}

func TestToolObserveInbox_NilInbox(t *testing.T) {
	h := newPushTestToolHandler(nil)
	resp := toolobserve.HandleInbox(buildObserveLocalDeps(h), mcp.JSONRPCRequest{ID: json.RawMessage(`3`)}, nil)
	if resp.Error != nil {
		t.Fatal("nil inbox should not error")
	}
}

func TestAppendPushPiggyback_Empty(t *testing.T) {
	h := newPushTestToolHandler(push.NewPushInbox(10))
	outResult := piggybackTestResponse(t, h, "hello")
	// Empty inbox should not add piggyback
	if len(outResult.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(outResult.Content))
	}
}

func TestAppendPushPiggyback_WithEvents(t *testing.T) {
	inbox := push.NewPushInbox(10)
	inbox.Enqueue(push.PushEvent{ID: "1", Type: "chat", Message: "pending"})

	h := newPushTestToolHandler(inbox)
	outResult := piggybackTestResponse(t, h, "hello")
	if len(outResult.Content) != 2 {
		t.Fatalf("expected 2 content blocks (original + piggyback), got %d", len(outResult.Content))
	}
}

func TestAppendPushPiggyback_CapsScreenshotsToOne(t *testing.T) {
	inbox := push.NewPushInbox(10)
	inbox.Enqueue(push.PushEvent{ID: "ss-1", Type: "screenshot", PageURL: "https://a.com", ScreenshotB64: "old", TabID: 1})
	inbox.Enqueue(push.PushEvent{ID: "ss-2", Type: "screenshot", PageURL: "https://b.com", ScreenshotB64: "new", TabID: 2})

	h := newPushTestToolHandler(inbox)
	outResult := piggybackTestResponse(t, h, "hello")

	// Expect: original text + summary text + label text + image = 4 blocks
	imageCount := 0
	for _, block := range outResult.Content {
		if block.Type == "image" {
			imageCount++
			if block.Data != "new" {
				t.Fatal("expected only the most recent screenshot")
			}
		}
	}
	if imageCount != 1 {
		t.Fatalf("expected exactly 1 image (most recent), got %d", imageCount)
	}
}

func TestAppendPushPiggyback_NonScreenshotEventsPassThrough(t *testing.T) {
	inbox := push.NewPushInbox(10)
	inbox.Enqueue(push.PushEvent{ID: "c-1", Type: "chat", Message: "msg1"})
	inbox.Enqueue(push.PushEvent{ID: "ss-1", Type: "screenshot", PageURL: "https://a.com", ScreenshotB64: "img1"})
	inbox.Enqueue(push.PushEvent{ID: "c-2", Type: "chat", Message: "msg2"})
	inbox.Enqueue(push.PushEvent{ID: "ss-2", Type: "screenshot", PageURL: "https://b.com", ScreenshotB64: "img2"})

	h := newPushTestToolHandler(inbox)
	outResult := piggybackTestResponse(t, h, "base")

	// Count chat blocks (should be 2 — all non-screenshot events pass through)
	chatCount := 0
	imageCount := 0
	for _, block := range outResult.Content {
		if block.Type == "text" && len(block.Text) > 0 {
			if contains(block.Text, "_push_chat:") {
				chatCount++
			}
		}
		if block.Type == "image" {
			imageCount++
		}
	}
	if chatCount != 2 {
		t.Fatalf("expected 2 chat events to pass through, got %d", chatCount)
	}
	if imageCount != 1 {
		t.Fatalf("expected 1 screenshot (capped), got %d", imageCount)
	}
}

func TestAppendPushPiggyback_SkippedCountInSummary(t *testing.T) {
	inbox := push.NewPushInbox(10)
	inbox.Enqueue(push.PushEvent{ID: "ss-1", Type: "screenshot", PageURL: "https://a.com", ScreenshotB64: "a", TabID: 1})
	inbox.Enqueue(push.PushEvent{ID: "ss-2", Type: "screenshot", PageURL: "https://b.com", ScreenshotB64: "b", TabID: 2})
	inbox.Enqueue(push.PushEvent{ID: "ss-3", Type: "screenshot", PageURL: "https://c.com", ScreenshotB64: "c", TabID: 3})

	h := newPushTestToolHandler(inbox)
	outResult := piggybackTestResponse(t, h, "base")

	// Look for summary mentioning skipped screenshots
	found := false
	for _, block := range outResult.Content {
		if block.Type == "text" && contains(block.Text, "2 earlier") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected summary mentioning 2 earlier screenshots skipped")
	}
}

func TestAppendPushPiggyback_SingleScreenshotNoSkipSummary(t *testing.T) {
	inbox := push.NewPushInbox(10)
	inbox.Enqueue(push.PushEvent{ID: "ss-1", Type: "screenshot", PageURL: "https://a.com", ScreenshotB64: "data"})

	h := newPushTestToolHandler(inbox)
	outResult := piggybackTestResponse(t, h, "base")

	for _, block := range outResult.Content {
		if block.Type == "text" && contains(block.Text, "earlier") {
			t.Fatal("single screenshot should not produce a skip summary")
		}
	}

	imageCount := 0
	for _, block := range outResult.Content {
		if block.Type == "image" {
			imageCount++
		}
	}
	if imageCount != 1 {
		t.Fatalf("expected 1 image, got %d", imageCount)
	}
}

func TestAppendPushPiggyback_NilInbox(t *testing.T) {
	h := newPushTestToolHandler(nil)

	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: json.RawMessage(`{}`)}
	out := toolobserve.AppendPushPiggyback(buildObserveLocalDeps(h), resp)
	if string(out.Result) != `{}` {
		t.Fatal("nil inbox piggyback should be no-op")
	}
}
