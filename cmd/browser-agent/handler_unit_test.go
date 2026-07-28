// Purpose: Unit tests for browser-agent handler logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// handler_unit_test.go — Core MCP handler unit tests: request routing, resource methods,
// tool dispatch, warnings, rate limiting, and shared test helpers.
package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type testLimiter struct {
	allowed bool
}

func (l testLimiter) Allow() bool { return l.allowed }

type testRedactor struct {
	replacement json.RawMessage
}

func (r testRedactor) Redact(input string) string {
	return input // no-op for existing tests
}

func (r testRedactor) RedactJSON(_ json.RawMessage) json.RawMessage {
	return r.replacement
}

func (r testRedactor) RedactMapValues(data map[string]any) map[string]any {
	return data // no-op for existing tests
}

type fakeToolHandlerForMCP struct {
	cap      *capture.Capture
	limiter  RateLimiter
	redactor RedactionEngine
	tools    []mcp.MCPTool
	handleFn func(req mcp.JSONRPCRequest, name string, arguments json.RawMessage) (mcp.JSONRPCResponse, bool)
}

func (f *fakeToolHandlerForMCP) GetCapture() *capture.Capture { return f.cap }
func (f *fakeToolHandlerForMCP) GetToolCallLimiter() RateLimiter {
	return f.limiter
}
func (f *fakeToolHandlerForMCP) GetRedactionEngine() RedactionEngine {
	return f.redactor
}
func (f *fakeToolHandlerForMCP) ToolsList() []mcp.MCPTool { return f.tools }
func (f *fakeToolHandlerForMCP) HandleToolCall(req mcp.JSONRPCRequest, name string, arguments json.RawMessage) (mcp.JSONRPCResponse, bool) {
	if f.handleFn == nil {
		return mcp.JSONRPCResponse{}, false
	}
	return f.handleFn(req, name, arguments)
}

func mustDecodeJSON[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return out
}

func mustTelemetryMetadata(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("json.Unmarshal(MCPToolResult) error = %v", err)
	}
	if result.Metadata == nil {
		t.Fatal("result metadata missing")
	}
	return result.Metadata
}

func mustTelemetrySummary(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	metadata := mustTelemetryMetadata(t, raw)
	summary, ok := metadata["telemetry_summary"].(map[string]any)
	if !ok {
		t.Fatalf("telemetry_summary missing or wrong type: %#v", metadata["telemetry_summary"])
	}
	return summary
}

func telemetrySummaryIfPresent(t *testing.T, raw json.RawMessage) (map[string]any, bool) {
	t.Helper()
	metadata := mustTelemetryMetadata(t, raw)
	summary, ok := metadata["telemetry_summary"].(map[string]any)
	if !ok {
		return nil, false
	}
	return summary, true
}

func mustTelemetryInt(t *testing.T, summary map[string]any, key string) int64 {
	t.Helper()
	v, ok := summary[key]
	if !ok {
		t.Fatalf("telemetry_summary[%q] missing", key)
	}
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("telemetry_summary[%q] type = %T, want number", key, v)
	}
	return int64(f)
}

func mustTelemetryChanged(t *testing.T, raw json.RawMessage) bool {
	t.Helper()
	metadata := mustTelemetryMetadata(t, raw)
	v, ok := metadata["telemetry_changed"]
	if !ok {
		t.Fatal("telemetry_changed missing")
	}
	changed, ok := v.(bool)
	if !ok {
		t.Fatalf("telemetry_changed type = %T, want bool", v)
	}
	return changed
}

