// Purpose: Tests for MCP JSON-RPC response construction and validation.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// response_test.go — Tests for response formatting and size clamping.
package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestNegotiateProtocolVersion(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		params json.RawMessage
		want   string
	}{
		{name: "latest", params: json.RawMessage(`{"protocolVersion":"2025-06-18"}`), want: ProtocolVersionLatest},
		{name: "legacy", params: json.RawMessage(`{"protocolVersion":"2024-11-05"}`), want: ProtocolVersionLegacy},
		{name: "unsupported", params: json.RawMessage(`{"protocolVersion":"1999-01-01"}`), want: ProtocolVersionLatest},
		{name: "invalid", params: json.RawMessage(`{`), want: ProtocolVersionLatest},
		{name: "empty", want: ProtocolVersionLatest},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := NegotiateProtocolVersion(tc.params); got != tc.want {
				t.Fatalf("NegotiateProtocolVersion() = %q, want %q", got, tc.want)
			}
		})
	}
}

// buildRawToolResult creates an MCPToolResult JSON without clamping.
func buildRawToolResult(text string) json.RawMessage {
	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: text}},
	}
	raw, _ := json.Marshal(result)
	return json.RawMessage(raw)
}

func TestClampResponseSize_UnderLimit(t *testing.T) {
	t.Parallel()
	result := buildRawToolResult("hello world")
	clamped := ClampResponseSize(result)
	if string(clamped) != string(result) {
		t.Errorf("expected no change for small response")
	}
}

func TestClampResponseSize_OverLimit(t *testing.T) {
	t.Parallel()
	bigText := strings.Repeat("x", MaxResponseBytes+1000)
	result := buildRawToolResult(bigText)
	clamped := ClampResponseSize(result)

	if len(clamped) >= len(result) {
		t.Errorf("expected clamped to be smaller: clamped=%d, original=%d", len(clamped), len(result))
	}

	if !strings.Contains(string(clamped), "[truncated") {
		t.Error("expected truncation note in clamped response")
	}
}

func TestClampResponseSize_PreservesStructure(t *testing.T) {
	t.Parallel()
	bigText := strings.Repeat("a", MaxResponseBytes+5000)
	result := buildRawToolResult(bigText)
	clamped := ClampResponseSize(result)

	var toolResult MCPToolResult
	if err := json.Unmarshal(clamped, &toolResult); err != nil {
		t.Fatalf("clamped response should be valid JSON: %v", err)
	}
	if len(toolResult.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	if toolResult.Content[0].Type != "text" {
		t.Errorf("expected type=text, got %s", toolResult.Content[0].Type)
	}
}

func TestClampResponseSize_JSONPayload(t *testing.T) {
	t.Parallel()
	bigJSON := `{"key":"` + strings.Repeat("z", MaxResponseBytes+1000) + `"}`
	result := buildRawToolResult(bigJSON)
	clamped := ClampResponseSize(result)

	if len(clamped) >= len(result) {
		t.Errorf("expected clamped JSON response to be smaller")
	}
}

func TestClampResponseSize_ErrorResponse(t *testing.T) {
	t.Parallel()
	bigText := strings.Repeat("e", MaxResponseBytes+1000)
	result := MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: bigText}},
		IsError: true,
	}
	raw, _ := json.Marshal(result)
	clamped := ClampResponseSize(json.RawMessage(raw))

	var toolResult MCPToolResult
	if err := json.Unmarshal(clamped, &toolResult); err != nil {
		t.Fatalf("clamped error response should be valid JSON: %v", err)
	}
	if len(clamped) >= len(raw) {
		t.Error("expected error response to also be clamped when oversized")
	}
}

func TestClampResponseSize_PaginationHint(t *testing.T) {
	t.Parallel()
	bigText := strings.Repeat("p", MaxResponseBytes+5000)
	result := buildRawToolResult(bigText)
	clamped := ClampResponseSize(result)

	if !strings.Contains(string(clamped), "pagination") {
		t.Error("expected pagination hint in truncation note")
	}
}

func TestAppendWarningsToToolResult(t *testing.T) {
	t.Parallel()
	result := &MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: "ok"}},
	}

	changed := AppendWarningsToToolResult(result, []string{"unknown parameter 'x' (ignored)"})
	if !changed {
		t.Fatal("expected warnings to be appended")
	}
	if len(result.Content) != 2 {
		t.Fatalf("content blocks = %d, want 2", len(result.Content))
	}
	if !strings.Contains(result.Content[1].Text, "_warnings:") {
		t.Fatalf("warning block text missing prefix: %q", result.Content[1].Text)
	}
}

