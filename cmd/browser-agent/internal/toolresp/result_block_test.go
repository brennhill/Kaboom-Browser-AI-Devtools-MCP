// result_block_test.go — Tests for decoding and rewriting the JSON payload in a tool result.

package toolresp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func textResult(t *testing.T, blocks ...string) mcp.JSONRPCResponse {
	t.Helper()
	content := make([]mcp.MCPContentBlock, 0, len(blocks))
	for _, text := range blocks {
		content = append(content, mcp.MCPContentBlock{Type: "text", Text: text})
	}
	payload, err := json.Marshal(mcp.MCPToolResult{Content: content})
	if err != nil {
		t.Fatalf("marshal tool result: %v", err)
	}
	return mcp.JSONRPCResponse{JSONRPC: "2.0", Result: payload}
}

func TestDecodeFirstJSONBlock_SkipsNonJSONBlocks(t *testing.T) {
	t.Parallel()
	resp := textResult(t, "not JSON at all", "list_interactive results\n{\"total\":3}")
	block, ok := DecodeFirstJSONBlock(resp)
	if !ok {
		t.Fatal("expected the second block to decode")
	}
	if block.Data["total"] != float64(3) {
		t.Fatalf("Data = %#v, want total 3", block.Data)
	}
}

func TestDecodeFirstJSONBlock_RefusesErrorResults(t *testing.T) {
	t.Parallel()
	// Control: the same payload decodes when the result is not an error.
	if _, ok := DecodeFirstJSONBlock(textResult(t, "x\n{\"a\":1}")); !ok {
		t.Fatal("control: a non-error result with a JSON block must decode")
	}
	payload, err := json.Marshal(mcp.MCPToolResult{
		IsError: true,
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "x\n{\"a\":1}"}},
	})
	if err != nil {
		t.Fatalf("marshal tool result: %v", err)
	}
	if _, ok := DecodeFirstJSONBlock(mcp.JSONRPCResponse{Result: payload}); ok {
		t.Fatal("an error result must not decode: annotating it would dress a refusal as a result")
	}
}

func TestResultBlock_ReplacePreservesPrefixAndOtherBlocks(t *testing.T) {
	t.Parallel()
	resp := textResult(t, "leading note", "list_interactive results\n{\"total\":3}")
	block, ok := DecodeFirstJSONBlock(resp)
	if !ok {
		t.Fatal("expected a block")
	}
	block.Data["index_generation"] = "gen_7"
	updated := block.Replace(resp)

	var result mcp.MCPToolResult
	if err := json.Unmarshal(updated.Result, &result); err != nil {
		t.Fatalf("decode replaced result: %v", err)
	}
	if result.Content[0].Text != "leading note" {
		t.Fatalf("first block changed: %q", result.Content[0].Text)
	}
	const prefix = "list_interactive results\n"
	if !strings.HasPrefix(result.Content[1].Text, prefix) {
		t.Fatalf("prefix lost: %q", result.Content[1].Text)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(result.Content[1].Text, prefix)), &data); err != nil {
		t.Fatalf("decode replaced payload: %v", err)
	}
	if data["index_generation"] != "gen_7" || data["total"] != float64(3) {
		t.Fatalf("replaced payload = %#v", data)
	}
}

func TestSetNestedElements(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		data map[string]any
		read func(map[string]any) []any
	}{
		{
			name: "top level",
			data: map[string]any{"elements": []any{1, 2, 3}},
			read: func(d map[string]any) []any { return d["elements"].([]any) },
		},
		{
			name: "under result",
			data: map[string]any{"result": map[string]any{"elements": []any{1, 2}}},
			read: func(d map[string]any) []any { return d["result"].(map[string]any)["elements"].([]any) },
		},
		{
			name: "under result.result",
			data: map[string]any{"result": map[string]any{"result": map[string]any{"elements": []any{1, 2}}}},
			read: func(d map[string]any) []any {
				return d["result"].(map[string]any)["result"].(map[string]any)["elements"].([]any)
			},
		},
	} {
		SetNestedElements(tc.data, []any{9})
		if got := tc.read(tc.data); len(got) != 1 {
			t.Errorf("%s: elements = %#v, want the replacement list", tc.name, got)
		}
	}
}

func TestSetNestedElements_LeavesUnrelatedPayloadsAlone(t *testing.T) {
	t.Parallel()
	data := map[string]any{"summary": "no elements here"}
	SetNestedElements(data, []any{9})
	if len(data) != 1 || data["summary"] != "no elements here" {
		t.Fatalf("payload without an element list was modified: %#v", data)
	}
}