func TestMCPHandlerHandleRequestCorePaths(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(nil, "v1.2.3")

	if resp := h.HandleRequest(mcp.JSONRPCRequest{JSONRPC: "2.0", Method: "ping"}); resp != nil {
		t.Fatalf("notification without ID should return nil, got %+v", resp)
	}
	notifyRequest := h.HandleRequest(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "notifications/initialized"})
	if notifyRequest == nil || notifyRequest.Error == nil || notifyRequest.Error.Code != -32601 {
		t.Fatalf("notifications/* request with id should return method-not-found, got %+v", notifyRequest)
	}

	unknown := h.HandleRequest(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "not/method"})
	if unknown == nil || unknown.Error == nil || unknown.Error.Code != -32601 {
		t.Fatalf("unknown method response = %+v, want method-not-found error", unknown)
	}

	initReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05"}`),
	}
	initResp := h.HandleRequest(initReq)
	if initResp == nil || initResp.Error != nil {
		t.Fatalf("initialize response = %+v, want success", initResp)
	}
	initData := mustDecodeJSON[mcp.MCPInitializeResult](t, initResp.Result)
	if initData.ProtocolVersion != "2024-11-05" {
		t.Fatalf("ProtocolVersion = %q, want %q", initData.ProtocolVersion, "2024-11-05")
	}
	if initData.ServerInfo.Version != "v1.2.3" {
		t.Fatalf("server version = %q, want v1.2.3", initData.ServerInfo.Version)
	}

	initFallback := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2023-01-01"}`),
	})
	initFallbackData := mustDecodeJSON[mcp.MCPInitializeResult](t, initFallback.Result)
	if initFallbackData.ProtocolVersion != "2025-06-18" {
		t.Fatalf("fallback protocol = %q, want latest supported version 2025-06-18", initFallbackData.ProtocolVersion)
	}

	initLatest := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2025-06-18"}`),
	})
	initLatestData := mustDecodeJSON[mcp.MCPInitializeResult](t, initLatest.Result)
	if initLatestData.ProtocolVersion != "2025-06-18" {
		t.Fatalf("latest protocol = %q, want %q", initLatestData.ProtocolVersion, "2025-06-18")
	}
}