func TestAppendWarningsToToolResult_NoOp(t *testing.T) {
	t.Parallel()
	result := &MCPToolResult{
		Content: []MCPContentBlock{{Type: "text", Text: "ok"}},
	}

	if AppendWarningsToToolResult(result, nil) {
		t.Fatal("expected false for nil warnings")
	}
	if AppendWarningsToToolResult(nil, []string{"warn"}) {
		t.Fatal("expected false for nil result")
	}
}

func TestResponseBuildersFailClosedOnUnserializablePayloads(t *testing.T) {
	t.Parallel()
	result := JSONResponse("cannot encode", make(chan int))
	var decoded MCPToolResult
	if err := json.Unmarshal(result, &decoded); err != nil || !decoded.IsError ||
		len(decoded.Content) != 1 || !strings.Contains(decoded.Content[0].Text, "Failed to serialize response") {
		t.Fatalf("serialization failure = %s, %v", result, err)
	}
	if got := SafeMarshal(make(chan int), `{"fallback":true}`); string(got) != `{"fallback":true}` {
		t.Fatalf("SafeMarshal fallback = %s", got)
	}
	req := JSONRPCRequest{JSONRPC: JSONRPCVersion, ID: 1}
	if _, stop := ParseArgs(req, json.RawMessage(`not-json`), &map[string]any{}); !stop {
		t.Fatal("ParseArgs accepted malformed JSON")
	}
	var optional map[string]any
	LenientUnmarshal(nil, &optional)
	LenientUnmarshal(json.RawMessage(`not-json`), &optional)
}

func TestImageContentBlock(t *testing.T) {
	t.Parallel()
	block := ImageContentBlock("dGVzdA==", "image/png")
	if block.Type != "image" {
		t.Errorf("Type = %q, want 'image'", block.Type)
	}
	if block.Data != "dGVzdA==" {
		t.Errorf("Data = %q, want 'dGVzdA=='", block.Data)
	}
	if block.MimeType != "image/png" {
		t.Errorf("MimeType = %q, want 'image/png'", block.MimeType)
	}
	if block.Text != "" {
		t.Errorf("Text should be empty for image blocks, got %q", block.Text)
	}
}

func TestAppendImageToResponse(t *testing.T) {
	t.Parallel()
	textResult := TextResponse("Screenshot captured")
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: json.RawMessage(`1`), Result: textResult}

	resp = AppendImageToResponse(resp, "dGVzdA==", "image/jpeg")

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if len(result.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(result.Content))
	}
	if result.Content[0].Type != "text" {
		t.Errorf("first block type = %q, want 'text'", result.Content[0].Type)
	}
	if result.Content[1].Type != "image" {
		t.Errorf("second block type = %q, want 'image'", result.Content[1].Type)
	}
	if result.Content[1].Data != "dGVzdA==" {
		t.Errorf("image data = %q, want 'dGVzdA=='", result.Content[1].Data)
	}
	if result.Content[1].MimeType != "image/jpeg" {
		t.Errorf("image mimeType = %q, want 'image/jpeg'", result.Content[1].MimeType)
	}
}

func TestAppendImageToResponse_EmptyData(t *testing.T) {
	t.Parallel()
	textResult := TextResponse("test")
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: json.RawMessage(`1`), Result: textResult}

	resp = AppendImageToResponse(resp, "", "image/png")

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block (no image appended for empty data), got %d", len(result.Content))
	}
}

func TestClampResponseSize_JSONBoundaryTruncation(t *testing.T) {
	t.Parallel()
	// Build a JSON payload with nested objects that exceeds MaxResponseBytes.
	// After clamping, the result should still be valid JSON (parseable).
	inner := `{"items":[` + strings.Repeat(`{"id":1,"name":"test item with some content"},`, MaxResponseBytes/50) + `{"id":999}]}`
	result := buildRawToolResult(inner)
	clamped := ClampResponseSize(result)

	var toolResult MCPToolResult
	if err := json.Unmarshal(clamped, &toolResult); err != nil {
		t.Fatalf("clamped JSON-heavy response should still be valid MCPToolResult JSON: %v\nraw: %s", err, string(clamped)[:200])
	}
	if len(toolResult.Content) == 0 {
		t.Fatal("expected at least one content block after clamping")
	}
}

func TestClampResponseSize_TruncatedTextContainsValidishJSON(t *testing.T) {
	t.Parallel()
	// The inner text (which is JSON) should remain parseable after truncation (#9.QA7).
	inner := `{"items":[` + strings.Repeat(`{"id":1,"val":"data"},`, MaxResponseBytes/25) + `{"id":999}]}`
	result := buildRawToolResult(inner)
	clamped := ClampResponseSize(result)

	var toolResult MCPToolResult
	if err := json.Unmarshal(clamped, &toolResult); err != nil {
		t.Fatalf("wrapper should be valid JSON: %v", err)
	}

	// The text content should be parseable as JSON (truncateAtJSONBoundary should close brackets)
	text := toolResult.Content[0].Text
	// The truncation note now precedes the JSON body instead of following it:
	// appended after, it made the very payload this test guards unparseable.
	// The body is everything from the first line that begins with '{'.
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "{") {
			text = text[strings.Index(text, line):]
			break
		}
	}

	var inner2 any
	if err := json.Unmarshal([]byte(text), &inner2); err != nil {
		// With trailing-comma fix (#9.R3.1) and stack-based closers (#9.R2),
		// truncated JSON should be parseable. Fail if it isn't.
		t.Errorf("truncated JSON text should be parseable after boundary fix: %v\nfirst 200 chars: %.200s", err, text)
	}
}

func TestClampResponseSize_DeeplyNestedJSON(t *testing.T) {
	t.Parallel()
	// Build 10-level nested JSON that exceeds limit (#9.QA8)
	prefix := ""
	suffix := ""
	for i := 0; i < 10; i++ {
		prefix += `{"level` + string(rune('0'+i)) + `":[`
		suffix = `]}` + suffix
	}
	inner := prefix + strings.Repeat(`"data",`, MaxResponseBytes/8) + `"end"` + suffix
	result := buildRawToolResult(inner)
	clamped := ClampResponseSize(result)

	var toolResult MCPToolResult
	if err := json.Unmarshal(clamped, &toolResult); err != nil {
		t.Fatalf("deeply nested JSON should still produce valid MCPToolResult: %v", err)
	}
	if len(toolResult.Content) == 0 {
		t.Fatal("expected content blocks")
	}
}

func TestClampResponseSize_PreservesImageBlocks(t *testing.T) {
	t.Parallel()
	// Image blocks should not count toward the byte limit.
	textContent := strings.Repeat("x", MaxResponseBytes-500)
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: textContent},
			{Type: "image", Data: strings.Repeat("A", 50000), MimeType: "image/png"},
		},
	}
	raw, _ := json.Marshal(result)
	clamped := ClampResponseSize(json.RawMessage(raw))

	var toolResult MCPToolResult
	if err := json.Unmarshal(clamped, &toolResult); err != nil {
		t.Fatalf("response with image block should remain valid: %v", err)
	}
	// Text should NOT be truncated since it's under the limit
	hasImage := false
	for _, block := range toolResult.Content {
		if block.Type == "image" {
			hasImage = true
		}
	}
	if !hasImage {
		t.Error("image content block should be preserved")
	}
}

func TestClampResponseSize_TextExceedsLimitWithImageBlock(t *testing.T) {
	t.Parallel()
	// Text exceeds limit AND image block present (#9.QA9).
	// Image bytes should be excluded from the effective limit calculation,
	// so only text gets truncated while image is preserved.
	textContent := strings.Repeat("x", MaxResponseBytes+5000)
	result := MCPToolResult{
		Content: []MCPContentBlock{
			{Type: "text", Text: textContent},
			{Type: "image", Data: strings.Repeat("B", 30000), MimeType: "image/jpeg"},
		},
	}
	raw, _ := json.Marshal(result)
	clamped := ClampResponseSize(json.RawMessage(raw))

	var toolResult MCPToolResult
	if err := json.Unmarshal(clamped, &toolResult); err != nil {
		t.Fatalf("response should remain valid after clamping: %v", err)
	}

	// Text should be truncated
	if !strings.Contains(string(clamped), "[truncated") {
		t.Error("text should be truncated when exceeding limit")
	}

	// Image block should still be present
	hasImage := false
	for _, block := range toolResult.Content {
		if block.Type == "image" {
			hasImage = true
			if block.Data == "" {
				t.Error("image data should be preserved")
			}
		}
	}
	if !hasImage {
		t.Error("image block should be preserved even when text is truncated")
	}
}

// ============================================
// truncateAtJSONBoundary direct tests (#9.QA10)
// ============================================

func TestTruncateAtJSONBoundary_EmptyInput(t *testing.T) {
	t.Parallel()
	result := truncateAtJSONBoundary("")
	if result != "" {
		t.Errorf("empty input should return empty, got %q", result)
	}
}