func TestMCPHandlerResourceAndToolMethods(t *testing.T) {
	origLastNotify := updateNotifyLastShown
	updateNotifyLastShown = time.Now()
	t.Cleanup(func() { updateNotifyLastShown = origLastNotify })

	h := NewMCPHandler(nil, "v-test")
	th := &fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
		tools: []mcp.MCPTool{
			{Name: "observe"},
		},
		handleFn: func(req mcp.JSONRPCRequest, name string, _ json.RawMessage) (mcp.JSONRPCResponse, bool) {
			if name != "observe" {
				return mcp.JSONRPCResponse{}, false
			}
			return mcp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"ok":true,"secret":"value"}`),
			}, true
		},
		redactor: testRedactor{replacement: json.RawMessage(`{"ok":true,"secret":"[REDACTED]"}`)},
	}
	h.SetToolHandler(th)

	resources := h.HandleRequest(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "resources/list"})
	if resources == nil || resources.Error != nil {
		t.Fatalf("resources/list response = %+v, want success", resources)
	}
	resourceData := mustDecodeJSON[mcp.MCPResourcesListResult](t, resources.Result)
	if len(resourceData.Resources) != 3 {
		t.Fatalf("resources/list result = %+v, want 3 resources", resourceData)
	}
	if resourceData.Resources[0].URI != "kaboom://capabilities" {
		t.Fatalf("resources/list first resource = %q, want kaboom://capabilities", resourceData.Resources[0].URI)
	}
	if resourceData.Resources[1].URI != "kaboom://guide" {
		t.Fatalf("resources/list second resource = %q, want kaboom://guide", resourceData.Resources[1].URI)
	}
	if resourceData.Resources[2].URI != "kaboom://quickstart" {
		t.Fatalf("resources/list third resource = %q, want kaboom://quickstart", resourceData.Resources[2].URI)
	}

	readCapabilities := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      40,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"kaboom://capabilities"}`),
	})
	if readCapabilities == nil || readCapabilities.Error != nil {
		t.Fatalf("resources/read capabilities response = %+v, want success", readCapabilities)
	}
	readCapabilitiesData := mustDecodeJSON[mcp.MCPResourcesReadResult](t, readCapabilities.Result)
	if len(readCapabilitiesData.Contents) != 1 || readCapabilitiesData.Contents[0].URI != "kaboom://capabilities" {
		t.Fatalf("resources/read capabilities result = %+v, want one capabilities content entry", readCapabilitiesData)
	}

	readInvalid := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "resources/read",
		Params:  json.RawMessage(`{`),
	})
	if readInvalid == nil || readInvalid.Error == nil || readInvalid.Error.Code != -32602 {
		t.Fatalf("resources/read invalid params response = %+v", readInvalid)
	}

	readNotFound := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"kaboom://missing"}`),
	})
	if readNotFound == nil || readNotFound.Error == nil || readNotFound.Error.Code != -32002 {
		t.Fatalf("resources/read not-found response = %+v", readNotFound)
	}

	readOK := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"kaboom://guide"}`),
	})
	if readOK == nil || readOK.Error != nil {
		t.Fatalf("resources/read response = %+v, want success", readOK)
	}
	readData := mustDecodeJSON[mcp.MCPResourcesReadResult](t, readOK.Result)
	if len(readData.Contents) != 1 || readData.Contents[0].URI != "kaboom://guide" {
		t.Fatalf("resources/read result = %+v, want one guide content entry", readData)
	}

	readQuickstart := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"kaboom://quickstart"}`),
	})
	if readQuickstart == nil || readQuickstart.Error != nil {
		t.Fatalf("resources/read quickstart response = %+v, want success", readQuickstart)
	}
	readQuickData := mustDecodeJSON[mcp.MCPResourcesReadResult](t, readQuickstart.Result)
	if len(readQuickData.Contents) != 1 || readQuickData.Contents[0].URI != "kaboom://quickstart" {
		t.Fatalf("resources/read quickstart result = %+v, want one quickstart content entry", readQuickData)
	}

	readDemo := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"kaboom://demo/ws"}`),
	})
	if readDemo == nil || readDemo.Error != nil {
		t.Fatalf("resources/read demo response = %+v, want success", readDemo)
	}
	readDemoData := mustDecodeJSON[mcp.MCPResourcesReadResult](t, readDemo.Result)
	if len(readDemoData.Contents) != 1 || readDemoData.Contents[0].URI != "kaboom://demo/ws" {
		t.Fatalf("resources/read demo result = %+v, want demo content entry", readDemoData)
	}

	readPlaybook := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      61,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"kaboom://playbook/performance/quick"}`),
	})
	if readPlaybook == nil || readPlaybook.Error != nil {
		t.Fatalf("resources/read playbook response = %+v, want success", readPlaybook)
	}
	readPlaybookData := mustDecodeJSON[mcp.MCPResourcesReadResult](t, readPlaybook.Result)
	if len(readPlaybookData.Contents) != 1 || readPlaybookData.Contents[0].URI != "kaboom://playbook/performance/quick" {
		t.Fatalf("resources/read playbook result = %+v, want playbook content entry", readPlaybookData)
	}

	readSecurityPlaybook := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      62,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"kaboom://playbook/security/full"}`),
	})
	if readSecurityPlaybook == nil || readSecurityPlaybook.Error != nil {
		t.Fatalf("resources/read security playbook response = %+v, want success", readSecurityPlaybook)
	}
	readSecurityPlaybookData := mustDecodeJSON[mcp.MCPResourcesReadResult](t, readSecurityPlaybook.Result)
	if len(readSecurityPlaybookData.Contents) != 1 || readSecurityPlaybookData.Contents[0].URI != "kaboom://playbook/security/full" {
		t.Fatalf("resources/read security playbook result = %+v, want security playbook content entry", readSecurityPlaybookData)
	}

	readAliasedPlaybook := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      63,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"kaboom://playbook/security_audit/quick"}`),
	})
	if readAliasedPlaybook == nil || readAliasedPlaybook.Error != nil {
		t.Fatalf("resources/read aliased playbook response = %+v, want success", readAliasedPlaybook)
	}
	readAliasedPlaybookData := mustDecodeJSON[mcp.MCPResourcesReadResult](t, readAliasedPlaybook.Result)
	if len(readAliasedPlaybookData.Contents) != 1 || readAliasedPlaybookData.Contents[0].URI != "kaboom://playbook/security/quick" {
		t.Fatalf("resources/read aliased playbook result = %+v, want canonical security/quick content entry", readAliasedPlaybookData)
	}

	readBareCapability := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      64,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"kaboom://playbook/security"}`),
	})
	if readBareCapability == nil || readBareCapability.Error != nil {
		t.Fatalf("resources/read bare capability response = %+v, want success defaulting to quick", readBareCapability)
	}
	readBareCapabilityData := mustDecodeJSON[mcp.MCPResourcesReadResult](t, readBareCapability.Result)
	if len(readBareCapabilityData.Contents) != 1 || readBareCapabilityData.Contents[0].URI != "kaboom://playbook/security/quick" {
		t.Fatalf("resources/read bare capability result = %+v, want canonical security/quick content entry", readBareCapabilityData)
	}

	readInvalidPlaybook := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      65,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"kaboom://playbook/nonexistent/quick"}`),
	})
	if readInvalidPlaybook == nil || readInvalidPlaybook.Error == nil || readInvalidPlaybook.Error.Code != -32002 {
		t.Fatalf("resources/read invalid playbook response = %+v, want -32002 error", readInvalidPlaybook)
	}

	readInvalidDemo := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      66,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri":"kaboom://demo/nonexistent"}`),
	})
	if readInvalidDemo == nil || readInvalidDemo.Error == nil || readInvalidDemo.Error.Code != -32002 {
		t.Fatalf("resources/read invalid demo response = %+v, want -32002 error", readInvalidDemo)
	}

	templates := h.HandleRequest(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 7, Method: "resources/templates/list"})
	if templates == nil || templates.Error != nil {
		t.Fatalf("resources/templates/list response = %+v, want success", templates)
	}

	toolsList := h.HandleRequest(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 6, Method: "tools/list"})
	if toolsList == nil || toolsList.Error != nil {
		t.Fatalf("tools/list response = %+v, want success", toolsList)
	}
	toolsData := mustDecodeJSON[mcp.MCPToolsListResult](t, toolsList.Result)
	if len(toolsData.Tools) != 1 || toolsData.Tools[0].Name != "observe" {
		t.Fatalf("tools/list result = %+v, want observe tool", toolsData)
	}

	callInvalid := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  json.RawMessage(`{`),
	})
	if callInvalid == nil || callInvalid.Error == nil || callInvalid.Error.Code != -32602 {
		t.Fatalf("tools/call invalid params response = %+v", callInvalid)
	}

	callUnknown := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"missing","arguments":{}}`),
	})
	if callUnknown == nil || callUnknown.Error == nil || callUnknown.Error.Code != -32601 {
		t.Fatalf("tools/call unknown response = %+v", callUnknown)
	}

	callObserve := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})
	if callObserve == nil || callObserve.Error != nil {
		t.Fatalf("tools/call handled response = %+v, want success", callObserve)
	}
	var redacted map[string]any
	if err := json.Unmarshal(callObserve.Result, &redacted); err != nil {
		t.Fatalf("json.Unmarshal(result) error = %v", err)
	}
	if redacted["secret"] != "[REDACTED]" {
		t.Fatalf("redacted result = %+v, expected secret to be redacted", redacted)
	}
}

func TestMCPHandler_AppendsServerWarningsToToolResponse(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "warnings.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	srv.AddWarning("state_dir_not_writable: test warning")

	h := NewMCPHandler(srv, "v-test")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
		handleFn: func(req mcp.JSONRPCRequest, name string, _ json.RawMessage) (mcp.JSONRPCResponse, bool) {
			if name != "observe" {
				return mcp.JSONRPCResponse{}, false
			}
			return mcp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mcp.TextResponse("ok"),
			}, true
		},
	})

	resp := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call response = %+v, want success", resp)
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("result unmarshal error = %v", err)
	}
	if len(result.Content) < 2 {
		t.Fatalf("expected warnings content block, got %d blocks", len(result.Content))
	}
	last := result.Content[len(result.Content)-1].Text
	if !strings.Contains(last, "_warnings:") {
		t.Fatalf("expected warnings content block, got %q", last)
	}

	// Warning should be one-shot.
	resp2 := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	})
	if resp2 == nil || resp2.Error != nil {
		t.Fatalf("second tools/call response = %+v, want success", resp2)
	}
	var result2 mcp.MCPToolResult
	if err := json.Unmarshal(resp2.Result, &result2); err != nil {
		t.Fatalf("second result unmarshal error = %v", err)
	}
	for _, block := range result2.Content {
		if strings.Contains(block.Text, "_warnings:") {
			t.Fatalf("warning should be one-shot, got %q", block.Text)
		}
	}
}

func TestMCPHandler_WarnsOnUnknownToolArguments(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "unknown-args-warning.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(srv.Close)

	h := NewMCPHandler(srv, "v-test")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
		tools: []mcp.MCPTool{
			{
				Name: "observe",
				InputSchema: map[string]any{
					"properties": map[string]any{
						"what":  map[string]any{"type": "string"},
						"limit": map[string]any{"type": "number"},
					},
				},
			},
		},
		handleFn: func(req mcp.JSONRPCRequest, name string, _ json.RawMessage) (mcp.JSONRPCResponse, bool) {
			if name != "observe" {
				return mcp.JSONRPCResponse{}, false
			}
			return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.TextResponse("ok")}, true
		},
	})

	resp := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"errors","limit":10,"typo_field":true}}`),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call response = %+v, want success", resp)
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("result unmarshal error = %v", err)
	}
	if len(result.Content) < 2 {
		t.Fatalf("expected warning content block, got %d blocks", len(result.Content))
	}

	last := result.Content[len(result.Content)-1].Text
	if !strings.Contains(last, "_warnings:") || !strings.Contains(last, "typo_field") {
		t.Fatalf("expected unknown-parameter warning, got %q", last)
	}
}