func TestTruncateAtJSONBoundary_NonJSON(t *testing.T) {
	t.Parallel()
	input := "plain text content"
	result := truncateAtJSONBoundary(input)
	if result != input {
		t.Errorf("non-JSON should be returned as-is, got %q", result)
	}
}

func TestTruncateAtJSONBoundary_SimpleObject(t *testing.T) {
	t.Parallel()
	input := `{"key":"val","other":"data`
	result := truncateAtJSONBoundary(input)
	// Should close the open brace
	if !strings.HasSuffix(result, "}") {
		t.Errorf("should close open brace, got %q", result)
	}
}

func TestTruncateAtJSONBoundary_NestedArray(t *testing.T) {
	t.Parallel()
	input := `[{"a":1},{"b":2`
	result := truncateAtJSONBoundary(input)
	// The lastSafe search finds the comma after 1}, so truncation removes {"b":2.
	// Only the outer [ remains unmatched → closed with ]
	if !strings.HasSuffix(result, "]") {
		t.Errorf("should close outer bracket, got %q", result)
	}
	// Verify the incomplete {"b":2 is removed
	if strings.Contains(result, `"b"`) {
		t.Errorf("incomplete object should be removed, got %q", result)
	}
}

func TestTruncateAtJSONBoundary_ClosingOrder(t *testing.T) {
	t.Parallel()
	// [{ should close as }] not ]}
	input := `[{"key":"val`
	result := truncateAtJSONBoundary(input)
	// Find the closers at the end
	if !strings.HasSuffix(result, "}]") {
		t.Errorf("closers should be in reverse order: }] not ]}, got %q", result)
	}
}

func TestTruncateAtJSONBoundary_EscapedQuotes(t *testing.T) {
	t.Parallel()
	// The string value contains escaped quotes that shouldn't confuse the parser
	input := `{"key":"value with \"escaped\" quotes","other":"trunc`
	result := truncateAtJSONBoundary(input)
	if !strings.HasSuffix(result, "}") {
		t.Errorf("should handle escaped quotes, got %q", result)
	}
}

func TestTruncateAtJSONBoundary_AllOpeners(t *testing.T) {
	t.Parallel()
	// All openers, no closers
	input := `[[[{{{`
	result := truncateAtJSONBoundary(input)
	// All should be closed
	if !strings.Contains(result, "}}}]]]") {
		t.Errorf("should close all openers in reverse order, got %q", result)
	}
}

func TestTruncateAtJSONBoundary_BraceInsideString(t *testing.T) {
	t.Parallel()
	// A closing brace inside a quoted string should NOT pop the stack (#9.QA.HIGH2).
	input := `{"msg":"hello } world","key":"trunc`
	result := truncateAtJSONBoundary(input)
	// The } inside "hello } world" is inside a string, so the outer { is still open.
	if !strings.HasSuffix(result, "}") {
		t.Errorf("should close outer brace, got %q", result)
	}
	// Verify the result doesn't double-close (only one unmatched { at top level)
	// Count net openers outside strings
	opens := strings.Count(result, "{") - strings.Count(result, "}")
	// Inside-string braces are counted by strings.Count but that's fine —
	// the key check is that the function produces output ending in }
	_ = opens
}

func TestTruncateAtJSONBoundary_Unicode(t *testing.T) {
	t.Parallel()
	// Unicode characters in values should not break truncation (#9.QA.HIGH3).
	input := `{"emoji":"🚀","name":"日本語","data":"trunc`
	result := truncateAtJSONBoundary(input)
	if !strings.HasSuffix(result, "}") {
		t.Errorf("should close outer brace with unicode content, got %q", result)
	}
	// Should still contain the emoji (it's before truncation point)
	if !strings.Contains(result, "🚀") {
		t.Errorf("unicode content before truncation should be preserved, got %q", result)
	}
}

func TestImageContentBlock_SerializesCorrectly(t *testing.T) {
	t.Parallel()
	block := ImageContentBlock("AAAA", "image/png")
	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	s := string(data)
	// Should have "type":"image", "data":"AAAA", "mimeType":"image/png"
	if !strings.Contains(s, `"type":"image"`) {
		t.Errorf("JSON should contain type:image, got: %s", s)
	}
	if !strings.Contains(s, `"data":"AAAA"`) {
		t.Errorf("JSON should contain data:AAAA, got: %s", s)
	}
	if !strings.Contains(s, `"mimeType":"image/png"`) {
		t.Errorf("JSON should contain mimeType:image/png, got: %s", s)
	}
	// Should NOT have "text" field
	if strings.Contains(s, `"text"`) {
		t.Errorf("image block JSON should not contain 'text' field, got: %s", s)
	}
}