func TestMCPHandler_DoesNotWarnOnKnownToolArguments(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "known-args-no-warning.jsonl")
	srv, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(srv.Close)

	h := NewMCPHandler(srv, "v-test")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: true},
		tools: []mcp.MCPTool{
			{
				Name: "observe",
				InputSchema: map[string]any{
					"properties": map[string]any{
						"what":  map[string]any{"type": "string"},
						"limit": map[string]any{"type": "number"},
					},
				},
			},
		},
		handleFn: func(req mcp.JSONRPCRequest, name string, _ json.RawMessage) (mcp.JSONRPCResponse, bool) {
			if name != "observe" {
				return mcp.JSONRPCResponse{}, false
			}
			return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.TextResponse("ok")}, true
		},
	})

	resp := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"errors","limit":10}}`),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/call response = %+v, want success", resp)
	}

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("result unmarshal error = %v", err)
	}
	for _, block := range result.Content {
		if strings.Contains(block.Text, "_warnings:") {
			t.Fatalf("did not expect warnings for known args, got %q", block.Text)
		}
	}
}

func TestMCPHandlerToolRateLimit(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(nil, "v")
	h.SetToolHandler(&fakeToolHandlerForMCP{
		cap:     capture.NewCapture(),
		limiter: testLimiter{allowed: false},
	})

	resp := h.HandleRequest(mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{}}`),
	})
	if resp == nil || resp.Error == nil || resp.Error.Code != -32603 {
		t.Fatalf("rate-limited response = %+v, want internal error with rate-limit message", resp)
	}
}

// Warning tests (upgrade/update) moved to handler_warning_test.go