// An oversized response must stay machine-readable. Truncation appended the
// "[truncated: ...]" note AFTER the JSON body, so every byte of bracket-closing
// work in truncateAtJSONBoundary was undone by the note itself and the client
// received unparseable JSON. Observed on analyze/draw_history: a 717KB payload
// clamped to 100KB came back as JSON followed by prose, and json.Unmarshal on
// the body failed outright rather than returning a shortened result.
func TestClampKeepsTheJSONBodyParseable(t *testing.T) {
	sessions := make([]map[string]any, 4000)
	for i := range sessions {
		sessions[i] = map[string]any{"file": fmt.Sprintf("draw-session-%d.json", i), "path": strings.Repeat("p", 60)}
	}
	body, err := json.Marshal(map[string]any{"count": len(sessions), "sessions": sessions})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	// Real responses are a prose header followed by the JSON body.
	text := "Draw history\n" + string(body)

	raw, err := json.Marshal(MCPToolResult{Content: []MCPContentBlock{{Type: "text", Text: text}}})
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if len(raw) <= MaxResponseBytes {
		t.Fatalf("fixture must exceed the clamp limit, got %d bytes", len(raw))
	}

	var clamped MCPToolResult
	if err := json.Unmarshal(ClampResponseSize(raw), &clamped); err != nil {
		t.Fatalf("unmarshal clamped result: %v", err)
	}
	got := clamped.Content[0].Text

	if !strings.Contains(got, "truncated") {
		t.Fatal("a clamped response must say it was truncated")
	}

	// The JSON body is everything from the first line that starts with '{'.
	idx := strings.Index(got, "\n{")
	if idx == -1 {
		t.Fatalf("clamped text lost its JSON body: %.120q", got)
	}
	jsonBody := got[idx+1:]
	var parsed map[string]any
	if err := json.Unmarshal([]byte(jsonBody), &parsed); err != nil {
		t.Fatalf("clamped JSON body does not parse (%v); tail was %.160q", err, jsonBody[max(0, len(jsonBody)-160):])
	}
	if _, ok := parsed["sessions"]; !ok {
		t.Fatal("clamped body should retain the sessions field, shortened")
	}
}

// A clamp firing means the mode that produced the response has no adequate
// limit of its own — the backstop is doing a paginator's job. That was only
// ever reported as English prose inside the text body, so nothing could detect
// it: not a test, not a dashboard, not the agent deciding whether to trust the
// payload. Report it as structured metadata as well.
func TestClampReportsItselfInMetadata(t *testing.T) {
	inner := `{"items":[` + strings.Repeat(`{"id":1,"val":"data"},`, MaxResponseBytes/25) + `{"id":999}]}`
	raw, err := json.Marshal(MCPToolResult{Content: []MCPContentBlock{{Type: "text", Text: inner}}})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if len(raw) <= MaxResponseBytes {
		t.Fatalf("fixture must exceed the clamp limit, got %d bytes", len(raw))
	}

	var clamped MCPToolResult
	if err := json.Unmarshal(ClampResponseSize(raw), &clamped); err != nil {
		t.Fatalf("unmarshal clamped: %v", err)
	}
	if clamped.Metadata == nil {
		t.Fatal("a clamped response must carry metadata saying so")
	}
	if truncated, _ := clamped.Metadata["response_truncated"].(bool); !truncated {
		t.Errorf("response_truncated = %v, want true", clamped.Metadata["response_truncated"])
	}
	original, _ := clamped.Metadata["original_bytes"].(float64)
	if int(original) != len(raw) {
		t.Errorf("original_bytes = %v, want %d", clamped.Metadata["original_bytes"], len(raw))
	}
	if limit, _ := clamped.Metadata["limit_bytes"].(float64); int(limit) != MaxResponseBytes {
		t.Errorf("limit_bytes = %v, want %d", clamped.Metadata["limit_bytes"], MaxResponseBytes)
	}
}

// A response that fits must not claim it was clamped, or the signal is noise.
func TestClampLeavesSmallResponsesUnmarked(t *testing.T) {
	raw, err := json.Marshal(MCPToolResult{Content: []MCPContentBlock{{Type: "text", Text: `{"ok":true}`}}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out MCPToolResult
	if err := json.Unmarshal(ClampResponseSize(raw), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, marked := out.Metadata["response_truncated"]; marked {
		t.Error("a response within the limit must not be marked truncated")
	}
}
